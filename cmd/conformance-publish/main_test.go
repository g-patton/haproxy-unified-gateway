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
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesBothFiles(t *testing.T) {
	reportPath := writeReport(t, goodReport)
	out := t.TempDir()

	err := run([]string{"--tag", "v1.0.7", "--report", reportPath, "--out", out})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	dir := filepath.Join(out, "conformance", "reports", "v1.5", ImplementationSlug)
	gotReport, err := os.ReadFile(filepath.Join(dir, "standard-v1.0.7-default-report.yaml"))
	if err != nil {
		t.Fatalf("reading published report: %v", err)
	}
	if string(gotReport) != goodReport {
		t.Error("published report is not byte-identical to the input")
	}

	readme, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading published README: %v", err)
	}
	if !bytes.Contains(readme, []byte("v1.0.7 report")) {
		t.Error("README does not link the report")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading output dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("output dir has %d entries, want exactly 2", len(entries))
	}
}

func TestRunIsIdempotent(t *testing.T) {
	reportPath := writeReport(t, goodReport)
	out := t.TempDir()

	if err := run([]string{"--tag", "v1.0.7", "--report", reportPath, "--out", out}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	readmePath := filepath.Join(out, "conformance", "reports", "v1.5", ImplementationSlug, "README.md")
	first, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}

	if err = run([]string{"--tag", "v1.0.7", "--report", reportPath, "--out", out}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("second run changed the README")
	}
}

func TestRunRequiresFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no tag", args: []string{"--report", "x", "--out", "y"}},
		{name: "no report and no CI environment", args: []string{"--tag", "v1.0.7", "--out", "y"}},
		{name: "no out", args: []string{"--tag", "v1.0.7", "--report", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args); err == nil {
				t.Error("run returned nil error, want a required-flag failure")
			}
		})
	}
}

func TestRunRejectsReproduceWithDefaultMode(t *testing.T) {
	reportPath := writeReport(t, goodReport)
	err := run([]string{
		"--tag", "v1.0.7", "--report", reportPath, "--out", t.TempDir(),
		"--reproduce", "should not be here",
	})
	if err == nil {
		t.Fatal("run accepted --reproduce with a default-mode report, want an error")
	}
}

func TestRunRequiresReproduceForNonDefaultMode(t *testing.T) {
	content := bytes.Replace([]byte(goodReport), []byte("mode: default"), []byte("mode: per-gateway"), 1)
	reportPath := writeReport(t, string(content))

	t.Run("missing is an error", func(t *testing.T) {
		err := run([]string{"--tag", "v1.0.7", "--report", reportPath, "--out", t.TempDir()})
		if err == nil {
			t.Fatal("run accepted a non-default mode with no --reproduce, want an error")
		}
	})

	t.Run("supplied is accepted", func(t *testing.T) {
		err := run([]string{
			"--tag", "v1.0.7", "--report", reportPath, "--out", t.TempDir(),
			"--reproduce", "One controller per Gateway.",
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	})
}

func TestRunWritesNothingWhenValidationFails(t *testing.T) {
	reportPath := writeReport(t, goodReport)
	out := t.TempDir()

	err := run([]string{"--tag", "v1.9.9", "--report", reportPath, "--out", out})
	if err == nil {
		t.Fatal("run accepted a mismatched tag, want a rule 2 error")
	}
	if _, err = os.Stat(filepath.Join(out, "conformance")); !os.IsNotExist(err) {
		t.Errorf("output tree exists after a validation failure: %v", err)
	}
}
