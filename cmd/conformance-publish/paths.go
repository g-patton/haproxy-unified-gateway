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
	"fmt"
	"path/filepath"
	"strings"

	confv1 "sigs.k8s.io/gateway-api/conformance/apis/v1"
)

// ImplementationSlug is the upstream folder name. It is a chosen slug, not a
// transformation of the organization, and must stay identical to the name used
// under conformance/list/implementations/ in the gateway-api repo.
const ImplementationSlug = "haproxy-unified-gateway"

// ImageTag maps a release tag to the published container image tag. The release
// workflow strips a leading "v" before pushing, so v1.0.7 ships as 1.0.7.
func ImageTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// VersionDir returns the upstream version directory, which is major.minor only.
func VersionDir(gatewayAPIVersion string) (string, error) {
	version := strings.TrimPrefix(gatewayAPIVersion, "v")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("gatewayAPIVersion %q is not major.minor", gatewayAPIVersion)
	}
	for _, part := range parts[:2] {
		if part == "" || strings.ContainsFunc(part, func(r rune) bool { return r < '0' || r > '9' }) {
			return "", fmt.Errorf("gatewayAPIVersion %q is not major.minor", gatewayAPIVersion)
		}
	}
	return "v" + parts[0] + "." + parts[1], nil
}

// ReportFileName is the upstream naming rule:
// <API channel>-<Implementation version>-<mode>-report.yaml.
func ReportFileName(channel, tag, mode string) string {
	return fmt.Sprintf("%s-%s-%s-report.yaml", channel, tag, mode)
}

// Paths are the two files the MR adds, under the output root.
type Paths struct {
	Dir    string
	Report string
	Readme string
}

// DerivePaths computes the upstream tree from the output root, the release tag,
// and the report's own contents.
func DerivePaths(out, tag string, report *confv1.ConformanceReport) (Paths, error) {
	versionDir, err := VersionDir(report.GatewayAPIVersion)
	if err != nil {
		return Paths{}, err
	}
	dir := filepath.Join(out, "conformance", "reports", versionDir, ImplementationSlug)
	return Paths{
		Dir:    dir,
		Report: filepath.Join(dir, ReportFileName(report.GatewayAPIChannel, tag, report.Mode)),
		Readme: filepath.Join(dir, "README.md"),
	}, nil
}
