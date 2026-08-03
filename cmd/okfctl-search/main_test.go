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
	"fmt"
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

// writeGateTailBundle lays down six nodes where the token "acidity" appears in
// exactly two (a wine node and a coffee node) — a MINORITY, so a gate on
// "acidity" does not degrade as over-broad, and the wine/coffee split lets a
// --path filter be shown to constrain the appended lexical tail.
func writeGateTailBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"wine/tannin.md":    "---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nTannin gives structure and astringency.\n",
		"wine/acidity.md":   "---\ntype: Concept\ntitle: Wine Acidity\n---\n\n# Wine Acidity\n\nAcidity gives a wine freshness and lift.\n",
		"wine/body.md":      "---\ntype: Concept\ntitle: Body\n---\n\n# Body\n\nBody is the weight of the wine on the palate.\n",
		"coffee/roast.md":   "---\ntype: Concept\ntitle: Roast\n---\n\n# Roast\n\nRoast level shapes the coffee acidity and sweetness.\n",
		"coffee/grind.md":   "---\ntype: Concept\ntitle: Grind\n---\n\n# Grind\n\nGrind size controls extraction rate.\n",
		"coffee/brewing.md": "---\ntype: Concept\ntitle: Brewing\n---\n\n# Brewing\n\nBrewing temperature and time drive flavor.\n",
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

// TestPlugin_FilterPathOR pins the CLI OR contract: repeating --path unions the
// roots. --path wine/ --path coffee/ surfaces both, and is a superset of either
// prefix alone.
func TestPlugin_FilterPathOR(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--path", "wine/", "--path", "coffee/", dir)
	if err != nil {
		t.Fatalf("semantic --path OR: %v", err)
	}
	if !strings.Contains(out, "wine/") {
		t.Errorf("--path wine/ --path coffee/ dropped wine results: %q", out)
	}
	if !strings.Contains(out, "coffee/") {
		t.Errorf("--path wine/ --path coffee/ dropped coffee results: %q", out)
	}
}

// TestPlugin_FilterTypeOR pins that repeating --type unions the types.
func TestPlugin_FilterTypeOR(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--type", "Concept", "--type", "Playbook", dir)
	if err != nil {
		t.Fatalf("semantic --type OR: %v", err)
	}
	// Playbook (wine/pairing.md) and Concept (wine/tannin.md, coffee/roast.md) both survive.
	if !strings.Contains(out, "wine/pairing.md") {
		t.Errorf("--type Concept --type Playbook dropped the Playbook node: %q", out)
	}
	if !strings.Contains(out, "wine/tannin.md") && !strings.Contains(out, "coffee/roast.md") {
		t.Errorf("--type Concept --type Playbook dropped all Concept nodes: %q", out)
	}
}

// TestPlugin_NotPath is the headline negation at the CLI: --not-path research/
// removes an entire root. Here --not-path coffee/ must drop the coffee node while
// keeping the wine nodes, with no positive flag at all (empty positive = all).
func TestPlugin_NotPath(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--not-path", "coffee/", dir)
	if err != nil {
		t.Fatalf("semantic --not-path: %v", err)
	}
	if strings.Contains(out, "coffee/") {
		t.Errorf("--not-path coffee/ leaked a coffee result: %q", out)
	}
	if !strings.Contains(out, "wine/") {
		t.Errorf("--not-path coffee/ dropped the wine results too: %q", out)
	}
}

// TestPlugin_NotType and NotTag exclude by the other two §4.1 dimensions.
func TestPlugin_NotType(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--not-type", "Playbook", dir)
	if err != nil {
		t.Fatalf("semantic --not-type: %v", err)
	}
	if strings.Contains(out, "wine/pairing.md") {
		t.Errorf("--not-type Playbook leaked the Playbook node: %q", out)
	}
}

func TestPlugin_NotTag(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure astringency", "--not-tag", "red", dir)
	if err != nil {
		t.Fatalf("semantic --not-tag: %v", err)
	}
	if strings.Contains(out, "wine/tannin.md") {
		t.Errorf("--not-tag red leaked the tag:[red] node: %q", out)
	}
}

// TestPlugin_ExclusionBeatsInclusion pins the specified positive-then-exclude
// order at the CLI: --path wine/ --not-path wine/pairing.md keeps wine/tannin.md
// but drops the excluded sub-path.
func TestPlugin_ExclusionBeatsInclusion(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--path", "wine/", "--not-path", "wine/pairing.md", dir)
	if err != nil {
		t.Fatalf("semantic --path + --not-path: %v", err)
	}
	if strings.Contains(out, "wine/pairing.md") {
		t.Errorf("exclusion did not beat inclusion; wine/pairing.md survived: %q", out)
	}
	if strings.Contains(out, "coffee/") {
		t.Errorf("--path wine/ leaked a coffee result: %q", out)
	}
	if !strings.Contains(out, "wine/tannin.md") {
		t.Errorf("exclusion over-reached and dropped wine/tannin.md too: %q", out)
	}
}

