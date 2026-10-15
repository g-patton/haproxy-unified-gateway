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
	"bytes"
	"fmt"
	"os"

	confv1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
	"sigs.k8s.io/yaml"
)

// expectedOrganization is pinned rather than merely required non-empty: the
// value never varies per release, and pinning turns the likeliest mistake —
// running against a harness that still hardcodes the old name — into a hard
// failure instead of a silently wrong report.
const expectedOrganization = "HAProxy Technologies"

// Report is an acquired report: the parsed view for validation, plus the exact
// bytes that will be written upstream. The bytes are never regenerated.
type Report struct {
	Data *confv1.ConformanceReport
	Raw  []byte
}

func ruleError(rule int, format string, args ...any) error {
	return fmt.Errorf("rule %d: %s", rule, fmt.Sprintf(format, args...))
}

// LoadReport reads a report from disk and applies rule 1.
func LoadReport(path string) (*Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading report %s: %w", path, err)
	}
	var data confv1.ConformanceReport
	if err = yaml.Unmarshal(raw, &data); err != nil {
		return nil, ruleError(1, "report does not parse as YAML: %v", err)
	}
	if data.Kind != "ConformanceReport" {
		return nil, ruleError(1, "kind is %q, want ConformanceReport", data.Kind)
	}
	return &Report{Raw: raw, Data: &data}, nil
}

// Validate applies rules 2 to 9, returning the first failure.
func (r *Report) Validate(tag string) error {
	impl := r.Data.Implementation

	if impl.Version != tag {
		return ruleError(2, "implementation.version is %q, want %q", impl.Version, tag)
	}
	if impl.Organization != expectedOrganization {
		return ruleError(3, "implementation.organization is %q, want %q", impl.Organization, expectedOrganization)
	}
	for _, field := range []struct{ name, value string }{
		{"project", impl.Project},
		{"url", impl.URL},
	} {
		if field.value == "" {
			return ruleError(4, "implementation.%s is empty", field.name)
		}
	}
	if len(impl.Contact) == 0 {
		return ruleError(4, "implementation.contact is empty")
	}

	if r.Data.GatewayAPIChannel != "standard" && r.Data.GatewayAPIChannel != "experimental" {
		return ruleError(5, "gatewayAPIChannel is %q, want standard or experimental", r.Data.GatewayAPIChannel)
	}
	if r.Data.Mode == "" {
		return ruleError(6, "mode is empty")
	}
	if r.Data.GatewayAPIVersion == "" {
		return ruleError(7, "gatewayAPIVersion is empty")
	}

	for _, profile := range r.Data.ProfileReports {
		if err := checkResult(profile.Name, "core", profile.Core.Result); err != nil {
			return err
		}
		if profile.Extended != nil {
			if err := checkResult(profile.Name, "extended", profile.Extended.Result); err != nil {
				return err
			}
		}
	}

	if bytes.ContainsRune(r.Raw, '\r') {
		return ruleError(9, "report contains CR characters; upstream yamllint requires unix newlines")
	}
	if !bytes.HasSuffix(r.Raw, []byte("\n")) {
		return ruleError(9, "report does not end with a newline")
	}
	return nil
}

// checkResult enforces that a profile's result is one upstream accepts. A
// failure result is unpublishable: reports may only be success or partial.
func checkResult(profileName, level string, result confv1.Result) error {
	if result != confv1.Success && result != confv1.Partial {
		return ruleError(8, "profile %q %s result is %q, want success or partial", profileName, level, result)
	}
	return nil
}
