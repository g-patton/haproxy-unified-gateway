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
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// artifactZip builds a single-entry zip holding the given file, which is the
// shape the GitLab job-artifacts endpoint returns.
func artifactZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err = w.Write([]byte(content)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err = zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestAcquireReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/42/pipelines":
			if got := r.URL.Query().Get("ref"); got != "v1.0.7" {
				t.Errorf("ref query = %q, want v1.0.7", got)
			}
			if got := r.Header.Get("PRIVATE-TOKEN"); got != "token" {
				t.Errorf("PRIVATE-TOKEN = %q, want token", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":7,"status":"success"}]`))
		case "/projects/42/pipelines/7/jobs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":99,"name":"conformance-GW-API-1_5_0"}]`))
		case "/projects/42/jobs/99/artifacts":
			zipBytes := artifactZip(t, "conformance-report-1.5.0.yaml", goodReport)
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	path, cleanup, err := AcquireReport(context.Background(), server.Client(),
		server.URL, "token", "42", "v1.0.7", 0)
	if err != nil {
		t.Fatalf("AcquireReport: %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading acquired report: %v", err)
	}
	if string(got) != goodReport {
		t.Error("acquired report differs from the artifact's contents")
	}
}

func TestAcquireReportUsesExplicitPipeline(t *testing.T) {
	var sawPipelines bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/42/pipelines":
			sawPipelines = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case "/projects/42/pipelines/7/jobs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":99,"name":"conformance-GW-API-1_5_0"}]`))
		case "/projects/42/jobs/99/artifacts":
			_, _ = w.Write(artifactZip(t, "conformance-report-1.5.0.yaml", goodReport))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, cleanup, err := AcquireReport(context.Background(), server.Client(),
		server.URL, "token", "42", "v1.0.7", 7)
	if err != nil {
		t.Fatalf("AcquireReport: %v", err)
	}
	defer cleanup()

	if sawPipelines {
		t.Error("AcquireReport listed pipelines even though --pipeline was given")
	}
}

func TestAcquireReportFailsOnMissingPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	if _, _, err := AcquireReport(context.Background(), server.Client(),
		server.URL, "token", "42", "v9.9.9", 0); err == nil {
		t.Fatal("AcquireReport returned nil error for an unknown ref, want a failure")
	}
}

func TestAcquireReportFailsWithoutToken(t *testing.T) {
	if _, _, err := AcquireReport(context.Background(), nil, "http://x", "", "42", "v1.0.7", 0); err == nil {
		t.Fatal("AcquireReport returned nil error with no token, want a failure")
	}
}
