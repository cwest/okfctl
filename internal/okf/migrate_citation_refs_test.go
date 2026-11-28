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

// v0.3 §4.2: per-claim attribution is a footnote whose label is a
// `sources[].id` (§5.1). A body `[k]` is only held back as a judgment item
// when the words around it may point at another document's list. Words in a
// different block (another paragraph, a heading, another list item or table
// row) do not qualify a reference, and neither does a word that only contains
// "node" or "brief" ("inode", "briefing").

func refNode(body string) map[string]string {
	return kb("---\ntype: Concept\n---\n\n# Def\n\n" + body + "\n\n# Citations\n\n" +
		"[1] VERIFIED — one https://one.example/\n" +
		"[2] VERIFIED — two https://two.example/\n")
}

// planRefs plans and applies a node and returns its migrated text and the
// citation-ref-unclear contexts the plan reported.
func planRefs(t *testing.T, body string) (string, []string) {
	t.Helper()
	dir := mkMigrateBundle(t, refNode(body))
	b, plan := planFor(t, dir, "", false)
	var unclear []string
	for _, j := range plan.Judgment {
		if j.Path == "a.md" && j.Kind == JudgmentCitationRef {
			unclear = append(unclear, j.Context)
		}
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return readRel(t, dir, "a.md"), unclear
}

// Positive controls: a qualifying word in another block does not hold back a
// reference.
func TestConformance_MigrateCitationRefs_Section4_2_OtherBlockDoesNotQualify(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"heading after": {
			"The answer to the gap. [1]\n\n# The node map\n\nText.",
			"The answer to the gap. [^1]\n\n# The node map",
		},
		"paragraph after": {
			"A plain claim. [1]\n\nThis node is the neutral landscape.",
			"A plain claim. [^1]\n\nThis node",
		},
		"paragraph before": {
			"See the brief at [x](/b.md).\n\nA plain claim. [1]",
			"A plain claim. [^1]",
		},
		"next list item": {
			"- A plain claim [1]\n- The parent node docs",
			"- A plain claim [^1]\n- The parent node docs",
		},
		"next table row": {
			"| a | claim [1] |\n|---|---|\n| [b](/b.md) | y |",
			"| a | claim [^1] |",
		},
		"adjacent run before a heading": {
			"A pair claim. [1][2]\n\n## Why this node matters",
			"A pair claim. [^1][^2]",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out, unclear := planRefs(t, c.body)
			if len(unclear) != 0 {
				t.Fatalf("§4.2: a word in another block must not hold the reference back; unclear = %q", unclear)
			}
			if !strings.Contains(out, c.want) {
				t.Fatalf("§4.2: reference not rewritten\n--- got\n%s\n--- want substring\n%s", out, c.want)
			}
		})
	}
}

// Positive control: a word that merely contains a trigger is not the trigger.
func TestConformance_MigrateCitationRefs_Section4_2_WholeWordsOnly(t *testing.T) {
	for _, body := range []string{
		"It recreates the socket with a new inode. [1]",
		"Graph embeddings such as node2vec score it [1].",
		"Agents ask through elicitation [1].",
		"The persona is the durable half of that briefing [1].",
	} {
		out, unclear := planRefs(t, body)
		if len(unclear) != 0 || !strings.Contains(out, strings.Replace(body, "[1]", "[^1]", 1)) {
			t.Errorf("§4.2: %q must be rewritten, not reported; unclear = %q\n%s", body, unclear, out)
		}
	}
}

// Negative controls: a trigger in the same block, including across a soft line
// break, still holds the reference back, plurals included.
func TestConformance_MigrateCitationRefs_Section4_2_SameBlockStillJudgment(t *testing.T) {
	for _, body := range []string{
		"The parent node describes it [1].",
		"As the sibling briefs say [1], it holds.",
		"As the write-time brief says in its\ncitations [1], it holds.",
		"- The routing node counted it [1]",
		"| claim [1] | [b](/b.md) |",
	} {
		out, unclear := planRefs(t, body)
		if len(unclear) != 1 {
			t.Errorf("§4.2: %q must be one judgment item; got %q", body, unclear)
		}
		if !strings.Contains(out, body) {
			t.Errorf("§4.2: %q must stay as written:\n%s", body, out)
		}
	}
}

// A `[k]` inside a code span or a fenced code block is code, not a citation
// reference (§4.2): it is neither rewritten nor reported, and a trigger word
// inside code does not hold back a reference beside it.
func TestConformance_MigrateCitationRefs_Section4_2_CodeIsNotAReference(t *testing.T) {
	body := "Read `node.refs[1]` here. Run `see brief [2]` too. Then a claim [2].\n\n```\nnode[1] = brief[2]\n```"
	out, unclear := planRefs(t, body)
	if len(unclear) != 0 {
		t.Fatalf("§4.2: code is not a citation reference; unclear = %q", unclear)
	}
	for _, keep := range []string{"`node.refs[1]`", "`see brief [2]`", "node[1] = brief[2]", "Then a claim [^2]."} {
		if !strings.Contains(out, keep) {
			t.Errorf("expected %q in\n%s", keep, out)
		}
	}
}

// Negative controls for link reference syntax (CommonMark): `[text][ref]`
// with a defined `[ref]` is a link and stays; `[text][k]` with no definition
// may still be meant as one, so the `[k]` is reported; a run of key-shaped
// brackets `[1][2]` with no definitions is two citations.
func TestConformance_MigrateCitationRefs_Section4_2_LinkLabels(t *testing.T) {
	out, unclear := planRefs(t, "See [the doc][2] for it.\n\n[2]: https://doc.example/")
	if !strings.Contains(out, "[the doc][2]") || len(unclear) != 0 {
		t.Fatalf("a defined reference link stays a link; unclear = %q\n%s", unclear, out)
	}
	out, unclear = planRefs(t, "Recollection [2, internal ref][1] holds.")
	if len(unclear) != 1 || !strings.Contains(out, "[2, internal ref][1]") {
		t.Fatalf("the label of an undefined [text][k] stays and is reported; unclear = %q\n%s", unclear, out)
	}
	out, unclear = planRefs(t, "Counts hold [2][1].")
	if len(unclear) != 0 || !strings.Contains(out, "Counts hold [^2][^1].") {
		t.Fatalf("an undefined [k][k] run is two citations; unclear = %q\n%s", unclear, out)
	}
}