// TestPlugin_NotPathCoveringEveryRoot: a negative filter covering every root
// returns no results with a clean exit (empty is not an error).
func TestPlugin_NotPathCoveringEveryRoot(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "structure acidity", "--not-path", "wine/", "--not-path", "coffee/", dir)
	if err != nil {
		t.Fatalf("negative filter covering every root must exit 0, got %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("negative filter covering every root must return empty output, got %q", out)
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

// parseTopPaths extracts the ranked bundle-relative paths from plugin output, in
// order. Each result line is "SCORE\tPATH"; snippet lines are indented and skipped.
func parseTopPaths(out string) []string {
	var paths []string
	for _, ln := range strings.Split(out, "\n") {
		if ln == "" || strings.HasPrefix(ln, "\t") {
			continue
		}
		fields := strings.Split(ln, "\t")
		if len(fields) >= 2 {
			paths = append(paths, fields[1])
		}
	}
	return paths
}

// writeDecayReproBundle lays down the issue's exact two-node repro: a strong OLD
// exact match (2023) and a weak FRESH near-noise node (2026) sharing incidental
// query words. With decay and no clamp, an aggressive half-life inverts them.
func writeDecayReproBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"docs/old-exact.md":  "---\ntype: Concept\ntitle: Bloom Filter Sizing\ngenerated:\n  by: synthetic\n  at: 2023-01-01T00:00:00Z\n---\n\n# Bloom Filter Sizing\n\n## Bloom Filter False Positive Bits Hashes Sizing\n\nBloom filter sizing bits hashes false positive rate bloom filter sizing bits\nhashes false positive rate bloom filter sizing bits hashes false positive rate.\n",
		"docs/fresh-weak.md": "---\ntype: Concept\ntitle: Weekly Status Roundup\ngenerated:\n  by: synthetic\n  at: 2026-08-01T00:00:00Z\n---\n\n# Weekly Status Roundup\n\n## Agenda Recap\n\nFilter positive agenda recap status roundup attendees followup parked owner\naction pending review draft revision comment summary snapshot digest triage\nboard column swimlane retro sprint cadence checkpoint readout brief rollup.\n",
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

const decayReproQuery = "bloom filter sizing false positive bits hashes"

// TestPlugin_DecayFloorPositiveControl is the card's POSITIVE control on the CLI:
// the issue's two-node repro at --half-life 90 with the DEFAULT --decay-floor
// (0.25). old-exact.md MUST rank above fresh-weak.md and MUST NOT score 0.0000.
// Assert the ordering, not just the score.
func TestPlugin_DecayFloorPositiveControl(t *testing.T) {
	dir := writeDecayReproBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", decayReproQuery, "--half-life", "90", dir)
	if err != nil {
		t.Fatalf("--half-life 90 (default decay-floor): %v", err)
	}
	paths := parseTopPaths(out)
	if len(paths) < 2 || paths[0] != "docs/old-exact.md" {
		t.Fatalf("default --decay-floor 0.25 must keep old-exact.md on top at half-life 90; got order %v\noutput=%q", paths, out)
	}
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "docs/old-exact.md") && strings.HasPrefix(ln, "0.0000\t") {
			t.Fatalf("old-exact.md scored 0.0000 even with the default clamp: %q", ln)
		}
	}
}

// TestPlugin_DecayFloorNegativeControl is the card's second NEGATIVE control on
// the CLI: --decay-floor 0 must restore today's exact unbounded behavior — the
// inversion returns (fresh-weak.md on top at half-life 90). Backward
// compatibility is opt-out-able and must be provable.
func TestPlugin_DecayFloorNegativeControl(t *testing.T) {
	dir := writeDecayReproBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", decayReproQuery, "--half-life", "90", "--decay-floor", "0", dir)
	if err != nil {
		t.Fatalf("--decay-floor 0: %v", err)
	}
	paths := parseTopPaths(out)
	if len(paths) < 2 || paths[0] != "docs/fresh-weak.md" {
		t.Fatalf("--decay-floor 0 must restore the unbounded inversion (fresh-weak.md on top); got order %v\noutput=%q", paths, out)
	}
}

