// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkAnalyzeBundle writes files under a temp dir and returns the dir.
func mkAnalyzeBundle(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func runAnalyze(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"analyze"}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestAnalyzeCmd_HumanDefault(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nSee [gone](gone.md).\n",
	})
	out, err := runAnalyze(t, dir)
	if err != nil {
		t.Fatalf("analyze returned error (should exit 0 on findings): %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "coverage") {
		t.Fatalf("human report should name the coverage dimension, got:\n%s", out)
	}
	if !strings.Contains(out, "gone.md") {
		t.Fatalf("human report should list the dangling link gone.md, got:\n%s", out)
	}
}

func TestAnalyzeCmd_JSON(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nSee [gone](gone.md).\n",
	})
	out, err := runAnalyze(t, "--json", dir)
	if err != nil {
		t.Fatalf("analyze --json returned error: %v", err)
	}
	var rep struct {
		Summary struct {
			Nodes int `json:"nodes"`
		} `json:"summary"`
		Coverage struct {
			DanglingLinks []struct {
				From   string `json:"from"`
				Target string `json:"target"`
			} `json:"dangling_links"`
		} `json:"coverage_gaps"`
	}
	if e := json.Unmarshal([]byte(out), &rep); e != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", e, out)
	}
	if rep.Summary.Nodes != 1 {
		t.Fatalf("want 1 node in summary, got %d", rep.Summary.Nodes)
	}
	if len(rep.Coverage.DanglingLinks) != 1 || rep.Coverage.DanglingLinks[0].Target != "gone.md" {
		t.Fatalf("want dangling gone.md in JSON, got %+v", rep.Coverage.DanglingLinks)
	}
}

// A clean corpus produces no findings but still exits 0 (report, not gate).
func TestAnalyzeCmd_ExitZeroCleanCorpus(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nBody.\n",
	})
	if _, err := runAnalyze(t, dir); err != nil {
		t.Fatalf("clean corpus should exit 0, got %v", err)
	}
}

// analyze must NOT accept --strict (that is lint's gate discriminator).
func TestAnalyzeCmd_NoStrictFlag(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nBody.\n",
	})
	if _, err := runAnalyze(t, "--strict", dir); err == nil {
		t.Fatalf("analyze should reject --strict (report, not gate)")
	}
}

func TestAnalyzeCmd_CustomStaleDaysInSummary(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nBody.\n",
	})
	out, err := runAnalyze(t, "--json", "--stale-days", "90", dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "\"stale_threshold_days\": 90") {
		t.Fatalf("want stale-days 90 reflected in summary, got:\n%s", out)
	}
}
