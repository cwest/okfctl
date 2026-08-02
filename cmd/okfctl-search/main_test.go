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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func runPlugin(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newSearchCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func writeSearchBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":        "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [T](wine/tannin.md)\n",
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nTannin gives structure and astringency.\n",
		"wine/acidity.md": "---\ntype: Concept\ntitle: Acidity\n---\n\n# Acidity\n\nAcidity gives freshness and lift.\n",
	}
	for rel, c := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeScopedBundle lays down nodes across two path prefixes, two types, and a
// tag so the CLI filter flags have something to bite on.
func writeScopedBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\ntags: [red]\n---\n\n# Tannin\n\nTannin gives structure and astringency to wine.\n",
		"wine/pairing.md": "---\ntype: Playbook\ntitle: Pairing\n---\n\n# Pairing\n\nPair structure and acidity with food.\n",
		"coffee/roast.md": "---\ntype: Concept\ntitle: Roast\n---\n\n# Roast\n\nRoast level shapes structure and acidity in coffee.\n",
	}
	for rel, c := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPlugin_IndexBuildThenSemantic(t *testing.T) {
	dir := writeSearchBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	idx := filepath.Join(dir, ".okfctl", "index.db")
	if _, err := os.Stat(idx); err != nil {
		t.Fatalf("index.db not written: %v", err)
	}
	data, _ := os.ReadFile(idx)
	if !strings.Contains(string(data), "hash-test-embedder") {
		t.Errorf("index.db should record the model; got %s", data)
	}
	out, err := runPlugin(t, "--semantic", "tannin structure astringency", dir)
	if err != nil {
		t.Fatalf("semantic query: %v", err)
	}
	if !strings.Contains(out, "wine/tannin.md") {
		t.Errorf("semantic query should surface wine/tannin.md; got %q", out)
	}
}

// TestPlugin_SemanticPrintsSnippet pins that a semantic query prints the matched
// passage snippet alongside the score and path, so the caller sees the passage
// that answered the query rather than just a filename.
func TestPlugin_SemanticPrintsSnippet(t *testing.T) {
	dir := writeSearchBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	out, err := runPlugin(t, "--semantic", "tannin structure astringency", dir)
	if err != nil {
		t.Fatalf("semantic query: %v", err)
	}
	// The snippet text from the Tannin node body must appear in the output.
	if !strings.Contains(out, "structure and astringency") {
		t.Errorf("semantic query should print the matched snippet; got %q", out)
	}
}

func TestPlugin_Related(t *testing.T) {
	dir := writeSearchBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "related", "wine/tannin.md", dir)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if strings.Contains(out, "wine/tannin.md") {
		t.Errorf("related should exclude the node itself; got %q", out)
	}
	if !strings.Contains(out, "wine/acidity.md") {
		t.Errorf("related should list the neighbor; got %q", out)
	}
}

// TestPlugin_Model2vecNeedsModelPath pins the end-to-end contract for an
// unconfigured model2vec run: the plugin must fail with an actionable message
// rather than silently falling back to the hash embedder, which would answer
// the query with the wrong vectors.
func TestPlugin_Model2vecNeedsModelPath(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir()) // ignore the developer's real config
	dir := writeSearchBundle(t)
	_, err := runPlugin(t, "--embedder", "model2vec", "index", "build", dir)
	if err == nil {
		t.Fatal("model2vec with no configured model_path should error, got nil")
	}
	for _, want := range []string{"model_path", "--model-path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should tell the user how to fix it (%q); got %v", want, err)
		}
	}
}

// TestPlugin_FilterPath pins that --path restricts semantic results to nodes
// under the prefix; a competing coffee node must not appear.
func TestPlugin_FilterPath(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--path", "wine/", dir)
	if err != nil {
		t.Fatalf("semantic --path: %v", err)
	}
	if strings.Contains(out, "coffee/") {
		t.Errorf("--path wine/ leaked a coffee result: %q", out)
	}
	if !strings.Contains(out, "wine/") {
		t.Errorf("--path wine/ returned no wine results: %q", out)
	}
}

// TestPlugin_FilterType pins that --type restricts to a single §4.1 type.
func TestPlugin_FilterType(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--type", "Playbook", dir)
	if err != nil {
		t.Fatalf("semantic --type: %v", err)
	}
	if !strings.Contains(out, "wine/pairing.md") {
		t.Errorf("--type Playbook should surface wine/pairing.md; got %q", out)
	}
	if strings.Contains(out, "wine/tannin.md") || strings.Contains(out, "coffee/roast.md") {
		t.Errorf("--type Playbook leaked a Concept node: %q", out)
	}
}

