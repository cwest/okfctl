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

package okf_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	internalokf "github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/pkg/okf"
)

// writeFixtureDir writes a small mixed bundle (clean spec floor, with a couple
// of lint-worthy shapes) and returns its root. It intentionally does NOT call
// Load: several tests want the raw root to Load through both the facade and the
// internal package independently, or to snapshot the tree before/after.
func writeFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":         "# Index\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md":   "---\ntype: Concept\ntitle: Tannin\ntags: [wine, chemistry]\n---\n\n# Tannin\n\nTannins bind proteins. See [Acidity](acidity.md).\n",
		"wine/acidity.md":  "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\nMouthfeel and pH. See [Tannin](tannin.md).\n",
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

// TestFacade_TypeAliasesAreIdentical proves the facade re-exports domain types
// as Go type aliases, not copies: a *okf.Bundle IS an *internalokf.Bundle, so it
// is assignable to the internal type and there is exactly one Bundle in the
// program. A copy-type ("type Bundle struct{...}") would fail to compile here.
func TestFacade_TypeAliasesAreIdentical(t *testing.T) {
	dir := writeFixtureDir(t)

	var facadeB *okf.Bundle
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load: %v", err)
	}
	facadeB = b

	// Cross-assign in both directions — only aliases (not distinct copy types)
	// permit this without a conversion.
	var internalB *internalokf.Bundle = facadeB
	var backToFacade *okf.Bundle = internalB
	if backToFacade != facadeB {
		t.Fatal("round-trip through the internal type changed the pointer")
	}

	// reflect confirms identical type identity, not merely structural sameness.
	if reflect.TypeOf(facadeB) != reflect.TypeOf(internalB) {
		t.Fatalf("Bundle is not a type alias: facade %v != internal %v",
			reflect.TypeOf(facadeB), reflect.TypeOf(internalB))
	}
	var _ okf.Node = internalokf.Node{}
	var _ okf.Finding = internalokf.Finding{}
	var _ okf.LintFinding = internalokf.LintFinding{}
	var _ okf.Graph = internalokf.Graph{}
	var _ okf.SearchResult = internalokf.SearchResult{}
	var _ okf.NeighborResult = internalokf.NeighborResult{}
	var _ okf.AnalyzeReport = internalokf.AnalyzeReport{}
}

// snapshotTree returns a stable map of bundle-relative path -> "mtime|sha256"
// for every file under root, so a before/after comparison catches any content
// OR metadata mutation.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = info.ModTime().String() + "|" + hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// TestFacade_ReadOnly_TreeUnchanged exercises the ENTIRE facade over a fixture
// bundle and asserts the on-disk tree is byte-for-byte and mtime-for-mtime
// unchanged. No exported facade function may mutate the bundle (the card's
// write-path-exclusion invariant).
func TestFacade_ReadOnly_TreeUnchanged(t *testing.T) {
	dir := writeFixtureDir(t)
	before := snapshotTree(t, dir)

	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_ = okf.Validate(b)
	_ = okf.Lint(b, okf.LintOptions{})
	_ = okf.BuildGraph(b)
	_ = okf.Search(b, "tannin", okf.FieldAny)
	_, _ = okf.Neighborhood(b, "wine/tannin.md", 2)
	_ = okf.Analyze(b, okf.AnalyzeOptions{})

	after := snapshotTree(t, dir)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("facade mutated the bundle tree:\nbefore=%v\nafter =%v", before, after)
	}
}

// TestFacade_ValidateEqualsInternal proves Validate through the facade returns
// findings identical to the internal package (which the CLI renders verbatim,
// so this is the CLI-equivalence assertion at the finding-slice level).
func TestFacade_ValidateEqualsInternal(t *testing.T) {
	dir := writeFixtureDir(t)

	fb, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load: %v", err)
	}
	ib, err := internalokf.Load(dir)
	if err != nil {
		t.Fatalf("internal Load: %v", err)
	}
	if got, want := okf.Validate(fb), internalokf.Validate(ib); !reflect.DeepEqual(got, want) {
		t.Fatalf("Validate mismatch:\nfacade  =%v\ninternal=%v", got, want)
	}
}

// TestFacade_LintEqualsInternal proves Lint through the facade returns findings
// identical to the internal package over the same fixture.
func TestFacade_LintEqualsInternal(t *testing.T) {
	dir := writeFixtureDir(t)

	fb, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load: %v", err)
	}
	ib, err := internalokf.Load(dir)
	if err != nil {
		t.Fatalf("internal Load: %v", err)
	}
	got := okf.Lint(fb, okf.LintOptions{})
	want := internalokf.Lint(ib, internalokf.LintOptions{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lint mismatch:\nfacade  =%v\ninternal=%v", got, want)
	}
}

