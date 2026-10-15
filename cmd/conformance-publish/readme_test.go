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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func baseInput() ReadmeInput {
	return ReadmeInput{
		Tag:     "v1.0.7",
		Channel: "standard",
		Mode:    "default",
		Slug:    ImplementationSlug,
		Contact: []string{"https://github.com/haproxytech/haproxy-unified-gateway/issues"},
	}
}

func TestRenderReadmeFromScratch(t *testing.T) {
	out, err := RenderReadme(baseInput())
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		"# HAProxy Unified Gateway",
		"https://www.haproxy.com/documentation/haproxy-unified-gateway/",
		"[HAProxy Technologies](https://www.haproxy.com)",
		"| API channel | Implementation version | Mode | Report |",
		"|-------------|------------------------|------|--------|",
		"| standard |",
		"https://github.com/haproxytech/haproxy-unified-gateway/releases/tag/v1.0.7",
		"[v1.0.7 report](./standard-v1.0.7-default-report.yaml)",
		"## Reproduce",
		"## Contact",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered README does not contain %q\n---\n%s", want, got)
		}
	}
	// Reproduce steps use fenced code blocks (as upstream's own READMEs do),
	// not inline backticks, so each command renders as its own copyable block.
	for _, want := range []string{
		"```bash\n    docker pull haproxytech/haproxy-unified-gateway:1.0.7\n    ```",
		"```bash\n    task conformance-run-job\n    ```",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered README does not contain the fenced block %q\n---\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("rendered README does not end with a newline")
	}
}

func TestRenderReadmeAppendsRow(t *testing.T) {
	first, err := RenderReadme(baseInput())
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}

	second := baseInput()
	second.Tag = "v1.0.8"
	out, err := RenderReadme(ReadmeInput{
		Tag: second.Tag, Channel: second.Channel, Mode: second.Mode,
		Slug: second.Slug, Contact: second.Contact, Existing: first,
	})
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "v1.0.7 report") {
		t.Error("previous version's row was lost")
	}
	if !strings.Contains(got, "v1.0.8 report") {
		t.Error("new version's row was not added")
	}
	if strings.Count(got, "| standard |") != 2 {
		t.Errorf("want exactly 2 table rows, got %d", strings.Count(got, "| standard |"))
	}
}

func TestRenderReadmeIsIdempotent(t *testing.T) {
	first, err := RenderReadme(baseInput())
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	in := baseInput()
	in.Existing = first
	second, err := RenderReadme(in)
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("second render differs from the first\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestRenderReadmePreservesExistingContent(t *testing.T) {
	first, err := RenderReadme(baseInput())
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	// A maintainer edits the Reproduce section while reviewing the first MR.
	edited := strings.Replace(string(first), "## Reproduce\n",
		"## Reproduce\n\nReviewed by the Gateway API maintainers.\n", 1)
	if edited == string(first) {
		t.Fatal("fixture edit did not apply")
	}

	in := baseInput()
	in.Tag = "v1.0.8"
	in.Existing = []byte(edited)
	out, err := RenderReadme(in)
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "Reviewed by the Gateway API maintainers.") {
		t.Error("existing Reproduce prose was discarded")
	}
	if !strings.Contains(got, "v1.0.7 report") || !strings.Contains(got, "v1.0.8 report") {
		t.Error("both version rows should be present")
	}
	if strings.Count(got, "## Contact") != 1 {
		t.Errorf("Contact section count = %d, want 1", strings.Count(got, "## Contact"))
	}
}

func TestRenderReadmeRejectsExistingFileWithoutTable(t *testing.T) {
	in := baseInput()
	in.Existing = []byte("# HAProxy Unified Gateway\n\nno table here\n")
	if _, err := RenderReadme(in); err == nil {
		t.Fatal("RenderReadme accepted an existing README with no table, want an error")
	}
}

func TestRenderReadmeWithNonDefaultMode(t *testing.T) {
	in := baseInput()
	in.Mode = "per-gateway"
	in.Reproduce = "Scale the default controller to 0 and create one controller per Gateway."
	out, err := RenderReadme(in)
	if err != nil {
		t.Fatalf("RenderReadme: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "standard-v1.0.7-per-gateway-report.yaml") {
		t.Error("filename does not carry the non-default mode")
	}
	if !strings.Contains(got, "### Configuring the per-gateway mode") {
		t.Error("non-default mode is not documented in the Reproduce section")
	}
	if !strings.Contains(got, "Scale the default controller to 0") {
		t.Error("supplied mode prose was not emitted")
	}
}

func TestCheckLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Run("reachable links pass", func(t *testing.T) {
		readme := []byte("[ok](" + server.URL + "/ok)\n")
		if err := CheckLinks(server.Client(), readme); err != nil {
			t.Errorf("CheckLinks = %v, want nil", err)
		}
	})

	t.Run("unreachable link fails with url and status", func(t *testing.T) {
		readme := []byte("[bad](" + server.URL + "/missing)\n")
		err := CheckLinks(server.Client(), readme)
		if err == nil {
			t.Fatal("CheckLinks = nil, want an error")
		}
		if !strings.Contains(err.Error(), server.URL+"/missing") || !strings.Contains(err.Error(), "404") {
			t.Errorf("error = %q, want it to name the URL and the status", err.Error())
		}
	})
}
