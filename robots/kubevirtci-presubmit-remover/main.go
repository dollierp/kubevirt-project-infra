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

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/flagutil"
	"sigs.k8s.io/yaml"

	"kubevirt.io/project-infra/pkg/kubevirt/release"
	"kubevirt.io/project-infra/pkg/querier"
)

const OrgAndRepoForJobConfig = "kubevirt/kubevirtci"

type options struct {
	dryRun bool

	github                           flagutil.GitHubOptions
	jobConfigPathKubevirtciPresubmit string
}

func (o *options) Validate() error {
	if err := o.github.Validate(o.dryRun); err != nil {
		return err
	}
	if _, err := os.Stat(o.jobConfigPathKubevirtciPresubmit); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtciPresubmit is required: %v", err)
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.BoolVar(&o.dryRun, "dry-run", true, "Whether the file should get modified or just modifications printed to stdout.")
	o.github.AddFlags(fs)
	fs.StringVar(&o.jobConfigPathKubevirtciPresubmit, "job-config-path-kubevirtci-presubmit", "", "The directory of the k8s providers")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		logrus.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

func main() {

	logrus.SetFormatter(&logrus.JSONFormatter{})
	// TODO: Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.StandardLogger().WithField("robot", "kubevirtci-presubmit-remover")

	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid arguments provided.")
	}

	client, err := o.github.GitHubClient(false)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a GitHub client.")
	}

	jobConfig, err := config.ReadJobConfig(o.jobConfigPathKubevirtciPresubmit)
	if err != nil {
		log.Panicln(err)
	}

	releases, _, err := client.Repositories.ListReleases(ctx, "kubernetes", "kubernetes", nil)
	if err != nil {
		log.Panicln(err)
	}
	releases = querier.ValidReleases(releases)
	if len(releases) == 0 {
		log.Info("No release found, nothing to do.")
		os.Exit(0)
	}

	latestMinorReleases := release.GetLatestMinorReleases(release.AsSemVers(releases))
	if len(latestMinorReleases) < 3 {
		log.Info("Not enough minor releases found, nothing to do.")
	}

	targetRelease := latestMinorReleases[3]
	updated := deletePresubmitJobForRelease(&jobConfig, targetRelease)
	if !updated {
		log.Info("Not updated, nothing to do")
		os.Exit(0)
	}

	marshalledConfig, err := yaml.Marshal(jobConfig)
	if err != nil {
		log.WithError(err).Error("Failed to marshall jobconfig")
	}

	if o.dryRun {
		_, err = os.Stdout.Write(marshalledConfig)
		if err != nil {
			log.WithError(err).Error("Failed to write jobconfig")
		}
		os.Exit(0)
	}

	err = os.WriteFile(o.jobConfigPathKubevirtciPresubmit, marshalledConfig, os.ModePerm)
	if err != nil {
		log.WithError(err).Error("Failed to write jobconfig")
	}

}

func deletePresubmitJobForRelease(jobConfig *config.JobConfig, targetReleaseSemver *querier.SemVer) (updated bool) {
	toDeleteJobNames := map[string]struct{}{}
	toDeleteJobNames[createKubevirtciPresubmitJobName(targetReleaseSemver)] = struct{}{}

	var newPresubmits []config.Presubmit

	for _, presubmit := range jobConfig.PresubmitsStatic[OrgAndRepoForJobConfig] {
		if _, exists := toDeleteJobNames[presubmit.Name]; exists {
			updated = true
			continue
		}
		newPresubmits = append(newPresubmits, presubmit)
	}

	if updated {
		jobConfig.PresubmitsStatic[OrgAndRepoForJobConfig] = newPresubmits
	}

	return
}

func createKubevirtciPresubmitJobName(latestReleaseSemver *querier.SemVer) string {
	return fmt.Sprintf("check-provision-k8s-%s.%s", latestReleaseSemver.Major, latestReleaseSemver.Minor)
}
