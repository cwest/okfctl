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

// §12: migration is idempotent. An entry whose full text is the footnote
// `[^k]` of a source with `id: k` was converted on an earlier run (by hand or
// through a resolution), whatever resource that source carries. A later plan
// that reads more of the entry, or splits an entry it used to leave alone,
// must not convert it a second time: the second source would take the same
// id and the node would be refused as a source-id-collision.

// Shape (a): the entry was read only up to a wrapped `#` line and resolved to
// a resource spelled differently from the URL that is now visible below that
// line. The held source keeps the entry; the visible URL adds nothing.
func TestConformance_MigrateHeldReplan_Section12_ResolvedEntryWithURLNowVisible(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - id: \"8\"\n    resource: https://example.org/moved/issues/1\n    title: Issue 1\n---\n\n" +
		"Claim [^8].\n\n" +
		"[^8]: VERIFIED — An issue, two comments (one on 2026-11-17 (x, y;\n\n" +
		"# Citations\n\n" +
		"[8] VERIFIED — An issue, two comments (one on 2026-11-17 (x, y;\n" +
		"#4765 shipped but no driver implements it)); more text.\n" +
		"https://example.org/old/issues/1\n"
	assertHeldReplanQuiet(t, node)
}

// Shape (b): a multi-resource entry resolved to its second resource while a
// link reference definition owned the first. The definition no longer owns
// it, so the entry would split; the held source keeps it instead.
func TestConformance_MigrateHeldReplan_Section12_ResolvedMultiURLEntryNotSplit(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - resource: https://example.org/docs/\n    title: \"[docs]: https://example.org/docs/\"\n" +
		"  - id: \"2\"\n    resource: https://example.org/llms-full.txt\n    title: Docs\n---\n\n" +
		"Claim [^2]. See [Docs][docs].\n\n" +
		"[^2]: VERIFIED — Docs <https://example.org/docs/> · <https://example.org/llms-full.txt>\n\n" +
		"# Citations\n\n" +
		"[docs]: https://example.org/docs/\n\n" +
		"[2] VERIFIED — Docs <https://example.org/docs/> ·\n" +
		"<https://example.org/llms-full.txt>\n"
	assertHeldReplanQuiet(t, node)
}

// Negative control: a footnote [^8] that holds other text, or only a prefix
// cut inside a word, does not mark the entry converted. The collision is
// still reported, never hidden.
func TestConformance_MigrateHeldReplan_Section12_UnrelatedFootnoteStillCollides(t *testing.T) {
	for name, def := range map[string]string{
		"other text":      "A different claim entirely.",
		"cut inside word": "VERIFIED — An iss",
	} {
		t.Run(name, func(t *testing.T) {
			node := "---\ntype: Concept\nsources:\n" +
				"  - id: \"8\"\n    resource: https://example.org/moved/issues/1\n    title: Issue 1\n---\n\n" +
				"Claim [^8].\n\n[^8]: " + def + "\n\n# Citations\n\n" +
				"[8] VERIFIED — An issue https://example.org/old/issues/1\n"
			_, plan := planFor(t, mkMigrateBundle(t, kb(node)), "", false)
			if !hasJudgment(plan, "a.md", JudgmentSourceIDCollision) {
				t.Errorf("§12: [^8] does not hold the entry, so the id must collide: %+v", plan.Judgment)
			}
		})
	}
}

func assertHeldReplanQuiet(t *testing.T, node string) {
	t.Helper()
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	for _, j := range plan.Judgment {
		if j.Path != "a.md" {
			continue
		}
		switch j.Kind {
		case JudgmentSourceIDCollision, JudgmentProseCitation, JudgmentCitationUnattached:
			t.Errorf("§12: held entry reported as %s: %q", j.Kind, j.Context)
		}
	}
	for _, n := range plan.Nodes {
		if n.Path == "a.md" && len(n.Sources)+len(n.IDs)+len(n.Footnotes)+len(n.Refs)+len(n.Titles) > 0 {
			t.Errorf("§12: held entry re-converted: %+v", n)
		}
	}
}
