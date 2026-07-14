/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/prow/pkg/flagutil"
)

type options struct {
	github              flagutil.GitHubOptions
	org                 string
	repo                string
	author              string
	branchName          string
	ensureLabelsMissing string
}

func (o *options) validate() error {
	if err := o.github.Validate(o.dryRun); err != nil {
		return err
	}
	if o.org == "" {
		return fmt.Errorf("org is required")
	}
	if o.repo == "" {
		return fmt.Errorf("repo is required")
	}
	if o.author == "" {
		return fmt.Errorf("author is required")
	}
	if o.branchName == "" {
		return fmt.Errorf("branch-name is required")
	}
	return nil
}

func (o *options) getEnsureLabelsMissing() []string {
	return strings.Split(o.ensureLabelsMissing, ",")
}

var o = options{}

func init() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	o.github.AddFlags(fs)
	fs.StringVar(&o.org, "org", "", "The org for the PR.")
	fs.StringVar(&o.repo, "repo", "", "The repo for the PR.")
	fs.StringVar(&o.author, "author", "", "The author for the PR.")
	fs.StringVar(&o.branchName, "branch-name", "", "The branch name for the PR.")
	fs.StringVar(&o.ensureLabelsMissing, "ensure-labels-missing", "lgtm", "What labels have to be missing on the PR (list of comma separated labels).")
}

func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	// TODO: Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)

	if err := fs.Parse(os.Args[1:]); err != nil {
		log().WithError(err).Fatal("Failed to parse arguments.")
	}
	if err := o.validate(); err != nil {
		log().WithError(err).Fatal("Invalid arguments provided.")
	}

	client, err := o.github.GitHubClient(false)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a GitHub client.")
	}

	prs, _, err := client.PullRequests.List(ctx, o.org, o.repo, &github.PullRequestListOptions{
		State:       "open",
		Head:        fmt.Sprintf("%s:%s", o.author, o.branchName),
		ListOptions: github.ListOptions{},
	})
	if err != nil {
		log().WithError(err).Fatal("failed to find PR")
	} else if len(prs) == 0 {
		log().Info("No PR found")
		os.Exit(0)
	} else if len(prs) > 1 {
		log().Fatalf("More than one PR found: %+v", prs)
	}

	if checkAnyLabelExists(prs[0], o.getEnsureLabelsMissing()) {
		log().WithField("PR", prs[0].GetNumber()).Fatalf("ensureLabelsMissing: some labels were present that shouldn't be")
	}

}

func checkAnyLabelExists(prToCheck *github.PullRequest, labelsToCheck []string) bool {
	labels := map[string]struct{}{}
	for _, label := range prToCheck.Labels {
		name := *label.Name
		labels[name] = struct{}{}
	}
	labelsExist := false
	for _, label := range labelsToCheck {
		if _, exists := labels[label]; exists {
			log().WithField("PR", prToCheck.GetNumber()).Infof("label %s exists", label)
			labelsExist = true
		}
	}
	return labelsExist
}

func log() *logrus.Entry {
	return logrus.StandardLogger().WithField("robot", "labels-checker")
}
