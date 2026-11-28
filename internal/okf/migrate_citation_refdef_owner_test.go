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

const alsoReferenced = "- Also referenced: Thing <https://example.org/t> · " +
	"Other <https://example.org/o> · Third <https://example.org/x>.\n"

// TestConformance_MigrateRefDefOwner_Section5_1_MultiURLEntrySplits covers a
// multi-resource entry whose first resource a link reference definition
// (`[t]: url`) also names. The definition is the target of a `[text][t]` link,
// not a citation: it gets no id and owns nothing, so the entry splits as it
// would without it, one source per resource, each with an id. Before, the
// definition owned the resource, the entry was rejected as a split and stayed
// a prose-citation, and resolving it gave one source for three resources.
func TestConformance_MigrateRefDefOwner_Section5_1_MultiURLEntrySplits(t *testing.T) {
	// The entry's id is positional (CitationEntryIDs): cite-1 when it is
	// listed first, cite-2 after the definition.
	for name, tc := range map[string]struct{ list, id string }{
		"definition after the entry":  {alsoReferenced + "\n[t]: https://example.org/t\n", "cite-1"},
		"definition before the entry": {"[t]: https://example.org/t\n\n" + alsoReferenced, "cite-2"},
	} {
		t.Run(name, func(t *testing.T) {
			node := "---\ntype: Concept\n---\n\nClaim. See [Thing][t].\n\n# Citations\n\n" + tc.list
			dir, plan := migrateOnce(t, kb(node))
			for _, j := range plan.Judgment {
				if j.Path == "a.md" && (j.Kind == JudgmentProseCitation || j.Kind == JudgmentCitationUnattached) {
					t.Errorf("§5.1: %s: %q", j.Kind, j.Context)
				}
			}
			nm := findNodeMigration(t, plan, "a.md")
			ids := map[string]string{}
			for _, s := range nm.Sources {
				ids[s.Resource] = s.ID
			}
			for res, id := range map[string]string{
				"https://example.org/t": tc.id,
				"https://example.org/o": tc.id + "-2",
				"https://example.org/x": tc.id + "-3",
			} {
				if ids[res] != id {
					t.Errorf("§5.1: source %s id = %q, want %q (sources %+v)", res, ids[res], id, nm.Sources)
				}
			}
			out := readRel(t, dir, "a.md")
			wantIn(t, out, "[^"+tc.id+"]: Also referenced:", "[^"+tc.id+"-2]: https://example.org/o", "[^"+tc.id+"-3]: https://example.org/x")
			mustValidate(t, dir)
			assertReplanQuiet(t, dir)
		})
	}
}

// TestConformance_MigrateRefDefOwner_Section5_1_SingleEntryAfterDefinition is
// the single-resource shape of the same defect: a definition listed before an
// entry that names the same resource claimed it first, so the entry was a
// citation-unattached item. The entry owns the resource and takes the id.
func TestConformance_MigrateRefDefOwner_Section5_1_SingleEntryAfterDefinition(t *testing.T) {
	node := "---\ntype: Concept\n---\n\nClaim [1]. See [Thing][t].\n\n# Citations\n\n" +
		"[t]: https://example.org/t\n\n[1] Thing https://example.org/t\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentCitationUnattached) {
		t.Errorf("§5.1: [1] is the only citation of its resource and must attach: %+v", plan.Judgment)
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out, "Claim [^1].", "[^1]: Thing https://example.org/t")
	mustValidate(t, dir)
	assertReplanQuiet(t, dir)
}

// assertReplanQuiet re-plans a migrated bundle and fails on any edit (§12:
// migration is idempotent).
func assertReplanQuiet(t *testing.T, dir string) {
	t.Helper()
	_, again := planFor(t, dir, "", false)
	for _, n := range again.Nodes {
		if len(n.Sources)+len(n.IDs)+len(n.Footnotes)+len(n.Refs)+len(n.Titles) > 0 {
			t.Errorf("§12: re-plan has edits: %+v\n%s", n, strings.TrimSpace(readRel(t, dir, n.Path)))
		}
	}
}
