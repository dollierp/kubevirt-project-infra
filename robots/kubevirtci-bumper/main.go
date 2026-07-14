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

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/prow/pkg/flagutil"

	"kubevirt.io/project-infra/pkg/kubevirtci"
	"kubevirt.io/project-infra/pkg/querier"
)

type options struct {
	github                 flagutil.GitHubOptions
	ensureLatest           bool
	forceTargetMajorMinor  string
	preReleaseVersion      string
	ensureLatestThreeMinor string
	ensureOnlyLatestThree  bool
	major                  int
	providerDir            string
	clusterUpDir           string
}

func (o *options) Validate() error {
	if err := o.github.Validate(false); err != nil {
		return err
	}
	tasks := 0
	if o.ensureLatest {
		tasks++
		if o.forceTargetMajorMinor != "" && !querier.SemVerMinorRegex.MatchString(o.forceTargetMajorMinor) {
			return fmt.Errorf("Invalid format given to -force-target-major-minor")
		}
	}
	if o.ensureLatestThreeMinor != "" {
		tasks++
		if !querier.SemVerMajorRegex.MatchString(o.ensureLatestThreeMinor) {
			return fmt.Errorf("Invalid format given to -latest-three-minor-of")
		}
		semver := querier.SemVerMajorRegex.FindStringSubmatch(o.ensureLatestThreeMinor)
		o.major, _ = strconv.Atoi(semver[1])
	}
	if o.ensureOnlyLatestThree {
		o.major = 1
		tasks++
	}

	if tasks == 0 {
		return fmt.Errorf("Either -ensure-latest or -ensure-last-three-minor-of must be specified.")
	} else if tasks > 1 {
		return fmt.Errorf("only one of -ensure-latest, -ensure-last-three-minor-of or -ensure-only-latest-three can be specified at the same time")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
        o.github.AddFlags(fs)
	fs.BoolVar(&o.ensureLatest, "ensure-latest", false, "Ensure that we have a provider for the latest k8s release")
	fs.StringVar(&o.forceTargetMajorMinor, "force-target-major-minor", "", `when using ensure-latest, override latest k8s release to use given target major.minor (i.e. "1.28"`)
	fs.StringVar(&o.preReleaseVersion, "pre-release-version", "", `when using ensure-latest, add k8s pre release suffix (i.e. add alpha0 as in "1.28.0-alpha0"`)
	fs.StringVar(&o.ensureLatestThreeMinor, "ensure-last-three-minor-of", "", "Ensure that the last three minor releases of the given major release are up to date (e.g. v1 or 2)")
	fs.BoolVar(&o.ensureOnlyLatestThree, "ensure-only-latest-three", false, "Ensure that only the latest three minor releases of the given major release exist (aka remove older providers)")
	fs.StringVar(&o.providerDir, "k8s-provider-dir", "", "The directory of the k8s providers")
	fs.StringVar(&o.clusterUpDir, "cluster-up-dir", "", "The directory of the cluster up configurations")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logrus.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

func main() {

	logrus.SetFormatter(&logrus.JSONFormatter{})
	// TODO: Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.StandardLogger().WithField("robot", "kubevirtci-bumper")

	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid arguments provided.")
	}

	client, err := o.github.GitHubClient(false)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a GitHub client.")
	}

	releases, _, err := client.Repositories.ListReleases(ctx, "kubernetes", "kubernetes", nil)
	if err != nil {
		log.Panicln(err)
	}
	releases = querier.ValidReleases(releases)

	_, err = os.Stat(o.providerDir)
	if os.IsNotExist(err) {
		log.Errorf("Directory '%s' does not exist", o.providerDir)
		os.Exit(1)
	} else if err != nil {
		log.WithError(err).Errorf("Failed to check directory '%s'", o.providerDir)
	}

	if len(releases) == 0 {
		log.Info("No release found.")
		os.Exit(0)
	}

	if o.ensureLatest {
		targetRelease := releases[0]
		if o.forceTargetMajorMinor != "" {
			tagName := fmt.Sprintf("v%s.0", o.forceTargetMajorMinor)
			if o.preReleaseVersion != "" {
				tagName += "-" + o.preReleaseVersion
			}
			targetRelease = &github.RepositoryRelease{
				TagName: &tagName,
			}
		}
		err := kubevirtci.EnsureProviderExists(o.providerDir, o.clusterUpDir, targetRelease)
		if err != nil {
			log.WithError(err).Info("Failed to ensure that a provider for the given release exists.")
		}
	} else if o.ensureLatestThreeMinor != "" {
		minors := querier.LastThreeMinor(uint(o.major), releases)
		err := kubevirtci.BumpMinorReleaseOfProvider(o.providerDir, minors)
		if err != nil {
			log.WithError(err).Info("Failed to update the providers for the last minor releases.")
		}
	} else if o.ensureOnlyLatestThree {
		minors := querier.LastThreeMinor(uint(o.major), releases)
		err := kubevirtci.DropUnsupportedProviders(o.providerDir, o.clusterUpDir, minors)
		if err != nil {
			log.WithError(err).Info("Failed to remove unsupported providers.")
		}
	}
}
