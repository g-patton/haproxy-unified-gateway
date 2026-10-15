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
	"path/filepath"
	"testing"

	confv1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
)

func TestImageTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{name: "strips leading v", tag: "v1.0.7", want: "1.0.7"},
		{name: "tag without v is unchanged", tag: "1.0.7", want: "1.0.7"},
		{name: "prerelease keeps its suffix", tag: "v1.1.0-rc.1", want: "1.1.0-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ImageTag(tt.tag); got != tt.want {
				t.Errorf("ImageTag(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestVersionDir(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{name: "patch is truncated", version: "v1.5.1", want: "v1.5"},
		{name: "no v prefix is normalized", version: "1.5.1", want: "v1.5"},
		{name: "already major.minor", version: "v1.5", want: "v1.5"},
		{name: "two-digit minor", version: "v1.10.0", want: "v1.10"},
		{name: "major only is rejected", version: "v1", wantErr: true},
		{name: "non-numeric is rejected", version: "dev", wantErr: true},
		{name: "empty is rejected", version: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VersionDir(tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("VersionDir(%q) = %q, want error", tt.version, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("VersionDir(%q) returned error: %v", tt.version, err)
			}
			if got != tt.want {
				t.Errorf("VersionDir(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestDerivePaths(t *testing.T) {
	report := &confv1.ConformanceReport{
		GatewayAPIVersion: "v1.5.1",
		GatewayAPIChannel: "standard",
		Mode:              "default",
	}

	got, err := DerivePaths("/tmp/out", "v1.0.7", report)
	if err != nil {
		t.Fatalf("DerivePaths returned error: %v", err)
	}

	wantDir := filepath.Join("/tmp/out", "conformance", "reports", "v1.5", ImplementationSlug)
	if got.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", got.Dir, wantDir)
	}
	wantReport := filepath.Join(wantDir, "standard-v1.0.7-default-report.yaml")
	if got.Report != wantReport {
		t.Errorf("Report = %q, want %q", got.Report, wantReport)
	}
	if want := filepath.Join(wantDir, "README.md"); got.Readme != want {
		t.Errorf("Readme = %q, want %q", got.Readme, want)
	}
}

func TestDerivePathsRejectsBadVersion(t *testing.T) {
	report := &confv1.ConformanceReport{GatewayAPIVersion: "dev", GatewayAPIChannel: "standard", Mode: "default"}
	if _, err := DerivePaths("/tmp/out", "v1.0.7", report); err == nil {
		t.Fatal("DerivePaths with gatewayAPIVersion=dev returned nil error, want error")
	}
}
