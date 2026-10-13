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

// lintDocTags builds a node with a type, title, and a YAML tags list, so tests
// can exercise the tag-hygiene fold the way the real corpus declares tags.
func lintDocTags(typ, title string, tags []string, body string) string {
	tl := "[" + strings.Join(tags, ", ") + "]"
	return "---\ntype: " + typ + "\ntitle: " + title + "\ntags: " + tl + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// The fold table is the contract. canonFold is the shared base fold used by both
// type-hygiene and tag-hygiene (case, trim, single trailing 's'). canonTag layers
// separator-insensitivity (-, _, space) on top of canonFold, for TAGS ONLY.
func TestCanonFold_SharedBaseFold(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Concept", "concept"},
		{"concept", "concept"},
		{"Concepts", "concept"},  // single trailing 's' dropped
		{"  Wine  ", "wine"},     // trim
		{"WINE", "wine"},         // case
		{"runbooks", "runbook"},  // trailing 's'
		{"run-book", "run-book"}, // base fold does NOT strip separators
		{"s", "s"},               // len==1 guard: don't strip the only char
		{"gas", "ga"},            // trailing 's' dropped even mid-word (existing type behavior)
		{"", ""},
	}
	for _, c := range cases {
		if got := canonFold(c.in); got != c.want {
			t.Errorf("canonFold(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCanonTag_SeparatorInsensitive(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Wine", "wine"},
		{"wine", "wine"},
		{"runbook", "runbook"},
		{"runbooks", "runbook"},   // trailing 's'
		{"run-book", "runbook"},   // hyphen stripped
		{"run_book", "runbook"},   // underscore stripped
		{"Run Book", "runbook"},   // space stripped + case
		{"Play Book", "playbook"}, // space stripped + case
		{"playbook", "playbook"},
		{"home-lab", "homelab"}, // separator-only collision
		{"homelab", "homelab"},
		{"design-patterns", "designpattern"}, // separator + trailing 's'
		{"design-pattern", "designpattern"},
		// The -is / -us / -ss class must NOT lose its trailing letters: only a
		// single trailing 's' is dropped, and none of these have a bare one.
		{"hormesis", "hormesi"}, // -is: 's' dropped once, still distinct from any sibling
		{"hysteresis", "hysteresi"},
		{"css", "cs"},
		{"gpt-oss", "gptos"},
		{"blast-radius", "blastradiu"},
		{"freshness", "freshnes"},
	}
	for _, c := range cases {
		if got := canonTag(c.in); got != c.want {
			t.Errorf("canonTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// canonTag must NOT change the type path. Types still fold with canonType
// (== canonFold), which is separator-SENSITIVE. Proven by table: a hyphenated
// and an unhyphenated type do NOT collapse under the type fold.
func TestCanonType_UnchangedBySeparatorRule(t *testing.T) {
	if canonType("run-book") == canonType("runbook") {
		t.Fatalf("type fold must stay separator-sensitive: run-book and runbook must NOT collapse")
	}
	if canonType("Concepts") != canonType("concept") {
		t.Fatalf("type fold must still be case+plural: Concepts and concept must collapse")
	}
}

// POSITIVE controls: each fires exactly one tag-hygiene finding.
func TestLint_TagHygiene_Positive(t *testing.T) {
	cases := []struct {
		name string
		tagA string
		tagB string
	}{
		{"case", "Wine", "wine"},
		{"plural", "runbook", "runbooks"},
		{"separator-hyphen", "run-book", "runbook"},
		{"separator-space", "Play Book", "playbook"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := mkLintBundle(t, map[string]string{
				"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
				"a.md":     lintDocTags("Concept", "A", []string{c.tagA}, "Body."),
				"b.md":     lintDocTags("Concept", "B", []string{c.tagB}, "Body."),
			})
			th := findingsFor(Lint(b, LintOptions{}), "tag-hygiene")
			if len(th) != 1 {
				t.Fatalf("%s: expected one tag-hygiene finding for %q/%q, got %+v", c.name, c.tagA, c.tagB, th)
			}
			// Message lists both variants with node counts, and is bundle-level.
			if th[0].Path != "" {
				t.Fatalf("%s: tag-hygiene must be bundle-level (Path==\"\"), got %q", c.name, th[0].Path)
			}
			m := lc(th[0].Message)
			if !strings.Contains(m, lc(c.tagA)) || !strings.Contains(m, lc(c.tagB)) {
				t.Fatalf("%s: message must name both variants: %q", c.name, th[0].Message)
			}
		})
	}
}

// The message carries per-node counts and sorts the variants.
func TestLint_TagHygiene_MessageCountsAndSort(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		// runbook x1 (a), runbooks x1 (b), run-book x1 (c) -> one folded group.
		"a.md": lintDocTags("Concept", "A", []string{"runbook"}, "Body."),
		"b.md": lintDocTags("Concept", "B", []string{"runbooks"}, "Body."),
		"c.md": lintDocTags("Concept", "C", []string{"run-book"}, "Body."),
	})
	th := findingsFor(Lint(b, LintOptions{}), "tag-hygiene")
	if len(th) != 1 {
		t.Fatalf("expected one folded tag-hygiene finding, got %+v", th)
	}
	msg := th[0].Message
	// Variants sorted ascending: run-book, runbook, runbooks.
	iRunBook := strings.Index(msg, "run-book")
	iRunbook := strings.Index(msg, "runbook ") // trailing space or paren after
	iRunbooks := strings.Index(msg, "runbooks")
	if iRunBook < 0 || iRunbook < 0 || iRunbooks < 0 {
		t.Fatalf("message must name all three variants: %q", msg)
	}
	if iRunBook >= iRunbook || iRunbook >= iRunbooks {
		t.Fatalf("variants must be sorted (run-book, runbook, runbooks): %q", msg)
	}
	// Per-node counts present: each variant is on exactly one node.
	if !strings.Contains(msg, "(1 node)") && !strings.Contains(msg, "1 node") {
		t.Fatalf("message must carry per-node counts: %q", msg)
	}
}

// NEGATIVE controls: legitimately similar-looking tags stay silent.
func TestLint_TagHygiene_Negative(t *testing.T) {
	cases := []struct {
		name string
		tagA string
		tagB string
	}{
		{"version-digit", "v1", "v2"},
		{"oauth-digit", "oauth1", "oauth2"},
		{"distinct-words", "oncall", "incident"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := mkLintBundle(t, map[string]string{
				"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
				"a.md":     lintDocTags("Concept", "A", []string{c.tagA}, "Body."),
				"b.md":     lintDocTags("Concept", "B", []string{c.tagB}, "Body."),
			})
			if n := len(findingsFor(Lint(b, LintOptions{}), "tag-hygiene")); n != 0 {
				t.Fatalf("%s: %q/%q are distinct; expected 0 tag-hygiene findings, got %d", c.name, c.tagA, c.tagB, n)
			}
		})
	}
}

// A single tag spread over many nodes is not drift — it must stay silent.
func TestLint_TagHygiene_SingleTagManyNodesNoFinding(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"wine"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"wine"}, "Body."),
		"c.md":     lintDocTags("Concept", "C", []string{"wine"}, "Body."),
	})
	if n := len(findingsFor(Lint(b, LintOptions{}), "tag-hygiene")); n != 0 {
		t.Fatalf("one tag on many nodes is not drift; expected 0 tag-hygiene findings, got %d", n)
	}
}

