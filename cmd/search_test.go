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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSearchCmdFixture lays down a bundle with a known edge shape for the
// search command tests: index -> tannin -> acidity (one-directional), plus an
// isolated auth node.
func writeSearchCmdFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":         "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md":   "---\ntype: Concept\ntitle: Tannin\ntags: [wine, chemistry]\n---\n\n# Tannin\n\nBinds proteins. See [Acidity](acidity.md).\n",
		"wine/acidity.md":  "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\npH and mouthfeel.\n",
		"security/auth.md": "---\ntype: Playbook\ntitle: Authentication\ntags: [security]\n---\n\n# Authentication\n\nToken rotation.\n",
	}
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

func TestSearchCmd_LexicalDefault(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	out, err := runOKF(t, "search", "tannin", dir)
	if err != nil {
		t.Fatalf("search errored: %v\n%s", err, out)
	}
	if !strings.Contains(out, "wine/tannin.md") {
		t.Fatalf("expected tannin.md in results:\n%s", out)
	}
	// index.md is reserved and must never appear.
	if strings.Contains(out, "index.md") {
		t.Fatalf("reserved index.md leaked into results:\n%s", out)
	}
}

func TestSearchCmd_FieldRestriction(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	// "wine" is a tag on two nodes but appears in no title.
	out, err := runOKF(t, "search", "wine", dir, "--field", "tag")
	if err != nil {
		t.Fatalf("search --field tag errored: %v\n%s", err, out)
	}
	if !strings.Contains(out, "wine/tannin.md") || !strings.Contains(out, "wine/acidity.md") {
		t.Fatalf("tag search should find both wine-tagged nodes:\n%s", out)
	}
	if strings.Contains(out, "security/auth.md") {
		t.Fatalf("auth.md is not wine-tagged:\n%s", out)
	}
}

func TestSearchCmd_UnknownFieldErrors(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	_, err := runOKF(t, "search", "tannin", dir, "--field", "banana")
	if err == nil {
		t.Fatalf("unknown --field should exit non-zero")
	}
}

func TestSearchCmd_Neighbors(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	out, err := runOKF(t, "search", "--neighbors", "wine/acidity.md", dir)
	if err != nil {
		t.Fatalf("search --neighbors errored: %v\n%s", err, out)
	}
	// tannin links acidity one-directionally; undirected traversal finds it.
	if !strings.Contains(out, "wine/tannin.md") {
		t.Fatalf("neighbors of acidity should include tannin:\n%s", out)
	}
}

func TestSearchCmd_NeighborsUnknownNodeErrors(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	_, err := runOKF(t, "search", "--neighbors", "does/not/exist.md", dir)
	if err == nil {
		t.Fatalf("--neighbors on unknown node should exit non-zero")
	}
}

func TestSearchCmd_JSONOutput(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	out, err := runOKF(t, "search", "tannin", dir, "--json")
	if err != nil {
		t.Fatalf("search --json errored: %v\n%s", err, out)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, out)
	}
	if len(results) == 0 {
		t.Fatalf("expected at least one JSON result:\n%s", out)
	}
}

func TestSearchCmd_JSONDeterministic(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	out1, _ := runOKF(t, "search", "wine", dir, "--field", "tag", "--json")
	out2, _ := runOKF(t, "search", "wine", dir, "--field", "tag", "--json")
	if out1 != out2 {
		t.Fatalf("search --json not byte-identical across runs")
	}
}

func TestSearchCmd_NoQueryNoNeighborsErrors(t *testing.T) {
	// Zero positionals and no --neighbors: nothing to search for.
	_, err := runOKF(t, "search")
	if err == nil {
		t.Fatalf("search with neither a query nor --neighbors should error")
	}
}

func TestSearchCmd_QueryAndNeighborsMutuallyExclusive(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	_, err := runOKF(t, "search", "tannin", "--neighbors", "wine/tannin.md", dir)
	if err == nil {
		t.Fatalf("a lexical query and --neighbors together should error")
	}
}

func TestSearchCmd_NoMatchesIsNotAnError(t *testing.T) {
	dir := writeSearchCmdFixture(t)
	out, err := runOKF(t, "search", "zzz-nonexistent-term", dir)
	if err != nil {
		t.Fatalf("a zero-result query is not an error: %v\n%s", err, out)
	}
}
