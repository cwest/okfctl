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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// semanticBundle writes a bundle with two near-identical unlinked nodes plus an
// unrelated one, then builds a real index over it with the deterministic hash
// embedder (no model download needed for a wiring test).
func semanticBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.md", "---\ntitle: Index\n---\n\n# Index\n")
	// Same words, so even the lexical hash embedder scores these near 1.0.
	write("wine/tannin.md", "---\ntitle: Tannin\ntype: concept\n---\n\nastringent polyphenol grip structure\n")
	write("wine/astringency.md", "---\ntitle: Astringency\ntype: concept\n---\n\nastringent polyphenol grip structure\n")
	write("wine/unrelated.md", "---\ntitle: Unrelated\ntype: concept\n---\n\nzebra xylophone quasar\n")
	return dir
}

func buildTestIndex(t *testing.T, dir string) {
	t.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := search.BuildIndex(b, search.NewHashEmbedder(), nil)
	p := filepath.Join(dir, ".okfctl", "index.db")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
}

func TestLint_SemanticFlagMissingIndex(t *testing.T) {
	dir := semanticBundle(t) // no index built
	_, err := runOKF(t, "lint", "--semantic", dir)
	if err == nil {
		t.Fatal("lint --semantic with no index must error, not silently skip")
	}
	// A silent structural-only fallback would let CI think semantic checks ran.
	if !strings.Contains(err.Error(), "index build") {
		t.Errorf("error should name the fix, got %v", err)
	}
}

func TestLint_WithoutSemanticUnchanged(t *testing.T) {
	dir := semanticBundle(t)
	buildTestIndex(t, dir) // index EXISTS but must not be read without the flag

	plain, err := runOKF(t, "lint", dir)
	if err != nil {
		t.Fatalf("plain lint: %v", err)
	}
	if strings.Contains(plain, "similar-unlinked") ||
		strings.Contains(plain, "semantically similar") {
		t.Errorf("semantic findings leaked into non-semantic lint:\n%s", plain)
	}
}

func TestLint_SemanticFindsUnlinkedPair(t *testing.T) {
	dir := semanticBundle(t)
	buildTestIndex(t, dir)

	out, err := runOKF(t, "lint", "--semantic", dir)
	if err != nil {
		t.Fatalf("lint --semantic: %v", err)
	}
	if !strings.Contains(out, "no link between them") {
		t.Errorf("want a similar-unlinked finding for the duplicate pair, got:\n%s", out)
	}
	if !strings.Contains(out, "wine/astringency.md") || !strings.Contains(out, "wine/tannin.md") {
		t.Errorf("finding should name both nodes, got:\n%s", out)
	}
}

func TestLint_SemanticThresholdFlags(t *testing.T) {
	dir := semanticBundle(t)
	buildTestIndex(t, dir)

	// A threshold above any achievable score suppresses the pair finding.
	out, err := runOKF(t, "lint", "--semantic", "--similarity-threshold", "1.01", dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if strings.Contains(out, "no link between them") {
		t.Errorf("threshold 1.01 should suppress every pair, got:\n%s", out)
	}

	// A floor above any achievable score isolates every node.
	out, err = runOKF(t, "lint", "--semantic", "--isolation-floor", "1.01", dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if !strings.Contains(out, "dead concept") {
		t.Errorf("floor 1.01 should isolate every node, got:\n%s", out)
	}
}

func TestLint_SemanticStrictExitsNonZero(t *testing.T) {
	dir := semanticBundle(t)
	buildTestIndex(t, dir)
	if _, err := runOKF(t, "lint", "--semantic", "--strict", dir); err == nil {
		t.Error("--strict with findings must exit non-zero")
	}
}
