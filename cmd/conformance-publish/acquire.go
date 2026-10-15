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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// reportArtifactName is the file the conformance CI job uploads.
const reportArtifactName = "conformance-report"

type pipeline struct {
	Status string `json:"status"`
	ID     int    `json:"id"`
}

type gitlabJob struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// AcquireReport downloads the conformance report artifact for a ref, returning a
// path to the extracted file and a cleanup func the caller must call. When
// pipelineID is 0 the latest successful pipeline for ref is used.
//
// This is the out-of-band path: the publish job runs the suite itself and passes
// --report, so nothing in the normal flow depends on it.
func AcquireReport(ctx context.Context, client *http.Client, baseURL, token, project, ref string, pipelineID int) (string, func(), error) {
	if token == "" {
		return "", nil, errors.New("GITLAB_TOKEN is not set")
	}
	if client == nil {
		client = &http.Client{}
	}

	id := pipelineID
	if id == 0 {
		var pipelines []pipeline
		path := fmt.Sprintf("%s/projects/%s/pipelines?ref=%s&status=success",
			baseURL, url.PathEscape(project), url.QueryEscape(ref))
		if err := getJSON(ctx, client, token, path, &pipelines); err != nil {
			return "", nil, err
		}
		if len(pipelines) == 0 {
			return "", nil, fmt.Errorf("no successful pipeline for ref %q; pass --report or --pipeline", ref)
		}
		id = pipelines[0].ID
	}

	var jobs []gitlabJob
	if err := getJSON(ctx, client, token,
		fmt.Sprintf("%s/projects/%s/pipelines/%d/jobs", baseURL, url.PathEscape(project), id), &jobs); err != nil {
		return "", nil, err
	}

	jobID := 0
	for _, j := range jobs {
		if strings.HasPrefix(j.Name, reportArtifactName) || strings.HasPrefix(j.Name, "conformance-GW-API") {
			jobID = j.ID
			break
		}
	}
	if jobID == 0 {
		return "", nil, fmt.Errorf("pipeline %d has no job producing %s", id, reportArtifactName)
	}

	body, err := getBytes(ctx, client, token,
		fmt.Sprintf("%s/projects/%s/jobs/%d/artifacts", baseURL, url.PathEscape(project), jobID))
	if err != nil {
		return "", nil, err
	}

	return extractReport(body)
}

// extractReport pulls the report out of the job-artifacts zip.
func extractReport(archive []byte) (string, func(), error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", nil, fmt.Errorf("job artifacts are not a zip archive: %w", err)
	}
	for _, file := range reader.File {
		if !strings.HasSuffix(file.Name, ".yaml") || !strings.Contains(file.Name, reportArtifactName) {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return "", nil, fmt.Errorf("opening %s in artifacts: %w", file.Name, err)
		}
		content, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			return "", nil, fmt.Errorf("reading %s from artifacts: %w", file.Name, err)
		}
		dir, err := os.MkdirTemp("", "conformance-publish-")
		if err != nil {
			return "", nil, fmt.Errorf("creating temp dir: %w", err)
		}
		path := filepath.Join(dir, filepath.Base(file.Name))
		if err = os.WriteFile(path, content, 0o600); err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, fmt.Errorf("writing %s: %w", path, err)
		}
		return path, func() { _ = os.RemoveAll(dir) }, nil
	}
	return "", nil, fmt.Errorf("no %s yaml found in the job artifacts", reportArtifactName)
}

func getJSON(ctx context.Context, client *http.Client, token, path string, out any) error {
	body, err := getBytes(ctx, client, token, path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing response from %s: %w", path, err)
	}
	return nil
}

func getBytes(ctx context.Context, client *http.Client, token, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("PRIVATE-TOKEN", token) //nolint:canonicalheader
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("requesting %s: status %d", path, resp.StatusCode)
	}
	return body, nil
}