// TestPlugin_MinRelevanceBothDirections is the card's --min-relevance control:
// a value above the weak node's raw cosine DROPS it entirely (positive); the
// default (--min-relevance 0) admits everything the ranker returned (negative).
func TestPlugin_MinRelevanceBothDirections(t *testing.T) {
	dir := writeDecayReproBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	// Learn the raw cosines (no decay) so the threshold is grounded, not guessed.
	raw, err := runPlugin(t, "--semantic", decayReproQuery, dir)
	if err != nil {
		t.Fatal(err)
	}
	var weakRaw float64
	for _, ln := range strings.Split(raw, "\n") {
		if strings.Contains(ln, "docs/fresh-weak.md") {
			if _, e := fmt.Sscanf(ln, "%f", &weakRaw); e != nil {
				t.Fatalf("could not parse fresh-weak raw score from %q", ln)
			}
		}
	}
	if weakRaw <= 0 {
		t.Fatalf("test premise broken: fresh-weak raw cosine should be > 0; got %.4f (raw=%q)", weakRaw, raw)
	}

	// Negative direction: default --min-relevance 0 admits BOTH nodes, unchanged.
	def, err := runPlugin(t, "--semantic", decayReproQuery, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseTopPaths(def); len(got) != 2 {
		t.Fatalf("default --min-relevance 0 must admit everything; got %v", got)
	}

	// Positive direction: a floor just above the weak node's raw cosine drops it.
	floor := fmt.Sprintf("%.4f", weakRaw+0.01)
	dropped, err := runPlugin(t, "--semantic", decayReproQuery, "--min-relevance", floor, dir)
	if err != nil {
		t.Fatalf("--min-relevance %s: %v", floor, err)
	}
	for _, p := range parseTopPaths(dropped) {
		if p == "docs/fresh-weak.md" {
			t.Fatalf("--min-relevance %s should DROP the sub-floor weak node; it survived. output=%q", floor, dropped)
		}
	}
	if !strings.Contains(dropped, "docs/old-exact.md") {
		t.Fatalf("--min-relevance %s dropped the strong node too; output=%q", floor, dropped)
	}
}

// TestPlugin_HelpTextDescribesClampedGuarantee pins that the --half-life help
// text no longer claims the un-clampable guarantee ("never promotes an
// irrelevant-but-fresh node") and instead points at the two bounds that actually
// deliver it (--decay-floor, --min-relevance).
func TestPlugin_HelpTextDescribesClampedGuarantee(t *testing.T) {
	out, err := runPlugin(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "never promotes an irrelevant-but-fresh node") {
		t.Errorf("--half-life help still makes the unreachable guarantee; got:\n%s", out)
	}
	for _, want := range []string{"--decay-floor", "--min-relevance"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output should document %s; got:\n%s", want, out)
		}
	}
}

// TestPlugin_LexicalGateAcceptedAndOffUnchanged is the default-off control for
// --lexical-gate: WITHOUT the flag, output is byte-identical to a bare query (the
// gate is off by default, mirroring the --half-life control).
func TestPlugin_LexicalGateAcceptedAndOffUnchanged(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	base, err := runPlugin(t, "--semantic", "structure acidity", dir)
	if err != nil {
		t.Fatal(err)
	}
	// Flag present but not set (the cobra default false) must match bare output.
	off, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate=false", dir)
	if err != nil {
		t.Fatalf("--lexical-gate=false: %v", err)
	}
	if base != off {
		t.Errorf("--lexical-gate=false changed output:\nbase=%q\noff=%q", base, off)
	}
	// A set gate must at least run without error and return results.
	on, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", dir)
	if err != nil {
		t.Fatalf("--lexical-gate: %v", err)
	}
	if strings.TrimSpace(on) == "" {
		t.Errorf("--lexical-gate returned no results")
	}
}

// TestPlugin_LexicalGateEmptyTermDegrades is the CLI empty-term degrade: an
// all-stopword query with the gate ON returns the same results as with it off —
// a no-op, the direct test of the reproduced 0-hit failure mode.
func TestPlugin_LexicalGateEmptyTermDegrades(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	q := "how should the"
	off, err := runPlugin(t, "--semantic", q, dir)
	if err != nil {
		t.Fatal(err)
	}
	on, err := runPlugin(t, "--semantic", q, "--lexical-gate", dir)
	if err != nil {
		t.Fatalf("--lexical-gate all-stopword: %v", err)
	}
	if off != on {
		t.Errorf("all-stopword gate must be a no-op:\noff=%q\non =%q", off, on)
	}
}