// TestPlugin_FilterTag pins that --tag restricts to nodes carrying that §4.1 tag.
func TestPlugin_FilterTag(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure", "--tag", "red", dir)
	if err != nil {
		t.Fatalf("semantic --tag: %v", err)
	}
	if !strings.Contains(out, "wine/tannin.md") {
		t.Errorf("--tag red should surface wine/tannin.md; got %q", out)
	}
	if strings.Contains(out, "wine/pairing.md") || strings.Contains(out, "coffee/roast.md") {
		t.Errorf("--tag red leaked an untagged node: %q", out)
	}
}

// TestPlugin_FilterZeroMatchEmptyNotError is the CLI-level negative control: a
// type filter matching nothing prints no result lines and exits 0 — not an error,
// and not a silent unfiltered fall-back.
func TestPlugin_FilterZeroMatchEmptyNotError(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure", "--type", "NoSuchType", dir)
	if err != nil {
		t.Fatalf("zero-match filter must exit 0, got %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("zero-match filter must print nothing (no silent unfiltered fall-back); got %q", out)
	}
}

// TestPlugin_UnfilteredUnchanged is the CLI filter control: a query with no filter
// flags produces the same output as before filters existed — proven by comparing a
// bare query against one that passes empty flags.
func TestPlugin_UnfilteredUnchanged(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	bare, err := runPlugin(t, "--semantic", "structure acidity", dir)
	if err != nil {
		t.Fatal(err)
	}
	// Empty-string filter flags are the no-op path and must match the bare output.
	empty, err := runPlugin(t, "--semantic", "structure acidity", "--path", "", "--type", "", "--tag", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if bare != empty {
		t.Errorf("empty filter flags changed output:\nbare=%q\nempty=%q", bare, empty)
	}
}

// TestPlugin_HalfLifeAcceptedAndUnsetUnchanged pins that --half-life is a real
// flag, and that WITHOUT it the ranking is unchanged (decay is off by default).
func TestPlugin_HalfLifeAcceptedAndUnsetUnchanged(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	// Unset: baseline.
	base, err := runPlugin(t, "--semantic", "structure acidity", dir)
	if err != nil {
		t.Fatal(err)
	}
	// half-life=0 must be treated as unset (no decay) and produce identical output.
	off, err := runPlugin(t, "--semantic", "structure acidity", "--half-life", "0", dir)
	if err != nil {
		t.Fatalf("--half-life 0: %v", err)
	}
	if base != off {
		t.Errorf("--half-life 0 (off) changed ranking:\nbase=%q\noff=%q", base, off)
	}
	// A set half-life must at least run without error and return results.
	on, err := runPlugin(t, "--semantic", "structure acidity", "--half-life", "30", dir)
	if err != nil {
		t.Fatalf("--half-life 30: %v", err)
	}
	if strings.TrimSpace(on) == "" {
		t.Errorf("--half-life 30 returned no results")
	}
}

// TestSnippetPreview_TruncatesOnRuneBoundary pins that a snippet longer than the
// preview cap is truncated without splitting a multi-byte UTF-8 rune. The KB
// carries non-ASCII text (curly quotes, em-dashes, accented names), so a
// byte-boundary cut would emit a mangled partial byte before the ellipsis.
func TestSnippetPreview_TruncatesOnRuneBoundary(t *testing.T) {
	// 198 ASCII runes then an em-dash (3 bytes: 0xE2 0x80 0x94). A byte-boundary
	// cut at 200 lands inside the em-dash (bytes 199..201), splitting the rune.
	in := strings.Repeat("a", 198) + "—" + strings.Repeat("b", 50)
	out := snippetPreview(in)

	if !utf8.ValidString(out) {
		t.Errorf("snippetPreview produced invalid UTF-8 (mid-rune cut): %q", out)
	}
	if strings.ContainsRune(out, utf8.RuneError) {
		t.Errorf("snippetPreview left a replacement-char/partial rune: %q", out)
	}
	// It must still truncate: 248 input runes exceed the 200-rune cap.
	if !strings.HasSuffix(out, "…") {
		t.Errorf("long snippet should be truncated with an ellipsis; got %q", out)
	}
	// The kept body is 200 runes plus the ellipsis.
	if got := utf8.RuneCountInString(out); got != 201 {
		t.Errorf("truncated preview should be 200 runes + ellipsis = 201 runes; got %d (%q)", got, out)
	}
}
