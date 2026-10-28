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
	"fmt"
	"strings"
	"testing"
)

// lintDocTags builds a node with a type, title, and a YAML tags list, so tests
// can exercise the tag-hygiene fold the way the real corpus declares tags.
func lintDocTags(typ, title string, tags []string, body string) string {
	tl := "[" + strings.Join(tags, ", ") + "]"
	return "---\ntype: " + typ + "\ntitle: " + title + "\ntags: " + tl + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// The base fold is the contract. canonFold is the shared base fold used by
// type-hygiene (case, trim, single trailing 's'); it is UNCHANGED by the tag
// rework, which lives entirely on the tag path.
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

// canonTag is the tag BASE KEY: lower, trim, separator-insensitive. It does NOT
// strip plurals — that is a vocabulary-aware, bundle-scoped decision (see
// TestTagFolder_PluralIsVocabularyAware). Separators are '-', '_', space, and a
// letter-flanked '/' or '.' (UAX #29). A leading '.' is NOT a separator.
func TestCanonTag_BaseKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Wine", "wine"},
		{"wine", "wine"},
		{"runbook", "runbook"},
		{"run-book", "runbook"},   // hyphen stripped
		{"run_book", "runbook"},   // underscore stripped
		{"Run Book", "runbook"},   // space stripped + case
		{"Play Book", "playbook"}, // space stripped + case
		{"home-lab", "homelab"},   // separator-only collision
		{"homelab", "homelab"},
		{"node.js", "nodejs"}, // letter-flanked '.' is a separator
		{"nodejs", "nodejs"},
		{"ci/cd", "cicd"}, // letter-flanked '/' is a separator
		{"cicd", "cicd"},
		{".net", ".net"}, // leading '.' is NOT letter-flanked: stays apart from net
		{"net", "net"},
		// Base key leaves plurals alone (no bundle context here).
		{"runbooks", "runbooks"},
		{"design-patterns", "designpatterns"},
		{"hormesis", "hormesis"}, // the -is/-us/-ss class is untouched
		{"css", "css"},
		{"gpt-oss", "gptoss"},
	}
	for _, c := range cases {
		if got := canonTag(c.in); got != c.want {
			t.Errorf("canonTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The plural merge is an adaptation of KStem's plural() (Lucene KStemmer.java):
// a plural folds onto its stem ONLY when that stem is itself attested in the
// bundle. The attested set is supplied, newTagFolder is built from it, and the
// fold is exercised through it.
func TestTagFolder_PluralIsVocabularyAware(t *testing.T) {
	attested := []string{
		"patch", "patches", // -es, drop "es" -> patch
		"policy", "policies", // -ies -> policy
		"process", "processes", // -es after double-s -> process (not processe)
		"anti-pattern", "anti-patterns", // plain 's' + separator
		"runbook", "runbooks", // plain 's'
		"category", "categories", // -ies -> category
		// Negatives that must NOT fold even though they look plural-ish:
		"new", "news", // Porter2 invariant: news never strips -> stays apart from new
		"op", "ops", // len <= 3: ops never strips -> stays apart from op
	}
	files := map[string]string{}
	idx := "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n"
	for i, tag := range attested {
		name := fmt.Sprintf("n%02d.md", i)
		files[name] = lintDocTags("Concept", name, []string{tag}, "Body.")
		idx += fmt.Sprintf("- [%s](%s)\n", name, name)
	}
	files["index.md"] = idx
	b := mkLintBundle(t, files)
	fold := newTagFolder(b)

	same := func(a, c string) {
		t.Helper()
		if fold(a) != fold(c) {
			t.Errorf("expected %q and %q to fold together, got %q vs %q", a, c, fold(a), fold(c))
		}
	}
	diff := func(a, c string) {
		t.Helper()
		if fold(a) == fold(c) {
			t.Errorf("expected %q and %q to stay distinct, both folded to %q", a, c, fold(a))
		}
	}

	// Plurals fold onto an attested stem.
	same("patch", "patches")
	same("policy", "policies")
	same("process", "processes")
	same("runbook", "runbooks")
	same("anti-pattern", "anti-patterns")
	same("category", "categories")

	// Distinct tags do NOT merge.
	diff("new", "news") // Porter2 invariant
	diff("op", "ops")   // length guard
}

// kstemPlural is the pure stem generator (vocabulary decision is separate).
func TestKstemPlural_Guards(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"runbooks", []string{"runbook"}},        // plain 's'
		{"patches", []string{"patche", "patch"}}, // -es, not double-s: try "patche" then "patch"
		{"policies", []string{"policie", "policy"}},
		{"processes", []string{"process"}}, // double-s -> skip the "processe" candidate
		{"aws", nil},                       // len 3: not > 3
		{"ops", nil},                       // len 3
		{"ios", nil},                       // len 3
		{"dns", nil},                       // len 3
		{"class", nil},                     // double-s (penult 's')
		{"famous", nil},                    // -ous
		{"news", nil},                      // Porter2 invariant
		{"k8s", nil},                       // not letters-only
		{"runbook", nil},                   // no trailing 's'
		{"", nil},
	}
	for _, c := range cases {
		got := kstemPlural(c.in)
		if len(got) != len(c.want) {
			t.Errorf("kstemPlural(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		// "patches" generates ["patche","patch"]; only assert "patch" is present
		// for the simple cases to keep the table readable.
		for _, w := range c.want {
			found := false
			for _, g := range got {
				if g == w {
					found = true
				}
			}
			if !found {
				t.Errorf("kstemPlural(%q) = %v, want it to contain %q", c.in, got, w)
			}
		}
	}
}

// canonTag must NOT change the type path. Types still fold with canonType
// (== canonFold), which is separator- AND plural-insensitive only in the base
// sense. A hyphenated and an unhyphenated type do NOT collapse under the type
// fold, and the tag's plural vocabulary rule never touches types.
func TestCanonType_UnchangedByTagRules(t *testing.T) {
	if canonType("run-book") == canonType("runbook") {
		t.Fatalf("type fold must stay separator-sensitive: run-book and runbook must NOT collapse")
	}
	if canonType("Concepts") != canonType("concept") {
		t.Fatalf("type fold must still be case+plural: Concepts and concept must collapse")
	}
}

// POSITIVE controls: each fires exactly one tag-hygiene finding. Covers case,
// plain plural, -es/-ies plurals, hyphen/space separators, and letter-flanked
// '/' and '.' separators.
func TestLint_TagHygiene_Positive(t *testing.T) {
	cases := []struct {
		name string
		tagA string
		tagB string
	}{
		{"case", "Wine", "wine"},
		{"plural", "runbook", "runbooks"},
		{"plural-es", "patch", "patches"},
		{"plural-ies", "policy", "policies"},
		{"plural-double-s", "process", "processes"},
		{"separator-hyphen", "run-book", "runbook"},
		{"separator-space", "Play Book", "playbook"},
		{"separator-slash", "ci/cd", "cicd"},
		{"separator-dot", "node.js", "nodejs"},
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

// The message pins the FULL string: variants ordered most-common-first (the
// dominant spelling to normalize toward leads), ties broken alphabetically, with
// per-node counts. runbook x2 (a, c) dominates runbooks x1 (b) and run-book x1
// (d); the two one-node variants tie and sort alphabetically (run-book < runbook).
func TestLint_TagHygiene_MessageFullStringMostCommonFirst(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [D](d.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"runbook"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"runbooks"}, "Body."),
		"c.md":     lintDocTags("Concept", "C", []string{"runbook"}, "Body."),
		"d.md":     lintDocTags("Concept", "D", []string{"run-book"}, "Body."),
	})
	th := findingsFor(Lint(b, LintOptions{}), "tag-hygiene")
	if len(th) != 1 {
		t.Fatalf("expected one folded tag-hygiene finding, got %+v", th)
	}
	const want = "tag-hygiene: near-duplicate tag values likely refer to one tag: runbook (2 nodes), run-book (1 node), runbooks (1 node)"
	if th[0].Message != want {
		t.Fatalf("message mismatch:\n got: %q\nwant: %q", th[0].Message, want)
	}
}

// NEGATIVE controls: legitimately similar-looking tags stay silent. The
// new/news and op/ops pairs are LOAD-BEARING — they collide under a bare
// trailing-'s' rule and only stay silent because of the Porter2 invariant and
// the length guard. See TestLint_TagHygiene_NegativeControlsAreLoadBearing.
func TestLint_TagHygiene_Negative(t *testing.T) {
	cases := []struct {
		name string
		tagA string
		tagB string
	}{
		{"version-digit", "v1", "v2"},
		{"oauth-digit", "oauth1", "oauth2"},
		{"distinct-words", "oncall", "incident"},
		{"porter2-invariant", "new", "news"}, // news never strips -> distinct from new
		{"length-guard", "op", "ops"},        // ops len 3 -> never strips
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

// Proof the new/news and op/ops negative controls are LOAD-BEARING: under the
// OLD bare trailing-'s' fold (lower+trim+drop one 's'), both pairs collide, so a
// test that passes them only means something because the new fold deliberately
// keeps them apart. This reconstructs the old fold inline and asserts it WOULD
// have merged them — so a regression back to the bare rule turns the negative
// control red.
func TestLint_TagHygiene_NegativeControlsAreLoadBearing(t *testing.T) {
	oldBareFold := func(s string) string {
		c := strings.ToLower(strings.TrimSpace(s))
		if len(c) > 1 && strings.HasSuffix(c, "s") {
			c = c[:len(c)-1]
		}
		return c
	}
	for _, p := range [][2]string{{"new", "news"}, {"op", "ops"}} {
		if oldBareFold(p[0]) != oldBareFold(p[1]) {
			t.Fatalf("negative control %q/%q is NOT load-bearing: the old bare fold did not merge them (%q vs %q)",
				p[0], p[1], oldBareFold(p[0]), oldBareFold(p[1]))
		}
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
// collisions) must each be silent. Two mechanisms keep them silent: 'ss' and
// 'ous' are never stripped at all (css, gpt-oss), and an '-is'/'-us' word only
// folds if its stripped stem is ATTESTED in the bundle — which it is not here,
// nor in the real corpus. Each is paired with an unrelated sibling so the only
// way a finding could fire is an over-eager fold; the vocabulary guard prevents
// it without a stop-list.
func TestLint_TagHygiene_DigraphClassSilent(t *testing.T) {
	digraphs := []string{"hormesis", "hysteresis", "css", "freshness", "blast-radius", "gpt-oss"}
	for _, d := range digraphs {
		t.Run(d, func(t *testing.T) {
			b := mkLintBundle(t, map[string]string{
				"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
				"a.md":     lintDocTags("Concept", "A", []string{d}, "Body."),
				"b.md":     lintDocTags("Concept", "B", []string{"incident"}, "Body."),
			})
			if n := len(findingsFor(Lint(b, LintOptions{}), "tag-hygiene")); n != 0 {
				t.Fatalf("digraph tag %q must not collide with an unrelated sibling; got %d findings", d, n)
			}
		})
	}
}

// The vocabulary guard is what keeps the '-is'/'-us' class quiet: hormesis WOULD
// strip to hormesi under the plain-'s' rule, but only folds if hormesi is an
// attested tag. This proves the stem generator does produce the candidate (so
// the guard is doing real work) AND that a bundle without the stem never folds.
func TestLint_TagHygiene_IsUsClassGuardedByVocabulary(t *testing.T) {
	// Generator DOES offer the stripped candidate (plain 's', len>3, not ss/ous).
	if got := kstemPlural("hormesis"); len(got) != 1 || got[0] != "hormesi" {
		t.Fatalf("kstemPlural(hormesis) = %v, want [hormesi]", got)
	}
	// But with no attested "hormesi" in the bundle, the two stay distinct.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"hormesis"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"incident"}, "Body."),
	})
	if n := len(findingsFor(Lint(b, LintOptions{}), "tag-hygiene")); n != 0 {
		t.Fatalf("hormesis must not fold without an attested hormesi stem; got %d findings", n)
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
	// separator-sensitive; the tag-only rules must not leak into the type path).
	b2 := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("run-book", "A", "Body."),
		"b.md":     lintDoc("runbook", "B", "Body."),
	})
	if n := len(findingsFor(Lint(b2, LintOptions{}), "type-hygiene")); n != 0 {
		t.Fatalf("type-hygiene must stay separator-sensitive: run-book/runbook types must NOT fold, got %d findings", n)
	}
}