// TestFacade_SearchAndGraphEqualInternal covers the remaining read-path
// delegations: Search, Neighborhood, BuildGraph, Analyze all forward to the
// internal implementation unchanged.
func TestFacade_SearchAndGraphEqualInternal(t *testing.T) {
	dir := writeFixtureDir(t)
	fb, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load: %v", err)
	}
	ib, err := internalokf.Load(dir)
	if err != nil {
		t.Fatalf("internal Load: %v", err)
	}

	if got, want := okf.Search(fb, "wine", okf.FieldTag), internalokf.Search(ib, "wine", internalokf.FieldTag); !reflect.DeepEqual(got, want) {
		t.Fatalf("Search mismatch:\nfacade=%v\ninternal=%v", got, want)
	}
	fn, fok := okf.Neighborhood(fb, "wine/tannin.md", 1)
	in, iok := internalokf.Neighborhood(ib, "wine/tannin.md", 1)
	if fok != iok || !reflect.DeepEqual(fn, in) {
		t.Fatalf("Neighborhood mismatch:\nfacade=(%v,%v)\ninternal=(%v,%v)", fn, fok, in, iok)
	}
	if got, want := okf.BuildGraph(fb), internalokf.BuildGraph(ib); !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildGraph mismatch:\nfacade=%v\ninternal=%v", got, want)
	}
	if got, want := okf.Analyze(fb, okf.AnalyzeOptions{}), internalokf.Analyze(ib, internalokf.AnalyzeOptions{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Analyze mismatch:\nfacade=%v\ninternal=%v", got, want)
	}
}

// TestFacade_V01FallbackFlowsThrough is the AGENTS.md §4 legacy control: a
// change touching the read path must still be exercised against a v0.1 bundle.
// The facade adds no logic, so the §13.1 fallbacks (legacy `timestamp` for
// `generated.at`, legacy body `# Citations` for `sources`) are the internal
// package's — this test proves they flow through the delegation unchanged by
// asserting the facade's Load+Validate+Analyze over a v0.1-shaped bundle equal
// the internal package's over the same bytes. v0.2 support must change nothing
// for a v0.1 bundle (the negative control).
func TestFacade_V01FallbackFlowsThrough(t *testing.T) {
	dir := t.TempDir()
	// A v0.1-shaped node: legacy `timestamp` provenance and a body `# Citations`
	// list rather than the v0.2 `generated.at` / frontmatter `sources`.
	node := "---\n" +
		"type: Concept\n" +
		"title: Tannin\n" +
		"timestamp: 2026-01-02T03:04:05Z\n" +
		"---\n\n" +
		"# Tannin\n\nTannins bind proteins.\n\n" +
		"# Citations\n\n- https://example.com/tannin\n"
	files := map[string]string{
		"index.md":       "# Index\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md": node,
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

	fb, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load v0.1 bundle: %v", err)
	}
	ib, err := internalokf.Load(dir)
	if err != nil {
		t.Fatalf("internal Load v0.1 bundle: %v", err)
	}

	// A v0.1 bundle validates clean (the fallbacks make it consumable).
	if got := okf.Validate(fb); len(got) != 0 {
		t.Fatalf("v0.1 bundle must validate clean through the facade; got: %v", got)
	}
	if fv, iv := okf.Validate(fb), internalokf.Validate(ib); !reflect.DeepEqual(fv, iv) {
		t.Fatalf("v0.1 Validate mismatch: facade=%v internal=%v", fv, iv)
	}
	// Analyze reads the freshness/provenance families; the fallback path must
	// produce identical reports through the facade and internally.
	if fa, ia := okf.Analyze(fb, okf.AnalyzeOptions{}), internalokf.Analyze(ib, internalokf.AnalyzeOptions{}); !reflect.DeepEqual(fa, ia) {
		t.Fatalf("v0.1 Analyze mismatch through the facade")
	}
}

// realCorpus is the 254-node knowledge base per the card. Absent on a CI machine
// without the checkout, in which case the real-corpus test skips cleanly.
const realCorpus = "knowledge-base/bundles/knowledge"

func realCorpusPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	p := filepath.Join(home, "src", realCorpus)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("real corpus not present at %s: %v", p, err)
	}
	return p
}

// TestFacade_RealCorpusEquivalence is the Layer-3 control per AGENTS.md: over the
// real 254-node corpus, facade Validate and Lint return exactly what the internal
// package (and thus the CLI) returns — validate 0 findings, lint 0 findings. The
// corpus is lint-clean today, so this is a PURE NEGATIVE CONTROL (must stay 0);
// the positive control lives in the fixture equivalence tests above.
func TestFacade_RealCorpusEquivalence(t *testing.T) {
	dir := realCorpusPath(t)

	fb, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("facade Load real corpus: %v", err)
	}
	ib, err := internalokf.Load(dir)
	if err != nil {
		t.Fatalf("internal Load real corpus: %v", err)
	}

	fv := okf.Validate(fb)
	iv := internalokf.Validate(ib)
	if !reflect.DeepEqual(fv, iv) {
		t.Fatalf("real-corpus Validate mismatch: facade=%d internal=%d", len(fv), len(iv))
	}
	if len(fv) != 0 {
		t.Fatalf("real-corpus Validate expected 0 findings (negative control), got %d: %v", len(fv), fv)
	}

	fl := okf.Lint(fb, okf.LintOptions{})
	il := internalokf.Lint(ib, internalokf.LintOptions{})
	if !reflect.DeepEqual(fl, il) {
		t.Fatalf("real-corpus Lint mismatch: facade=%d internal=%d", len(fl), len(il))
	}
	if len(fl) != 0 {
		t.Fatalf("real-corpus Lint expected 0 findings (negative control), got %d: %v", len(fl), fl)
	}
}
