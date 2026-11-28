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

// footnote-definition (§4.2, §5.1): a footnote label is a sources[].id and
// its definition holds the citation text. A label defined twice (GFM matches
// labels case-insensitively) leaves the reference pointing at one of them, and
// a definition with no text cites nothing.

func footnoteDoc(body string) string {
	return "---\ntype: Concept\ntitle: A\nsources:\n  - id: \"1\"\n    resource: https://one.example/\n---\n\n# A\n\n" + body + "\n"
}

func footnoteFindings(t *testing.T, body string) []LintFinding {
	t.Helper()
	b := mkLintBundle(t, map[string]string{"index.md": "# Index\n\n- [A](a.md)\n", "a.md": footnoteDoc(body)})
	return findingsFor(Lint(b, LintOptions{}), "footnote-definition")
}

func TestLint_FootnoteDefinition_FlagsDuplicateAndEmpty_Section4_2(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		// The migrate output from a line-initial `[1]:` (KB ecosystem-watch [24]).
		"migrate repro": {
			"Count the commits on the default branch\n[^1]:\n\n```\ngit log\n```\n\n[^1]: VERIFIED — local measurement, 2026-11-22.",
			"2 times",
		},
		"duplicate with text":   {"A claim.[^1]\n\n[^1]: one\n[^1]: two", "2 times"},
		"duplicate case-folded": {"A claim.[^s1]\n\n[^s1]: one\n[^S1]: two", "2 times"},
		"empty at end":          {"A claim.[^1]\n\n[^1]:", "no text"},
		"empty before para":     {"A claim.[^1]\n\n[^1]:   \n\nNext paragraph.", "no text"},
	}
	for name, c := range cases {
		fs := footnoteFindings(t, c.body)
		if len(fs) == 0 {
			t.Errorf("%s: want a footnote-definition finding, got none", name)
			continue
		}
		joined := ""
		for _, f := range fs {
			if f.Path != "a.md" {
				t.Errorf("%s: finding on %q, want a.md", name, f.Path)
			}
			joined += f.Message + "\n"
		}
		if !strings.Contains(joined, c.want) || !strings.Contains(joined, "§4.2") {
			t.Errorf("%s: message should say %q and cite §4.2:\n%s", name, c.want, joined)
		}
	}
}

// The duplicate in the migrate repro is one finding for the label, plus one
// for the empty definition: two defects, each named once.
func TestLint_FootnoteDefinition_ReproNamesBothDefects_Section4_2(t *testing.T) {
	fs := footnoteFindings(t, "Count the commits on the default branch\n[^1]:\n\n```\ngit log\n```\n\n[^1]: VERIFIED — local measurement, 2026-11-22.")
	if len(fs) != 2 {
		t.Fatalf("want 2 findings (duplicate, empty), got %+v", fs)
	}
}

// Negative controls: one definition per label, a definition whose text sits on
// an indented continuation line, and look-alikes inside code are silent.
func TestLint_FootnoteDefinition_WellFormedSilent_Section4_2(t *testing.T) {
	for name, body := range map[string]string{
		"one each":          "Claims.[^1][^2]\n\n[^1]: one\n[^2]: two",
		"continuation":      "A claim.[^1]\n\n[^1]:\n    The text sits on the next line.",
		"continuation tab":  "A claim.[^1]\n\n[^1]:\n\tThe text sits on the next line.",
		"fenced look-alike": "A claim.[^1]\n\n```\n[^1]:\n[^1]: x\n```\n\n[^1]: one",
		"tilde fence":       "A claim.[^1]\n\n~~~\n[^1]:\n~~~\n\n[^1]: one",
		"reference only":    "A claim.[^1] and again.[^1]\n\n[^1]: one",
	} {
		if fs := footnoteFindings(t, body); len(fs) != 0 {
			t.Errorf("%s: want no finding, got %+v", name, fs)
		}
	}
}
