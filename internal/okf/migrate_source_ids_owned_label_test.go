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

import "testing"

// v0.3 §4.2: a footnote label is a `sources[].id`, matched case-insensitively.
// A label that equals an id a `sources` entry already carries is owned by that
// source, even when no `# Citations` entry attaches to it (a hand-written
// descriptor source, §5.1). It is not a collision. A label that only equals an
// id this migration would assign is still one.

// Positive control, from a real node: sources 1–4 are URLs the remaining
// `# Citations` entries [1]–[4] hold; source 5 is a plain-words descriptor
// with footnote [^5] and no list entry. Re-planning must not refuse the node.
func TestConformance_MigrateSourceIDs_Section4_2_LabelOwnedByExistingSourceIsNotCollision(t *testing.T) {
	node := "---\ntype: Research Brief\nsources:\n" +
		"  - resource: \"https://one.example/\"\n    id: \"1\"\n" +
		"  - resource: \"https://two.example/\"\n    id: \"2\"\n" +
		"  - resource: \"https://three.example/\"\n    id: \"3\"\n" +
		"  - resource: \"https://four.example/\"\n    id: \"4\"\n" +
		"  - id: \"5\"\n    resource: \"the private selection skill on the authoring host (not published)\"\n" +
		"---\n\n# Executive answer\n\nRules [^1][^2]. History [^3]. Ratings [^4]. Criteria [^5].\n\n" +
		"[^1]: VERIFIED — rules <https://one.example/>\n" +
		"[^2]: VERIFIED — body <https://two.example/>\n" +
		"[^3]: VERIFIED — history <https://three.example/>\n" +
		"[^4]: VERIFIED — ratings <https://four.example/>\n\n" +
		"[^5]: VERIFIED (internal) — the selection skill in this system.\n\n" +
		"# Citations\n\n" +
		"[1] VERIFIED — rules <https://one.example/>\n\n" +
		"[2] VERIFIED — body <https://two.example/>\n\n" +
		"[3] VERIFIED — history <https://three.example/>\n\n" +
		"[4] VERIFIED — ratings <https://four.example/>\n"
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	if hasJudgment(plan, "a.md", JudgmentSourceIDCollision) {
		t.Fatalf("§4.2: [^5] is owned by the source with id 5, not a collision; got %+v", plan.Judgment)
	}
}

// Negative control: the body already uses [^7], no source carries id 7, and
// the list entry [7] would be assigned id 7. That label is foreign, so the
// node is still refused.
func TestConformance_MigrateSourceIDs_Section4_2_LabelWithoutSourceIDStillCollides(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - id: \"5\"\n    resource: \"a descriptor with no link\"\n" +
		"---\n\n# Def\n\nA claim [7]. Another [^7]. Criteria [^5].\n\n" +
		"[^7]: an unrelated note.\n\n" +
		"[^5]: the descriptor's entry text.\n\n" +
		"# Citations\n\n" +
		"[7] VERIFIED — thing https://z.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	j := findJudgment(t, plan, "a.md", JudgmentSourceIDCollision)
	if j.Context != "footnote label [^7] is already in the body" {
		t.Fatalf("§4.2: [^7] matches no sources id and the new id 7; want the footnote-label collision, got %+v", j)
	}
}
