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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodReport is a minimal but structurally complete ConformanceReport whose
// profiles use only legal result values.
const goodReport = `apiVersion: gateway.networking.k8s.io/v1
date: "2026-08-10T10:00:00Z"
gatewayAPIChannel: standard
gatewayAPIVersion: v1.5.1
implementation:
  contact:
  - https://github.com/haproxytech/haproxy-unified-gateway/issues
  organization: HAProxy Technologies
  project: haproxy-unified-gateway
  url: https://github.com/haproxytech/haproxy-unified-gateway
  version: v1.0.7
kind: ConformanceReport
mode: default
profiles:
- core:
    result: success
    statistics: {Passed: 30, Failed: 0, Skipped: 0}
  name: GATEWAY-HTTP
`

func writeReport(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "conformance-report.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func loadGood(t *testing.T) *Report {
	t.Helper()
	r, err := LoadReport(writeReport(t, goodReport))
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}
	return r
}

func TestLoadReportAcceptsGoodReport(t *testing.T) {
	r := loadGood(t)
	if r.Data.Implementation.Version != "v1.0.7" {
		t.Errorf("Version = %q, want v1.0.7", r.Data.Implementation.Version)
	}
	if string(r.Raw) != goodReport {
		t.Error("Raw is not the file's bytes verbatim")
	}
}

func TestLoadReportRejectsRule1(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "not yaml", content: "\tnot: [valid"},
		{name: "wrong kind", content: strings.Replace(goodReport, "kind: ConformanceReport", "kind: SomethingElse", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadReport(writeReport(t, tt.content))
			if err == nil {
				t.Fatal("LoadReport returned nil error, want a rule 1 failure")
			}
			if !strings.HasPrefix(err.Error(), "rule 1:") {
				t.Errorf("error = %q, want a rule 1 prefix", err.Error())
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		tag      string
		wantRule string
	}{
		{
			name:     "rule 2 version mismatches the tag",
			content:  goodReport,
			tag:      "v1.0.8",
			wantRule: "rule 2:",
		},
		{
			name:     "rule 2 dev is never publishable",
			content:  strings.Replace(goodReport, "version: v1.0.7", "version: dev", 1),
			tag:      "v1.0.7",
			wantRule: "rule 2:",
		},
		{
			name:     "rule 3 old organization name",
			content:  strings.Replace(goodReport, "organization: HAProxy Technologies", "organization: haproxytech", 1),
			tag:      "v1.0.7",
			wantRule: "rule 3:",
		},
		{
			name:     "rule 4 empty project",
			content:  strings.Replace(goodReport, "project: haproxy-unified-gateway", "project: ''", 1),
			tag:      "v1.0.7",
			wantRule: "rule 4:",
		},
		{
			name:     "rule 4 empty contact",
			content:  strings.Replace(goodReport, "  contact:\n  - https://github.com/haproxytech/haproxy-unified-gateway/issues\n", "  contact: []\n", 1),
			tag:      "v1.0.7",
			wantRule: "rule 4:",
		},
		{
			name:     "rule 5 unknown channel",
			content:  strings.Replace(goodReport, "gatewayAPIChannel: standard", "gatewayAPIChannel: beta", 1),
			tag:      "v1.0.7",
			wantRule: "rule 5:",
		},
		{
			name:     "rule 6 empty mode",
			content:  strings.Replace(goodReport, "mode: default", "mode: ''", 1),
			tag:      "v1.0.7",
			wantRule: "rule 6:",
		},
		{
			name:     "rule 7 empty gatewayAPIVersion",
			content:  strings.Replace(goodReport, "gatewayAPIVersion: v1.5.1", "gatewayAPIVersion: ''", 1),
			tag:      "v1.0.7",
			wantRule: "rule 7:",
		},
		{
			name:     "rule 8 core failure",
			content:  strings.Replace(goodReport, "result: success", "result: failure", 1),
			tag:      "v1.0.7",
			wantRule: "rule 8:",
		},
		{
			name: "rule 8 extended failure",
			content: strings.Replace(goodReport, "profiles:\n- core:", `profiles:
- extended:
    result: failure
    statistics: {Passed: 0, Failed: 0, Skipped: 1}
  core:`, 1),
			tag:      "v1.0.7",
			wantRule: "rule 8:",
		},
		{
			name:     "rule 9 CRLF line endings",
			content:  strings.ReplaceAll(goodReport, "\n", "\r\n"),
			tag:      "v1.0.7",
			wantRule: "rule 9:",
		},
		{
			name:     "rule 9 missing trailing newline",
			content:  strings.TrimSuffix(goodReport, "\n"),
			tag:      "v1.0.7",
			wantRule: "rule 9:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := LoadReport(writeReport(t, tt.content))
			if err != nil {
				t.Fatalf("LoadReport: %v", err)
			}
			err = r.Validate(tt.tag)
			if err == nil {
				t.Fatal("Validate returned nil error, want a failure")
			}
			if !strings.HasPrefix(err.Error(), tt.wantRule) {
				t.Errorf("error = %q, want a %s prefix", err.Error(), tt.wantRule)
			}
		})
	}
}

func TestValidateAcceptsGoodReport(t *testing.T) {
	r := loadGood(t)
	if err := r.Validate("v1.0.7"); err != nil {
		t.Errorf("Validate(v1.0.7) = %v, want nil", err)
	}
}

func TestValidateRejectsTagWithoutV(t *testing.T) {
	r := loadGood(t)
	if err := r.Validate("v1.0.7"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// ImageTag strips the leading v for the container image, but the report's
	// implementation.version keeps it, so a tag supplied without the v is a
	// mismatch against the report rather than a silently accepted variant.
	if err := r.Validate("1.0.7"); err == nil {
		t.Error("Validate(1.0.7) = nil, want a rule 2 mismatch against v1.0.7")
	}
}

func TestValidateToleratesMissingExtendedProfile(t *testing.T) {
	r := loadGood(t)
	if len(r.Data.ProfileReports) == 0 {
		t.Fatal("fixture has no profiles")
	}
	if r.Data.ProfileReports[0].Extended != nil {
		t.Skip("fixture unexpectedly has an extended profile")
	}
	if err := r.Validate("v1.0.7"); err != nil {
		t.Errorf("Validate with no extended profile = %v, want nil", err)
	}
}