// TestPlugin_LexicalGateOverBroadDegrades is the CLI over-broad degrade: every
// node in the scoped bundle contains "structure", so a query on it matches 100%
// of the bundle — well over the fraction — and the gate must be a no-op.
func TestPlugin_LexicalGateOverBroadDegrades(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	q := "structure"
	off, err := runPlugin(t, "--semantic", q, dir)
	if err != nil {
		t.Fatal(err)
	}
	on, err := runPlugin(t, "--semantic", q, "--lexical-gate", dir)
	if err != nil {
		t.Fatalf("--lexical-gate over-broad: %v", err)
	}
	if off != on {
		t.Errorf("over-broad gate must be a no-op:\noff=%q\non =%q", off, on)
	}
}

// TestPlugin_LexicalGateInteractionSmoke is the interaction control the card
// mandates: --lexical-gate combined with --path and with --half-life must not
// panic and must return sane (non-error) output. The card is explicit that the
// interaction with scope filters (#63) and recency decay (#65) is not otherwise
// specified, so this asserts composition, not a particular ordering.
func TestPlugin_LexicalGateInteractionSmoke(t *testing.T) {
	dir := writeScopedBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	// --lexical-gate + --path
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--path", "wine/", dir); err != nil {
		t.Errorf("--lexical-gate + --path errored: %v", err)
	}
	// --lexical-gate + --half-life
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--half-life", "30", dir); err != nil {
		t.Errorf("--lexical-gate + --half-life errored: %v", err)
	}
	// --lexical-gate + the negating scope filters (#68). These flags did not
	// exist when the gate landed; composing the gate with exclusion is new
	// behavior and must at minimum not panic or error.
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--not-path", "coffee/", dir); err != nil {
		t.Errorf("--lexical-gate + --not-path errored: %v", err)
	}
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--not-type", "Playbook", dir); err != nil {
		t.Errorf("--lexical-gate + --not-type errored: %v", err)
	}
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--not-tag", "red", dir); err != nil {
		t.Errorf("--lexical-gate + --not-tag errored: %v", err)
	}
	// Inclusion and exclusion together, with the gate on.
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--path", "wine/", "--not-path", "wine/pairing.md", dir); err != nil {
		t.Errorf("--lexical-gate + --path + --not-path errored: %v", err)
	}
	// All the moving parts together.
	if _, err := runPlugin(t, "--semantic", "structure acidity", "--lexical-gate", "--path", "wine/", "--half-life", "30", dir); err != nil {
		t.Errorf("--lexical-gate + --path + --half-life errored: %v", err)
	}
}

// TestPlugin_LexicalGateNotPathConstrainsTail is the exclusion analog of
// TestPlugin_LexicalGatePathConstrainsTail against the #68 negating filters: a
// lexical-only hit that is EXCLUDED by --not-path must not be appended through the
// gate. "acidity" matches a wine node and a coffee node (a minority, so the gate
// does not degrade); with --not-path coffee/ the coffee node must never appear
// even though it lexically matches — exclusion beats the preserved lexical tail.
func TestPlugin_LexicalGateNotPathConstrainsTail(t *testing.T) {
	dir := writeGateTailBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "acidity", "--lexical-gate", "--not-path", "coffee/", dir)
	if err != nil {
		t.Fatalf("--lexical-gate + --not-path: %v", err)
	}
	if strings.Contains(out, "coffee/") {
		t.Errorf("--not-path coffee/ leaked a coffee lexical hit through the gate: %q", out)
	}
	if !strings.Contains(out, "wine/") {
		t.Errorf("--not-path coffee/ dropped the wine lexical hits too: %q", out)
	}
}

// TestPlugin_LexicalGatePathConstrainsTail pins the documented filter/gate
// interaction: a lexical-only hit that fails the --path filter is NOT appended.
// The bundle has enough nodes that "acidity" matches a MINORITY (so the gate does
// not degrade as over-broad), and both a wine and a coffee node match it; with
// --path wine/ the coffee node must never appear even though it lexically matches.
func TestPlugin_LexicalGatePathConstrainsTail(t *testing.T) {
	dir := writeGateTailBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "--semantic", "acidity", "--lexical-gate", "--path", "wine/", dir)
	if err != nil {
		t.Fatalf("--lexical-gate + --path: %v", err)
	}
	if strings.Contains(out, "coffee/") {
		t.Errorf("--path wine/ leaked a coffee lexical hit through the gate: %q", out)
	}
}

// TestSnippetPreview_TruncatesOnRuneBoundary pins that a snippet longer than the
// preview cap is truncated without splitting a multi-byte UTF-8 rune. The KB
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
