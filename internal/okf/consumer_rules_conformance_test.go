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

package okf

import (
	"strings"
	"testing"
)

// Spec-conformance suite for the OKF v0.3 §11.2 consumer rules on values that
// are absent, empty, or have an unexpected shape:
//
//  1. A null value, an empty string, and an empty list are the same as an
//     absent key.
//  2. A list field (tags, sources, verified, parameters) accepts one item as a
//     one-item list; a scalar tags value is one tag, never split at commas.
//  3. A family entry without its REQUIRED key is ignored, and consumers SHOULD
//     warn about it.
//
// In every case the bundle stays conformant, so none of these may surface as a
// validate finding.

// concept renders a minimal concept document carrying extra frontmatter lines.
func concept(title, extra string) string {
	return "---\ntype: Concept\ntitle: " + title + "\n" + extra + "---\n\n# " + title + "\n\nBody.\n"
}

func consumerRulesBundle(t *testing.T, nodes map[string]string) *Bundle {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# KB\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	for p, src := range nodes {
		writeFile(t, dir, p, src)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("§11.2: the bundle stays conformant in every case; got validate findings: %v", f)
	}
	return b
}

// --- §11.2 rule 1: null / empty string / empty list == absent ---------------

func TestConformance_NullOrEmptyStatusMeansStable_Section11_2_Rule1(t *testing.T) {
	b := consumerRulesBundle(t, map[string]string{
		"null.md":       concept("Null", "status:\n"),
		"tilde.md":      concept("Tilde", "status: ~\n"),
		"empty-str.md":  concept("EmptyStr", "status: \"\"\n"),
		"empty-list.md": concept("EmptyList", "status: []\n"),
		// Positive control: a genuine out-of-enum value is still reported, so the
		// silence above is the null rule, not a broken detector.
		"grade.md": concept("Grade", "status: verified\n"),
	})
	for _, p := range []string{"null.md", "tilde.md", "empty-str.md", "empty-list.md"} {
		if got := b.Nodes[p].Status(); got != StatusStable {
			t.Errorf("%s: Status() = %q, want %q (§11.2 rule 1: null/empty is absent; §5.4 absent is stable)", p, got, StatusStable)
		}
	}
	fs := findingsFor(Lint(b, LintOptions{}), "status-lifecycle")
	if len(fs) != 1 || fs[0].Path != "grade.md" {
		t.Fatalf("want exactly one status-lifecycle finding (grade.md); got %+v", fs)
	}
	for _, f := range fs {
		if strings.Contains(f.Message, "<nil>") {
			t.Errorf("a null status must never render as <nil>: %s", f.Message)
		}
	}
}

func TestConformance_NullOrEmptyValueIsAbsentKey_Section11_2_Rule1(t *testing.T) {
	b := consumerRulesBundle(t, map[string]string{
		"a.md": concept("A", "epistemic:\nsources: []\ntags: \"\"\nverified: ~\n"+
			// Negative control: falsy values that are NOT null/empty stay present.
			"authority: false\nweight: 0\nnote: \" \"\n"),
	})
	fm := b.Nodes["a.md"].Frontmatter
	for _, k := range []string{"epistemic", "sources", "tags", "verified"} {
		if _, present := fm[k]; present {
			t.Errorf("key %q with a null/empty value must read as absent (§11.2 rule 1); got %#v", k, fm[k])
		}
	}
	for _, k := range []string{"authority", "weight", "note", "type", "title"} {
		if _, present := fm[k]; !present {
			t.Errorf("key %q is not null/empty and must stay present", k)
		}
	}
	if _, ok := b.Nodes["a.md"].Epistemic(); ok {
		t.Error("Epistemic() reported a null epistemic as present (§11.2 rule 1)")
	}
}

