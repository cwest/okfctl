// Copyright 2026 Casey West
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

package okf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_CountsConceptNodes(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "testdata", "good-bundle"))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// index.md and log.md are reserved, not concept nodes.
	if got := len(b.Nodes); got != 2 {
		t.Fatalf("concept nodes = %d, want 2 (%v)", got, keys(b.Nodes))
	}
	if _, ok := b.Nodes["wine/tannin.md"]; !ok {
		t.Errorf("missing node wine/tannin.md; have %v", keys(b.Nodes))
	}
}

func TestLoad_ExtractsEdgesFromLinks(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "testdata", "good-bundle"))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	edges := b.OutboundLinks("wine/tannin.md")
	found := false
	for _, e := range edges {
		if e == "wine/acidity.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edge tannin -> acidity, got %v", edges)
	}
}

func TestBuildEdges_TitledLinkStillResolves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	writeFile(t, dir, "a.md", "---\ntype: Reference\n---\n# A\n\nSee [b](b.md \"The B Node\").\n")
	writeFile(t, dir, "b.md", "---\ntype: Reference\n---\n# B\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !linksTo(b, "a.md", "b.md") {
		t.Errorf("titled link a.md -> b.md was dropped; edges=%v", b.OutboundLinks("a.md"))
	}
}

func TestBuildEdges_ImageIsNotAnEdge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	writeFile(t, dir, "a.md", "---\ntype: Reference\n---\n# A\n\n![diagram](b.md)\n")
	writeFile(t, dir, "b.md", "---\ntype: Reference\n---\n# B\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if linksTo(b, "a.md", "b.md") {
		t.Errorf("image ![](b.md) must NOT be an edge; edges=%v", b.OutboundLinks("a.md"))
	}
}

func TestLoad_ReadsBundleOkfVersion(t *testing.T) {
	dir := t.TempDir()
	// A bundle authored under a different spec version must report THAT version,
	// not the compiled-in constant.
	writeFile(t, dir, ".okf", "okf_version: 0.9\n")
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b.OkfVersion != "0.9" {
		t.Errorf("OkfVersion = %q, want 0.9 (the bundle's own .okf)", b.OkfVersion)
	}
}

func TestLoad_MissingDotOkfFallsBackToSpecVersion(t *testing.T) {
	dir := t.TempDir()
	// No .okf file at all: fall back to the build's SpecVersion rather than "".
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b.OkfVersion != SpecVersion {
		t.Errorf("OkfVersion = %q, want fallback %q", b.OkfVersion, SpecVersion)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func linksTo(b *Bundle, from, to string) bool {
	for _, e := range b.OutboundLinks(from) {
		if e == to {
			return true
		}
	}
	return false
}

func keys(m map[string]*Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
