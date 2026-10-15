// Copyright 2026 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command conformance-publish turns a Gateway API conformance report for a
// release tag into the files an upstream kubernetes-sigs/gateway-api MR adds.
// It validates the report against the rules upstream enforces and renders the
// implementation README; it never modifies the report, which is published
// byte-for-byte as the suite produced it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "conformance-publish:", err)
		os.Exit(1)
	}
}

type options struct {
	tag        string
	out        string
	report     string
	reproduce  string
	ref        string
	project    string
	pipeline   int
	checkLinks bool
}

func parseOptions(args []string) (*options, error) {
	opts := &options{}
	fs := flag.NewFlagSet("conformance-publish", flag.ContinueOnError)
	fs.StringVar(&opts.tag, "tag", "", "release tag to publish, e.g. v1.0.7 (required)")
	fs.StringVar(&opts.out, "out", "", "output root; the upstream tree is written beneath it (required)")
	fs.StringVar(&opts.report, "report", "", "path to the conformance report YAML; bypasses CI acquisition")
	fs.StringVar(&opts.reproduce, "reproduce", "", "prose documenting a non-default mode; required when mode != default")
	fs.IntVar(&opts.pipeline, "pipeline", 0, "GitLab pipeline ID to fetch the report artifact from")
	fs.StringVar(&opts.ref, "ref", "", "Git ref to resolve a pipeline for; defaults to --tag")
	fs.StringVar(&opts.project, "project", "", "GitLab project ID; defaults to $CI_PROJECT_ID")
	fs.BoolVar(&opts.checkLinks, "check-links", false, "HTTP-check the generated README links before writing")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	for _, req := range []struct{ name, value string }{
		{"tag", opts.tag},
		{"out", opts.out},
	} {
		if req.value == "" {
			return nil, fmt.Errorf("--%s is required", req.name)
		}
	}
	return opts, nil
}

// resolveReport returns a path to the report to publish and a cleanup func. The
// report comes from --report when given, and otherwise from a GitLab pipeline
// artifact.
func resolveReport(opts *options) (string, func(), error) {
	if opts.report != "" {
		return opts.report, func() {}, nil
	}
	baseURL := os.Getenv("CI_API_V4_URL")
	project := opts.project
	if project == "" {
		project = os.Getenv("CI_PROJECT_ID")
	}
	if baseURL == "" || project == "" {
		return "", nil, errors.New("no report: pass --report, or set CI_API_V4_URL and CI_PROJECT_ID to use --pipeline")
	}
	ref := opts.ref
	if ref == "" {
		ref = opts.tag
	}
	return AcquireReport(context.Background(), &http.Client{Timeout: 5 * time.Minute},
		baseURL, os.Getenv("GITLAB_TOKEN"), project, ref, opts.pipeline)
}

// run does all work in memory and only touches the filesystem once validation
// has passed, so a rejected report leaves no partial tree behind.
func run(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}

	reportPath, cleanup, err := resolveReport(opts)
	if err != nil {
		return err
	}
	defer cleanup()

	report, err := LoadReport(reportPath)
	if err != nil {
		return err
	}
	if err = report.Validate(opts.tag); err != nil {
		return err
	}
	if err = checkReproduce(opts, report); err != nil {
		return err
	}

	paths, err := DerivePaths(opts.out, opts.tag, report.Data)
	if err != nil {
		return err
	}

	existing, err := readIfExists(paths.Readme)
	if err != nil {
		return err
	}
	readme, err := RenderReadme(ReadmeInput{
		Tag:       opts.tag,
		Channel:   report.Data.GatewayAPIChannel,
		Mode:      report.Data.Mode,
		Slug:      ImplementationSlug,
		Reproduce: opts.reproduce,
		Contact:   report.Data.Implementation.Contact,
		Existing:  existing,
	})
	if err != nil {
		return err
	}

	if opts.checkLinks {
		if err = CheckLinks(&http.Client{Timeout: 30 * time.Second}, readme); err != nil {
			return err
		}
	}

	if err = os.MkdirAll(paths.Dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", paths.Dir, err)
	}
	if err = os.WriteFile(paths.Report, report.Raw, 0o644); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	if err = os.WriteFile(paths.Readme, readme, 0o644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}

	fmt.Printf("wrote %s\nwrote %s\n", paths.Report, paths.Readme)
	return nil
}

// checkReproduce enforces rule 10 in both directions, so the documented mode and
// the reported mode cannot drift apart.
func checkReproduce(opts *options, report *Report) error {
	switch {
	case report.Data.Mode != "default" && opts.reproduce == "":
		return fmt.Errorf("rule 10: mode is %q, so --reproduce is required to document the setup", report.Data.Mode)
	case report.Data.Mode == "default" && opts.reproduce != "":
		return errors.New("rule 10: --reproduce was supplied but the report's mode is default")
	}
	return nil
}

// readIfExists returns the file's contents, or nil when it does not exist. Any
// other error is fatal.
func readIfExists(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return content, nil
}