// TestConformance_ReservedFilesNotNormalized_Section11_2 is the negative control
// on scope: the §11.2 value rules are applied to concept frontmatter only.
// Reserved index.md frontmatter is governed by §8.1/§12 and keeps its raw map,
// so a root-index block validates exactly as it did before.
func TestConformance_ReservedFilesNotNormalized_Section11_2(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\nokf_version: \"\"\n---\n\n# KB\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "a.md", concept("A", ""))
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := b.Reserved["index.md"].Frontmatter["okf_version"]; !present {
		t.Fatal("reserved index.md frontmatter must not be rewritten by concept normalization")
	}
}

// --- §11.2 rule 2: one item is a one-item list --------------------------------

func TestConformance_SingleMappingSourcesIsOneItemList_Section11_2_Rule2(t *testing.T) {
	b := consumerRulesBundle(t, map[string]string{
		"single.md": concept("Single", "sources: { id: s, resource: \"https://example.com/x\" }\n"),
		"scalar.md": concept("Scalar", "sources: https://example.com/flat\n"),
		// Negative control: a node with no provenance at all is still uncited.
		"bare.md": concept("Bare", ""),
	})
	n := b.Nodes["single.md"]
	src := n.Sources()
	if len(src) != 1 || src[0].ID != "s" || src[0].Resource != "https://example.com/x" {
		t.Fatalf("single-mapping sources: Sources() = %+v, want one entry s→https://example.com/x (§11.2 rule 2)", src)
	}
	if got := n.SourceCitations(); got != 1 {
		t.Errorf("SourceCitations() = %d, want 1", got)
	}
	if raw, ok := b.Nodes["scalar.md"].Frontmatter["sources"].([]any); !ok || len(raw) != 1 {
		t.Errorf("a scalar sources value must read as a one-item list; got %#v", b.Nodes["scalar.md"].Frontmatter["sources"])
	}

	var uncited []string
	for _, f := range EvalTransparency(b, EvalOptions{}) {
		if f.Check == "uncited" {
			uncited = append(uncited, f.Path)
		}
	}
	// scalar.md is a v0.1-style flat string: a list item, but not a structured
	// §5.1 entry, so it is not counted as a citation. Only bare.md and scalar.md
	// may be uncited; single.md must not be.
	for _, p := range uncited {
		if p == "single.md" {
			t.Fatalf("single-mapping sources must not mark the node uncited (§11.2 rule 2); uncited=%v", uncited)
		}
	}
	found := false
	for _, p := range uncited {
		if p == "bare.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("negative control: bare.md (no provenance) must still be uncited; uncited=%v", uncited)
	}
}

func TestConformance_SingleItemListFields_Section11_2_Rule2(t *testing.T) {
	b := consumerRulesBundle(t, map[string]string{
		"a.md": concept("A", "tags: \"alpha, beta\"\nverified: { by: \"human:x\", at: 2026-06-25T09:00:00Z }\n"+
			"parameters: { name: year, type: integer }\n"),
	})
	n := b.Nodes["a.md"]
	tags := nodeTags(n)
	if len(tags) != 1 || tags[0] != "alpha, beta" {
		t.Errorf("scalar tags = %q, want one tag \"alpha, beta\" (§11.2 rule 2: never split at commas)", tags)
	}
	if v := n.Verified(); len(v) != 1 || v[0].By != "human:x" {
		t.Errorf("bare-mapping verified = %+v, want one event by human:x", v)
	}
	for _, k := range []string{"tags", "verified", "parameters"} {
		if l, ok := n.Frontmatter[k].([]any); !ok || len(l) != 1 {
			t.Errorf("%s: single item must read as a one-item list; got %#v", k, n.Frontmatter[k])
		}
	}
}

// --- §11.2 rule 3: ignore a resource-less source, and warn --------------------

