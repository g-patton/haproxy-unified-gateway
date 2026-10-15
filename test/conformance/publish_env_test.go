//go:build conformance

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

package conformance_test

import "testing"

func TestImplementationFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantVer string
	}{
		{name: "unset defaults to dev", env: "", wantVer: "dev"},
		{name: "release tag is used verbatim", env: "v1.0.7", wantVer: "v1.0.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HUG_IMPLEMENTATION_VERSION", tt.env)
			impl := implementationFromEnv()
			if impl.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", impl.Version, tt.wantVer)
			}
			if impl.Organization != "HAProxy Technologies" {
				t.Errorf("Organization = %q, want %q", impl.Organization, "HAProxy Technologies")
			}
			if impl.Project != "haproxy-unified-gateway" {
				t.Errorf("Project = %q, want %q", impl.Project, "haproxy-unified-gateway")
			}
			if err := impl.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestControllerImageFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "unset defaults to latest", env: "", want: "haproxytech/haproxy-unified-gateway:latest"},
		{name: "release image tag is used", env: "1.0.7", want: "haproxytech/haproxy-unified-gateway:1.0.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HUG_CONTROLLER_IMAGE_TAG", tt.env)
			if got := controllerImageFromEnv(); got != tt.want {
				t.Errorf("controllerImageFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}
