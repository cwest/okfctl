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
	// --stale-days is a deprecated alias of --aging-days; both summary keys
	// carry the value (stale_threshold_days is the deprecated JSON alias).
	for _, want := range []string{"\"stale_threshold_days\": 90", "\"aging_threshold_days\": 90"} {
		if !strings.Contains(out, want) {
			t.Fatalf("want %s in summary, got:\n%s", want, out)
		}
	}
}

func TestAnalyzeCmd_AgingDaysFlag(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\n---\n\n# A\n\nBody.\n",
	})
	out, err := runAnalyze(t, "--json", "--aging-days", "45", dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "\"aging_threshold_days\": 45") {
		t.Fatalf("want aging-days 45 reflected in summary, got:\n%s", out)
	}
}

// OKF v0.3 §5.5/§11.2: the report separates real staleness (stale_after) from
// the age advisory. Positive control: a past stale_after is listed under
// stale_after and the human "Stale" section. Negative control: an undated node
// with no stale_after is only "aging", never stale.
func TestAnalyzeCmd_StaleAfterSeparateFromAging_Section5_5(t *testing.T) {
	dir := mkAnalyzeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [S](s.md)\n- [U](u.md)\n- [M](m.md)\n",
		"s.md":     "---\ntype: Concept\ntitle: S\nstale_after: 2000-01-01\n---\n\n# S\n\nBody.\n",
		"u.md":     "---\ntype: Concept\ntitle: U\n---\n\n# U\n\nBody.\n",
		"m.md":     "---\ntype: Concept\ntitle: M\nstale_after: someday\n---\n\n# M\n\nBody.\n",
	})
	out, err := runAnalyze(t, "--json", dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	var rep struct {
		Freshness struct {
			StaleAfter []struct {
				Path       string `json:"path"`
				StaleAfter string `json:"stale_after"`
				Malformed  bool   `json:"malformed"`
			} `json:"stale_after"`
			Aging []struct {
				Path string `json:"path"`
			} `json:"aging"`
			Stale []struct {
				Path string `json:"path"`
			} `json:"stale"`
		} `json:"freshness"`
	}
	if e := json.Unmarshal([]byte(out), &rep); e != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", e, out)
	}
	sa := rep.Freshness.StaleAfter
	if len(sa) != 2 || sa[0].Path != "m.md" || !sa[0].Malformed || sa[0].StaleAfter != "someday" ||
		sa[1].Path != "s.md" || sa[1].Malformed || sa[1].StaleAfter != "2000-01-01" {
		t.Fatalf("want m.md (malformed) and s.md stale via stale_after, got %+v", sa)
	}
	if len(rep.Freshness.Aging) != 3 || len(rep.Freshness.Stale) != 3 {
		t.Fatalf("want all 3 undated nodes aging and mirrored in deprecated stale, got aging=%+v stale=%+v",
			rep.Freshness.Aging, rep.Freshness.Stale)
	}

	human, err := runAnalyze(t, dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for _, want := range []string{
		"## Stale — past stale_after or malformed stale_after (§5.5)",
		"  - s.md (stale_after: 2000-01-01)",
		"  - m.md (stale_after: \"someday\" is malformed; never fresh)",
		"## Aging — not reviewed in 180 days, or undated (advisory, not staleness)",
		"  - u.md (age: n/a, basis: (none))",
	} {
		if !strings.Contains(human, want) {
			t.Fatalf("human report missing %q, got:\n%s", want, human)
		}
	}
	if strings.Contains(human, "Freshness — stale") {
		t.Fatalf("human report must no longer label the age heuristic as stale:\n%s", human)
	}
}
