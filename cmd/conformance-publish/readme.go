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
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const repoURL = "https://github.com/haproxytech/haproxy-unified-gateway"

// docsURL is HAProxy's product documentation, linked from the README intro.
const docsURL = "https://www.haproxy.com/documentation/haproxy-unified-gateway/"

// companyURL is HAProxy Technologies' homepage, linked from the README intro.
const companyURL = "https://www.haproxy.com"

const tableHeader = "| API channel | Implementation version | Mode | Report |"

// reproduceSteps carries formatting verbs for the tag and the image tag. Commands
// are fenced code blocks rather than inline code, matching upstream's own
// implementation READMEs, so each step is a copyable block.
const reproduceSteps = "The following steps reproduce the HAProxy Unified Gateway `%s` conformance test report.\n" +
	"\n" +
	"1. Pull the released controller image:\n" +
	"\n" +
	"    ```bash\n" +
	"    docker pull haproxytech/haproxy-unified-gateway:%s\n" +
	"    ```\n" +
	"\n" +
	"1. Create a Kind cluster and deploy the prerequisites:\n" +
	"\n" +
	"    ```bash\n" +
	"    task conformance-create-ci\n" +
	"    ```\n" +
	"\n" +
	"1. Build and load the conformance test image:\n" +
	"\n" +
	"    ```bash\n" +
	"    task conformance-build-test-image\n" +
	"    ```\n" +
	"\n" +
	"1. Run the suite as an in-cluster Job, reporting version `%s`:\n" +
	"\n" +
	"    ```bash\n" +
	"    task conformance-run-job\n" +
	"    ```\n" +
	"\n" +
	"1. The suite writes `conformance-report.yaml`, which is published here verbatim.\n"

// ReadmeInput is everything needed to render the implementation README. Existing
// is nil when the file does not exist yet.
type ReadmeInput struct {
	Tag       string
	Channel   string
	Mode      string
	Slug      string
	Reproduce string
	Contact   []string
	Existing  []byte
}

// RenderReadme produces the upstream README. When the file does not exist it is
// rendered from the template. When it does exist, its bytes are preserved and
// only this version's table row is replaced or inserted, so prose a maintainer
// added during a previous review is not silently discarded.
func RenderReadme(in ReadmeInput) ([]byte, error) {
	row := renderRow(in)

	if len(in.Existing) == 0 {
		return []byte(renderFull(in, []string{row})), nil
	}
	return spliceRow(in.Existing, in.Tag, row)
}

// renderRow builds this version's table row. Mode is emitted verbatim so a
// non-default mode stays consistent with the report field and the filename.
func renderRow(in ReadmeInput) string {
	return fmt.Sprintf("| %s | [%s](%s/releases/tag/%s) | %s | [%s report](./%s) |",
		in.Channel, in.Tag, repoURL, in.Tag, in.Mode, in.Tag,
		ReportFileName(in.Channel, in.Tag, in.Mode))
}

// renderFull emits the whole document. Rows are emitted in the order given so
// previously published versions keep their position.
func renderFull(in ReadmeInput, rows []string) string {
	var b strings.Builder
	b.WriteString("# HAProxy Unified Gateway\n\n")
	b.WriteString("[HAProxy Unified Gateway](")
	b.WriteString(repoURL)
	b.WriteString(") is the official Gateway API implementation from [HAProxy Technologies](")
	b.WriteString(companyURL)
	b.WriteString(").\n")
	b.WriteString("For installation and configuration, see the [HAProxy Unified Gateway documentation](")
	b.WriteString(docsURL)
	b.WriteString(").\n\n")
	b.WriteString("## Table of contents\n\n")
	b.WriteString(tableHeader)
	// The plain separator from upstream's template (conformance/reports/README.md),
	// not an aligned one, so the table matches every other implementation's README.
	b.WriteString("\n|-------------|------------------------|------|--------|\n")
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\n")
	}
	b.WriteString("\n## Reproduce\n\n")
	// reproduceSteps carries formatting verbs for the tag and the image tag.
	_, _ = fmt.Fprintf(&b, reproduceSteps, in.Tag, ImageTag(in.Tag), in.Tag)
	if in.Mode != "default" {
		_, _ = fmt.Fprintf(&b, "\n### Configuring the %s mode\n\n", in.Mode)
		b.WriteString(strings.TrimRight(in.Reproduce, "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\n## Contact\n\n")
	for _, contact := range in.Contact {
		b.WriteString("- ")
		b.WriteString(contact)
		b.WriteString("\n")
	}
	return b.String()
}

// releaseLink matches the version cell of a rendered row and captures the tag.
var releaseLink = regexp.MustCompile(`/releases/tag/([^)]+)\)`)

// spliceRow returns existing with this tag's row replaced, or inserted at the end
// of the table when absent. Every other byte — including prose in Reproduce and
// Contact — is preserved verbatim.
func spliceRow(existing []byte, tag, row string) ([]byte, error) {
	lines := strings.Split(string(existing), "\n")
	inTable := false
	headerSeen := false
	replaced := false
	insertAt := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Table lines all start with "|", including the separator row
		// "|---|:-:|:-:|---|", which does not start with "| ".
		if !strings.HasPrefix(trimmed, "|") {
			if inTable && insertAt == -1 {
				insertAt = i
			}
			inTable = false
			continue
		}
		if !inTable {
			// The first "|" line opens the table and is its header.
			inTable = true
			headerSeen = true
			continue
		}
		if strings.HasPrefix(trimmed, "|---") {
			continue
		}
		if match := releaseLink.FindStringSubmatch(trimmed); match != nil && match[1] == tag {
			lines[i] = row
			replaced = true
		}
	}
	if !headerSeen {
		return nil, errors.New("existing README has no conformance table to update")
	}
	if replaced {
		return []byte(strings.Join(lines, "\n")), nil
	}
	if insertAt == -1 {
		insertAt = len(lines)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, row)
	out = append(out, lines[insertAt:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// markdownLink captures the destination of a markdown link.
var markdownLink = regexp.MustCompile(`\]\(([^)]+)\)`)

// CheckLinks performs a GET request for every link in the README. Upstream's
// verify-reports.sh link-checks this file, so a dead link fails the MR; catching
// it here saves a round trip. GET rather than HEAD because some hosts answer
// HEAD with 405, which would read as a dead link.
func CheckLinks(client *http.Client, readme []byte) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	seen := map[string]bool{}
	for _, match := range markdownLink.FindAllStringSubmatch(string(readme), -1) {
		target := match[1]
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			continue
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return fmt.Errorf("building request for %s: %w", target, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("checking %s: %w", target, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf("checking %s: status %d", target, resp.StatusCode)
		}
	}
	return nil
}
