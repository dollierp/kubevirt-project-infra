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
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"sigs.k8s.io/prow/pkg/flagutil"

	"kubevirt.io/project-infra/pkg/flakefinder"
	ghapi "kubevirt.io/project-infra/pkg/flakefinder/github"
)

func flagOptions() options {
	o := options{
		endpoint: flagutil.NewStrings("https://api.github.com"),
	}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.BoolVar(&o.dryRun, "dry-run", true, "Whether report should be only printed to standard out instead of written to gcs") // TODO: incompatible change, requires setting flags on jobs
	fs.DurationVar(&o.merged, "merged", 24*7*time.Hour, "Filter to issues merged in the time window")
	o.github.AddFlags(fs)
	// TODO: remove after backwards compatibility is not required anymore
	fs.StringVar(&o.tokenPath, "token", "", "Path to github token")
	fs.Var(&o.endpoint, "endpoint", "GitHub's API endpoint")
	fs.BoolVar(&o.isPreview, "preview", false, "Whether report should be written to preview directory")
	fs.StringVar(&o.prBaseBranch, "pr_base_branch", PRBaseBranchDefault, "Base branch for the PRs")
	fs.StringVar(&o.reportOutputChildPath, "report_output_child_path", "", fmt.Sprintf("Child path below the main reporting directory '%s' (i.e. 'master')", flakefinder.ReportsPath))
	fs.StringVar(&o.org, "org", Org, "GitHub org name")
	fs.StringVar(&o.repo, "repo", Repo, "GitHub org name")
	fs.BoolVar(&o.today, "today", false, "Whether to create a report for the current day only (i.e. using data starting from report day 00:00Z till now)")
	fs.BoolVar(&o.skipBeforeStartOfReport, "skip_results_before_start_of_report", true, "Whether to skip test results occurring before start of report")
	fs.StringVar(&o.periodicJobDirRegex, "periodic_job_dir_regex", "", "Regular expression to use for fetching data from periodic jobs, or empty string if not wanted")
	fs.StringVar(&o.batchJobDirRegex, "batch_job_dir_regex", "pull-kubevirt-e2e-.*", "Regular expression to use for filtering the fetching of batch job data")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	return o
}

type options struct {
	dryRun bool
	github flagutil.GitHubOptions
	// TODO: remove after backwards compatibility is not required anymore
	tokenPath               string
	endpoint                flagutil.Strings
	merged                  time.Duration
	isPreview               bool
	prBaseBranch            string
	reportOutputChildPath   string
	org                     string
	repo                    string
	today                   bool
	skipBeforeStartOfReport bool
	periodicJobDirRegex     string
	batchJobDirRegex        string
}

const MaxNumberOfReportsToLinkTo = 50
const PRBaseBranchDefault = "master"
const Org = "kubevirt"
const Repo = "kubevirt"

var ReportOutputPath = flakefinder.ReportsPath

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	o := flagOptions()

	// TODO: remove after backwards compatibility is not required anymore
	if o.tokenPath != "" {
		o.github.TokenPath = o.tokenPath
	}

	if err := o.github.Validate(o.dryRun); err != nil {
		log.Fatalf("Failed to validate GitHub options: %v.", err)
	}

	ReportOutputPath = BuildReportOutputPath(o)

	ghClient, err := o.github.GitHubClient(o.dryRun)
	if err != nil {
		log.Fatalf("Failed to create a GitHub client: %v.", err)
	}

	ctx := context.Background()
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create new storage client: %v.\n", err)
	}

	reportBaseDataOptions := flakefinder.NewReportBaseDataOptions(o.prBaseBranch, o.today, o.merged, o.org, o.repo, o.skipBeforeStartOfReport)
	reportBaseDataOptions.SetPeriodicJobDirRegex(o.periodicJobDirRegex)
	reportBaseDataOptions.SetBatchJobDirRegex(o.batchJobDirRegex)

	reportBaseData := flakefinder.GetReportBaseData(ctx, ghapi.NewQuery(ghClient, o.org, o.repo, o.prBaseBranch), storageClient, reportBaseDataOptions)

	err = WriteReportToBucket(ctx, storageClient, o.merged, o.org, o.repo, o.dryRun, reportBaseData)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to write report: %v", err))
		return
	}

	printIndexPageToStdOut := o.dryRun
	err = CreateReportIndex(ctx, storageClient, o.org, o.repo, printIndexPageToStdOut)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to create report index page: %v", err))
		return
	}
}

// BuildReportOutputPath creates the path to which the report will get written, considering also if we are in
// preview mode, so that existing production reports will not be overwritten. I.e. considering
//
//	options{
//			reportOutputChildPath: "kubevirt/kubevirt"
//			isPreview:			   true
//	}
//
// will lead to
// "reports/flakefinder/preview/kubevirt/kubevirt"
func BuildReportOutputPath(o options) string {
	outputPath := flakefinder.ReportsPath
	if o.isPreview {
		outputPath = filepath.Join(outputPath, flakefinder.PreviewPath)
	}
	outputPath = filepath.Join(outputPath, o.reportOutputChildPath)
	return outputPath
}