func TestConformance_ResourcelessSourceWarns_Section11_2_Rule3(t *testing.T) {
	b := consumerRulesBundle(t, map[string]string{
		"missing.md": concept("Missing", "sources:\n  - { id: good, resource: \"https://example.com/a\" }\n"+
			"  - { id: nores, title: No resource }\n  - { id: blank, resource: \"\" }\n  - { id: nul, resource: ~ }\n"),
		"single.md": concept("Single", "sources: { id: lone, title: Lone }\n"),
		// Negative controls: well-formed entries, a scope descriptor, and a v0.1
		// flat-string list are all silent.
		"ok.md":    concept("OK", "sources:\n  - { resource: \"https://example.com/b\" }\n  - { resource: all queries in BigQuery project X }\n"),
		"flat.md":  concept("Flat", "sources:\n  - https://example.com/flat\n"),
		"empty.md": concept("Empty", "sources: []\n"),
	})

	// The entry is still IGNORED by the reader (§11.2 rule 3).
	if src := b.Nodes["missing.md"].Sources(); len(src) != 1 || src[0].ID != "good" {
		t.Fatalf("Sources() = %+v, want only the well-formed entry", src)
	}

	fs := findingsFor(Lint(b, LintOptions{}), "source-missing-resource")
	got := map[string]int{}
	for _, f := range fs {
		got[f.Path]++
		if !strings.Contains(f.Message, "§11.2") || !strings.Contains(f.Message, "resource") {
			t.Errorf("finding should name resource and cite §11.2: %s", f.Message)
		}
	}
	if got["missing.md"] != 3 {
		t.Errorf("missing.md: %d source-missing-resource findings, want 3 (nores, blank, nul); all=%+v", got["missing.md"], fs)
	}
	if got["single.md"] != 1 {
		t.Errorf("single.md: a single-mapping entry without resource must warn too; all=%+v", fs)
	}
	for _, p := range []string{"ok.md", "flat.md", "empty.md"} {
		if got[p] != 0 {
			t.Errorf("negative control %s must be silent; got %+v", p, fs)
		}
	}
	for _, f := range fs {
		if f.Path == "missing.md" && strings.Contains(f.Message, "\"good\"") {
			t.Errorf("the well-formed entry must not be reported: %s", f.Message)
		}
	}
}

// TestConformance_MigrateExtendsNullOrSingleSources_Section11_2_Rule2 covers the
// write side of the same shape rules: a null or single-mapping `sources` is an
// absent key or a one-item list (§11.2 rules 1 and 2), so migrating body
// citations into it must extend that value, not append a duplicate `sources`
// key that leaves the frontmatter unparseable.
func TestConformance_MigrateExtendsNullOrSingleSources_Section11_2_Rule2(t *testing.T) {
	cite := "\n# Def\n\n# Citations\n- https://example.com/new\n"
	dir := mkMigrateBundle(t, map[string]string{
		".okf":      "okf_version: 0.1\n",
		"index.md":  "# KB\n",
		"log.md":    "# Log\n",
		"null.md":   "---\ntype: Metric\nsources:\n---\n" + cite,
		"single.md": "---\ntype: Metric\nsources: { id: old, resource: \"https://example.com/old\" }\n---\n" + cite,
		// Negative control: an existing block list is extended exactly as before.
		"list.md": "---\ntype: Metric\nsources:\n  - resource: https://example.com/old\n---\n" + cite,
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "agent/v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatal(err)
	}
	b2 := loadMigrate(t, dir)
	if f := Validate(b2); len(f) != 0 {
		t.Fatalf("validate after migrate: %v", f)
	}
	want := map[string][]string{
		"null.md":   {"https://example.com/new"},
		"single.md": {"https://example.com/old", "https://example.com/new"},
		"list.md":   {"https://example.com/old", "https://example.com/new"},
	}
	for p, w := range want {
		var got []string
		for _, s := range b2.Nodes[p].Sources() {
			got = append(got, s.Resource)
		}
		if strings.Join(got, ",") != strings.Join(w, ",") {
			t.Errorf("%s: sources after migrate = %v, want %v", p, got, w)
		}
	}
	if s := b2.Nodes["single.md"].Sources(); len(s) == 0 || s[0].ID != "old" {
		t.Errorf("single.md: the original entry's other keys must survive; got %+v", s)
	}
}
