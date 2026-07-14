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
	"strconv"
	"text/template"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/prow/pkg/flagutil"

	"kubevirt.io/project-infra/pkg/querier"
)

type options struct {
	github           flagutil.GitHubOptions
	org              string
	repo             string
	latest           bool
	latestPatchOf    string
	latestThreeMinor string
	template         string
	major            int
	minor            int
}

func (o *options) Validate() error {
	if err := o.github.Validate(false); err != nil {
		return err
	}
	queries := 0
	if o.latest {
		queries++
	}
	if o.latestThreeMinor != "" {
		queries++
		if !querier.SemVerMajorRegex.MatchString(o.latestThreeMinor) {
			return fmt.Errorf("Invalid format given to -latest-three-minor-of")
		}
		semver := querier.SemVerMajorRegex.FindStringSubmatch(o.latestThreeMinor)
		o.major, _ = strconv.Atoi(semver[1])
	}
	if o.latestPatchOf != "" {
		queries++
		if !querier.SemVerMinorRegex.MatchString(o.latestPatchOf) {
			return fmt.Errorf("Invalid format given to -latest-patch-of")
		}
		semver := querier.SemVerMinorRegex.FindStringSubmatch(o.latestPatchOf)
		o.major, _ = strconv.Atoi(semver[1])
		o.minor, _ = strconv.Atoi(semver[2])
	}

	if queries == 0 {
		return fmt.Errorf("Either -latest, -last-three-minor-of or -last-patch-of must be specified.")
	} else if queries > 1 {
		return fmt.Errorf("Only one of -latest, -last-three-minor-of or -last-patch-of can be specified at the same time.")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	o.github.AddFlags(fs)
	fs.StringVar(&o.org, "org", "kubevirt", "Organization")
	fs.StringVar(&o.repo, "repo", "kubevirt", "Organization")
	fs.BoolVar(&o.latest, "latest", false, "Query for the latest release")
	fs.StringVar(&o.latestThreeMinor, "last-three-minor-of", "", "Query for the last three minor releases of a given release (e.g. v1 or 2)")
	fs.StringVar(&o.latestPatchOf, "last-patch-of", "", "Latest patch release of the given release (e.g. v1.14 or 0.12)")
	fs.StringVar(&o.template, "template", "v{{.Major}}.{{.Minor}}.{{.Patch}}", "How to print the detected versions")
	_ = fs.Parse(os.Args[1:])
	return o
}

func main() {

	logrus.SetFormatter(&logrus.JSONFormatter{})
	// TODO: Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.StandardLogger().WithField("robot", "release-querier")

	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid arguments provided.")
	}

	client, err := o.github.GitHubClient(false)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a GitHub client.")
	}

	releases, _, err := client.Repositories.ListReleases(ctx, o.org, o.repo, nil)
	if err != nil {
		log.Panicln(err)
	}
	tmpl, err := template.New("test").Parse(o.template)
	if err != nil {
		log.Panicln(err)
	}

	if o.latest {
		latest := querier.LatestRelease(releases)
		if latest != nil {
			if err := tmpl.Execute(os.Stdout, querier.ParseRelease(latest)); err != nil {
				log.Panicln(err)
			}
			fmt.Print("\n")
		}
	} else if o.latestPatchOf != "" {
		latestPatchOf := querier.LastPatchOf(uint(o.major), uint(o.minor), releases)
		if latestPatchOf != nil {
			if err := tmpl.Execute(os.Stdout, querier.ParseRelease(latestPatchOf)); err != nil {
				log.Panicln(err)
			}
			fmt.Print("\n")
		}
	} else if o.latestThreeMinor != "" {
		minors := querier.LastThreeMinor(uint(o.major), releases)
		for _, release := range minors {
			if err := tmpl.Execute(os.Stdout, querier.ParseRelease(release)); err != nil {
				log.Panicln(err)
			}
			fmt.Print("\n")
		}
	}
}
