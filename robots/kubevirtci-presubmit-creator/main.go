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
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	prowjobs "sigs.k8s.io/prow/pkg/apis/prowjobs/v1"
	"sigs.k8s.io/prow/pkg/config"
	"sigs.k8s.io/prow/pkg/flagutil"
	"sigs.k8s.io/yaml"

	"kubevirt.io/project-infra/pkg/querier"
)

const OrgAndRepoForJobConfig = "kubevirt/kubevirtci"

type options struct {
	dryRun bool

	github                           flagutil.GitHubOptions
	jobConfigPathKubevirtciPresubmit string
	k8sReleaseSemver                 string
}

func (o *options) Validate() error {
	if err := o.github.Validate(o.dryRun); err != nil {
		return err
	}
	if _, err := os.Stat(o.jobConfigPathKubevirtciPresubmit); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtciPresubmit is required: %v", err)
	}
	if o.k8sReleaseSemver != "" && !querier.SemVerMinorRegex.MatchString(o.k8sReleaseSemver) {
		return fmt.Errorf("k8s-release-semver does not match SemVerMinorRegex: %s", o.k8sReleaseSemver)
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.BoolVar(&o.dryRun, "dry-run", true, "Whether the file should get modified or just modifications printed to stdout.")
	o.github.AddFlags(fs)
	fs.StringVar(&o.jobConfigPathKubevirtciPresubmit, "job-config-path-kubevirtci-presubmit", "", "The directory of the k8s providers")
	fs.StringVar(&o.k8sReleaseSemver, "k8s-release-semver", "", "The semver of the k8s release to create a presubmit for")
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
	log := logrus.StandardLogger().WithField("robot", "kubevirtci-presubmit-creator")

	o := gatherOptions()
	if err := o.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid arguments provided.")
	}

	client, err := o.github.GitHubClient(false)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a GitHub client.")
	}

	var latestReleaseSemver *querier.SemVer

	if o.k8sReleaseSemver == "" {
		releases, _, err := client.Repositories.ListReleases(ctx, "kubernetes", "kubernetes", nil)
		if err != nil {
			log.Panicln(err)
		}
		releases = querier.ValidReleases(releases)
		if len(releases) == 0 {
			log.Info("No release found, nothing to do.")
			os.Exit(0)
		}
		latestReleaseSemver = querier.ParseRelease(releases[0])
	} else {
		majorMinor := querier.SemVerMinorRegex.FindStringSubmatch(o.k8sReleaseSemver)
		latestReleaseSemver = &querier.SemVer{
			Major: majorMinor[1],
			Minor: majorMinor[2],
			Patch: "0",
		}
	}

	jobConfig, err := config.ReadJobConfig(o.jobConfigPathKubevirtciPresubmit)
	if err != nil {
		log.Panicln(err)
	}

	newJobConfig, exists := AddNewPresubmitIfNotExists(jobConfig, latestReleaseSemver)
	if exists && !o.dryRun {
		log.Info(fmt.Sprintf("presubmit job for %v exists, nothing to do.", latestReleaseSemver))
		os.Exit(0)
	}

	marshalledConfig, err := yaml.Marshal(newJobConfig)
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

func AddNewPresubmitIfNotExists(jobConfig config.JobConfig, latestReleaseSemver *querier.SemVer) (newJobConfig config.JobConfig, jobExists bool) {
	newJobConfig = jobConfig
	kubevirtciJobs := make(map[string]config.Presubmit, len(newJobConfig.PresubmitsStatic[OrgAndRepoForJobConfig]))
	for _, job := range newJobConfig.PresubmitsStatic[OrgAndRepoForJobConfig] {
		kubevirtciJobs[job.Name] = job
	}

	wantedCheckProvisionJobName := createKubevirtciPresubmitJobName(latestReleaseSemver)
	if _, exists := kubevirtciJobs[wantedCheckProvisionJobName]; exists {
		return newJobConfig, true
	}

	newPresubmitJobForRelease := CreatePresubmitJobForRelease(latestReleaseSemver)
	newJobConfig.PresubmitsStatic[OrgAndRepoForJobConfig] = append(newJobConfig.PresubmitsStatic[OrgAndRepoForJobConfig], newPresubmitJobForRelease)
	return newJobConfig, false
}

func CreatePresubmitJobForRelease(semver *querier.SemVer) config.Presubmit {
	yes := true
	golangImage := "quay.io/kubevirtci/golang:v20260319-c8f1db8"
	cluster := "prow-workloads"
	res := config.Presubmit{
		AlwaysRun: false,
		Optional:  true,
		JobBase: config.JobBase{
			UtilityConfig: config.UtilityConfig{
				DecorationConfig: &prowjobs.DecorationConfig{
					Timeout: &prowjobs.Duration{Duration: 3*time.Hour + 30*time.Minute},
				},
			},
			Name:           fmt.Sprintf("check-provision-k8s-%s.%s", semver.Major, semver.Minor),
			MaxConcurrency: 3,
			Labels: map[string]string{
				"preset-docker-mirror-proxy":            "true",
				"preset-kubevirtci-check-provision-env": "true",
				"preset-podman-in-container-enabled":    "true",
			},
			Cluster: cluster,
			Spec: &v1.PodSpec{
				NodeSelector: map[string]string{
					"type": "bare-metal-external",
				},
				Containers: []v1.Container{
					{
						Image: golangImage,
						Command: []string{
							"/usr/local/bin/entrypoint.sh",
						},
						Args: []string{
							"/bin/sh",
							"-c",
							fmt.Sprintf("cd cluster-provision/k8s/%s.%s && ../provision.sh", semver.Major, semver.Minor),
						},
						Env: []v1.EnvVar{
							{
								Name:  "GO_MOD_PATH",
								Value: "cluster-provision/gocli/go.mod",
							},
							{
								Name:  "PROVISION_CENTOS_VERSION",
								Value: "10",
							},
						},
						SecurityContext: &v1.SecurityContext{
							Privileged: &yes,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceMemory: *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
							},
						},
					},
				},
			},
		},
	}
	return res
}

func createKubevirtciPresubmitJobName(latestReleaseSemver *querier.SemVer) string {
	return fmt.Sprintf("check-provision-k8s-%s.%s", latestReleaseSemver.Major, latestReleaseSemver.Minor)
}