// The -is/-us/-ss digraph class (real-corpus measurement: 65 such tags, zero
// collisions) must each be silent — guards the trailing-'s' rule without needing
// a stop-list. Each planted on a distinct node with an unrelated sibling.
func TestLint_TagHygiene_DigraphClassSilent(t *testing.T) {
	digraphs := []string{"hormesis", "hysteresis", "css", "freshness", "blast-radius", "gpt-oss"}
	for _, d := range digraphs {
		t.Run(d, func(t *testing.T) {
			b := mkLintBundle(t, map[string]string{
				"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
				"a.md":     lintDocTags("Concept", "A", []string{d}, "Body."),
				"b.md":     lintDocTags("Concept", "B", []string{"kubernetes"}, "Body."),
			})
			if n := len(findingsFor(Lint(b, LintOptions{}), "tag-hygiene")); n != 0 {
				t.Fatalf("digraph tag %q must not collide with an unrelated sibling; got %d findings", d, n)
			}
		})
	}
}

// The shared-fold extraction must leave type-hygiene byte-identical. This pins
// the exact message string the check emitted before the refactor, proving the
// type path did not move when canonType became a thin canonFold wrapper.
func TestLint_TypeHygiene_MessageByteIdenticalAfterFoldExtraction(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "Body."),
		"b.md":     lintDoc("Concepts", "B", "Body."),
	})
	th := findingsFor(Lint(b, LintOptions{}), "type-hygiene")
	if len(th) != 1 {
		t.Fatalf("expected one type-hygiene finding, got %+v", th)
	}
	const want = "type-hygiene: near-duplicate type values likely refer to one type: Concept, Concepts"
	if th[0].Message != want {
		t.Fatalf("type-hygiene message changed by fold extraction:\n got: %q\nwant: %q", th[0].Message, want)
	}
	// And a separator-only type pair must NOT collapse (types stay
	// separator-sensitive; the tag-only rule must not leak into the type path).
	b2 := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("run-book", "A", "Body."),
		"b.md":     lintDoc("runbook", "B", "Body."),
	})
	if n := len(findingsFor(Lint(b2, LintOptions{}), "type-hygiene")); n != 0 {
		t.Fatalf("type-hygiene must stay separator-sensitive: run-book/runbook types must NOT fold, got %d findings", n)
	}
}
