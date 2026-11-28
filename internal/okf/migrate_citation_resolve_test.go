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

// v0.3 §5.1: a `resource` is a concrete artifact a consumer can follow, or a
// population or scope descriptor it cannot. A prose-citation item has no
// resource the tool can read, so only the person resolving it can supply one.
// They write it on the plan item (`resource`, optionally `title`), and the
// next plan takes it from there (MigrateOptions.Resolutions): the entry gets
// the source, id, footnote and reference edits any converted entry gets.

const descriptorNode = "---\ntype: Concept\n---\n\n# Def\n\nPick by occasion [5]. The law is strict [1].\n\n# Citations\n\n" +
	"[1] VERIFIED — the production rules https://rules.example/brunello\n\n" +
	"[5] VERIFIED (internal) — the `brunello-selection` skill in this system, drawn on for the list section.\n"

const descriptorEntry = "[5] VERIFIED (internal) — the `brunello-selection` skill in this system, drawn on for the list section."

const brunelloDescriptor = "the private brunello-selection skill on the authoring host (not published)"

// resolvedPlan plans dir with the prose-citation item for entry resolved to
// resource.
func resolvedPlan(t *testing.T, dir, entry, resource string) (*Bundle, MigratePlan) {
	t.Helper()
	b := loadMigrate(t, dir)
	plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
		{Path: "a.md", Kind: JudgmentProseCitation, Context: entry, Resource: resource},
	}})
	if err != nil {
		t.Fatalf("PlanMigrationWith: %v", err)
	}
	return b, plan
}

// Positive control, real corpus shape (wine/brunello-di-montalcino.md [5]):
// the item is resolved to a descriptor. The entry becomes a source with that
// resource and id 5, its full text the footnote [^5], the body [5] becomes
// [^5], and the list, now held, is removed (§4.2). A re-plan of the result
// is clean.
func TestConformance_MigrateResolve_Section5_1_DescriptorResolution(t *testing.T) {
	dir := mkMigrateBundle(t, kb(descriptorNode))
	_, first := planFor(t, dir, "", false)
	item := findJudgment(t, first, "a.md", JudgmentProseCitation)
	if item.Context != descriptorEntry {
		t.Fatalf("prose-citation context = %q, want %q", item.Context, descriptorEntry)
	}
	b, plan := resolvedPlan(t, dir, item.Context, brunelloDescriptor)
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: a resolved item is no longer a prose-citation: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if got, ok := sourceByID(nm, "5"); !ok || got.Resource != brunelloDescriptor {
		t.Fatalf("§5.1: source 5 = %+v (found %v), want resource %q", got, ok, brunelloDescriptor)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out,
		"Pick by occasion [^5]. The law is strict [^1].\n",
		"[^5]: VERIFIED (internal) — the `brunello-selection` skill in this system, drawn on for the list section.\n",
		"resource: "+brunelloDescriptor,
	)
	if strings.Contains(out, "# Citations") {
		t.Errorf("§4.2: every entry is held, so the list goes:\n%s", out)
	}
	mustValidate(t, dir)

	_, again := planFor(t, dir, "", false)
	if nm := planNodeMigration(again, "a.md"); nm != nil {
		t.Errorf("§12: a re-plan of the result plans no edits, got %+v", *nm)
	}
	for _, j := range again.Judgment {
		if j.Path == "a.md" {
			t.Errorf("a re-plan of the result reports nothing for a.md, got %+v", j)
		}
	}
}

// A resolution may also give the work's name; it becomes the §5.1 title.
func TestConformance_MigrateResolve_Section5_1_ResolutionTitle(t *testing.T) {
	dir := mkMigrateBundle(t, kb(descriptorNode))
	b := loadMigrate(t, dir)
	plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{{
		Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry,
		Resource: brunelloDescriptor, Title: "brunello-selection skill",
	}}})
	if err != nil {
		t.Fatalf("PlanMigrationWith: %v", err)
	}
	got, ok := sourceByID(findNodeMigration(t, plan, "a.md"), "5")
	if !ok || got.Title != "brunello-selection skill" {
		t.Errorf("§5.1: source 5 = %+v (found %v), want title from the resolution", got, ok)
	}
	for _, j := range plan.Judgment {
		if j.Kind == JudgmentSourceTitleUnnamed && j.Context == descriptorEntry {
			t.Errorf("a titled resolution is not unnamed: %+v", j)
		}
	}
}

// Positive control, kept list: the section also holds a status-key line, so
// the list stays (citations-kept). The resolved entry stays in it as text,
// and a re-plan WITHOUT the resolution still reads it as converted: its text
// is the footnote of a source with an id, so it is not a prose-citation again
// and the version is not held for it.
func TestConformance_MigrateResolve_Section5_1_KeptListReplanRecognisesHeldEntry(t *testing.T) {
	node := strings.Replace(descriptorNode, "# Citations\n\n", "# Citations\n\nStatus key: VERIFIED = read against the source.\n\n", 1)
	dir := mkMigrateBundle(t, kb(node))
	b, plan := resolvedPlan(t, dir, descriptorEntry, brunelloDescriptor)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out, "# Citations", descriptorEntry, "[^5]: VERIFIED (internal)")
	mustValidate(t, dir)

	_, again := planFor(t, dir, "", false)
	if hasJudgment(again, "a.md", JudgmentProseCitation) {
		t.Errorf("§5.1: a held entry is not a prose-citation on re-plan: %+v", again.Judgment)
	}
	if hasJudgment(again, "a.md", JudgmentSourceIDCollision) {
		t.Errorf("the held entry owns [^5]: %+v", again.Judgment)
	}
	if nm := planNodeMigration(again, "a.md"); nm != nil {
		t.Errorf("§12: a re-plan plans no edits, got %+v", *nm)
	}
	kept := findJudgment(t, again, "a.md", JudgmentCitationsKept)
	if strings.Contains(kept.Context, "entry [5]") {
		t.Errorf("§4.2: entry [5] is held by [^5], only the status line keeps the list: %q", kept.Context)
	}
}

// Negative control: an entry with no resource is held only when a footnote
// of a source with an id holds its text. A source id with a footnote that
// says something else, or with no footnote, leaves it a prose-citation.
func TestConformance_MigrateResolve_Section5_1_UnheldEntryStaysProse(t *testing.T) {
	for name, extra := range map[string]string{
		"other text":  "[^5]: a different note entirely.\n\n",
		"no footnote": "",
	} {
		t.Run(name, func(t *testing.T) {
			node := strings.Replace(descriptorNode, "---\n\n# Def", "sources:\n  - id: \"5\"\n    resource: \""+brunelloDescriptor+"\"\n---\n\n# Def", 1)
			node = strings.Replace(node, "# Citations\n", extra+"# Citations\n", 1)
			dir := mkMigrateBundle(t, kb(node))
			_, plan := planFor(t, dir, "", false)
			if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
				t.Errorf("§5.1: no footnote holds [5]'s text, so it stays a prose-citation: %+v", plan.Judgment)
			}
		})
	}
}

// Negative controls: a resolution is honored only for a prose-citation item
// whose text the plan still finds. A stale one (the entry changed), or one on
// another kind of item, fails the plan rather than being dropped unnoticed.
// An item with no resource is not a resolution and is ignored.
func TestConformance_MigrateResolve_Section5_1_ResolutionMustMatch(t *testing.T) {
	dir := mkMigrateBundle(t, kb(descriptorNode))
	b := loadMigrate(t, dir)
	for name, r := range map[string]JudgmentItem{
		"stale context":  {Path: "a.md", Kind: JudgmentProseCitation, Context: "[5] some older text", Resource: brunelloDescriptor},
		"other path":     {Path: "b.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: brunelloDescriptor},
		"not prose kind": {Path: "a.md", Kind: JudgmentMissingActor, Context: descriptorEntry, Resource: brunelloDescriptor},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{r}}); err == nil {
				t.Errorf("a resolution that matches no prose-citation item must fail the plan: %+v", r)
			}
		})
	}
	plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
		{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry},
		{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: "  "},
	}})
	if err != nil {
		t.Fatalf("an unresolved item is not a resolution: %v", err)
	}
	if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Errorf("an item with no resource stays a prose-citation: %+v", plan.Judgment)
	}
}

// §12: planning again with the same resolution once it is applied is a
// no-op. The kept list still holds the entry, now held by a source with that
// resource, so the resolution is satisfied, not stale. One naming a
// different resource for the held entry fails: the plan cannot honor it.
func TestConformance_MigrateResolve_Section12_ReplayedResolutionIsNoop(t *testing.T) {
	node := strings.Replace(descriptorNode, "# Citations\n\n", "# Citations\n\nStatus key: VERIFIED = read against the source.\n\n", 1)
	dir := mkMigrateBundle(t, kb(node))
	b, plan := resolvedPlan(t, dir, descriptorEntry, brunelloDescriptor)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	_, again := resolvedPlan(t, dir, descriptorEntry, brunelloDescriptor)
	if nm := planNodeMigration(again, "a.md"); nm != nil {
		t.Errorf("§12: a replayed resolution plans no edits, got %+v", *nm)
	}
	b = loadMigrate(t, dir)
	if _, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
		{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: "some other descriptor"},
	}}); err == nil {
		t.Errorf("a resolution that disagrees with the held source must fail the plan")
	} else if !strings.Contains(err.Error(), "already resolved") || !strings.Contains(err.Error(), brunelloDescriptor) {
		t.Errorf("the error names the resource the entry already has, got: %v", err)
	}
}

// §12.1: a resolution to a root-relative bundle path is written `/`-rooted
// when the node sits below the root (the form its body link would be
// rewritten to). The list is kept, so the entry stays in it. Replaying the
// resolution after the apply matches the source it made, whether another
// prose-citation still holds the declaration (the fallback still applies) or
// the bundle now declares v0.3 (it no longer does). The replay is a no-op,
// not a stale-resolution failure.
func TestConformance_MigrateResolve_Section12_ReplayedRootRelativePathIsNoop(t *testing.T) {
	const entry = "[5] VERIFIED (internal) — the spec page in this bundle."
	const other = "[7] VERIFIED — said on a call, not written down anywhere."
	const nodeDoc = "---\ntype: Concept\n---\n\n# Def\n\nThe rule holds [5].\n\n# Citations\n\n" +
		"Status key: VERIFIED = read against the source.\n\n" + entry + "\n"
	for name, held := range map[string]bool{"declaration held": true, "declared v0.3": false} {
		t.Run(name, func(t *testing.T) {
			doc := nodeDoc
			if held {
				doc += "\n" + other + "\n"
			}
			dir := mkMigrateBundle(t, map[string]string{
				"index.md": "# KB\n", "log.md": "# Log\n",
				"refs/spec.md": "---\ntype: Concept\n---\n\n# Spec\n\nText.\n",
				"sub/a.md":     doc,
			})
			resolve := func() (*Bundle, MigratePlan, error) {
				b := loadMigrate(t, dir)
				plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
					{Path: "sub/a.md", Kind: JudgmentProseCitation, Context: entry, Resource: "refs/spec.md"},
				}})
				return b, plan, err
			}
			b, plan, err := resolve()
			if err != nil {
				t.Fatalf("PlanMigrationWith: %v", err)
			}
			if got, ok := sourceByID(findNodeMigration(t, plan, "sub/a.md"), "5"); !ok || got.Resource != "/refs/spec.md" {
				t.Fatalf("§12.1: source 5 = %+v (found %v), want the /-rooted path", got, ok)
			}
			if err := MigrateApply(dir, b, plan); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if declared := loadMigrate(t, dir).DeclaredOkfVersion; held == sameVersion(declared, TargetOkfVersion) {
				t.Fatalf("fixture: held=%v but the bundle declares %q", held, declared)
			}
			wantIn(t, readRel(t, dir, "sub/a.md"), "# Citations", entry)
			_, again, err := resolve()
			if err != nil {
				t.Fatalf("§12: replaying the resolution after apply must not fail: %v", err)
			}
			if nm := planNodeMigration(again, "sub/a.md"); nm != nil {
				t.Errorf("§12: a replayed resolution plans no edits, got %+v", *nm)
			}
			if again.Resolved != 0 {
				t.Errorf("a replayed resolution converts nothing new: Resolved = %d", again.Resolved)
			}
		})
	}
}

// MigratePlan.Resolved counts the entries this plan converts from a
// resolution: one for a fresh resolution even when the same item is listed
// twice, none on a replay after the apply.
func TestConformance_MigrateResolve_Section5_1_ResolvedCountsConvertedEntries(t *testing.T) {
	node := strings.Replace(descriptorNode, "# Citations\n\n", "# Citations\n\nStatus key: VERIFIED = read against the source.\n\n", 1)
	dir := mkMigrateBundle(t, kb(node))
	b := loadMigrate(t, dir)
	r := JudgmentItem{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: brunelloDescriptor}
	plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{r, r}})
	if err != nil {
		t.Fatalf("PlanMigrationWith: %v", err)
	}
	if plan.Resolved != 1 {
		t.Errorf("one entry resolved (listed twice): Resolved = %d, want 1", plan.Resolved)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	_, again := resolvedPlan(t, dir, descriptorEntry, brunelloDescriptor)
	if again.Resolved != 0 {
		t.Errorf("a replay converts nothing: Resolved = %d, want 0", again.Resolved)
	}
}

// §12: when every entry is held, the apply removes the `# Citations` list, so
// the entry a resolution names is no longer in any list. Replaying the same
// resolution finds it in the footnote of the source it made and is a no-op.
// One naming a different resource fails, and the error names the resource
// the entry already has.
func TestConformance_MigrateResolve_Section12_ReplayOnRemovedListIsNoop(t *testing.T) {
	dir := mkMigrateBundle(t, kb(descriptorNode))
	b, plan := resolvedPlan(t, dir, descriptorEntry, brunelloDescriptor)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); strings.Contains(out, "# Citations") {
		t.Fatalf("fixture: every entry is held, so the list goes:\n%s", out)
	}
	b = loadMigrate(t, dir)
	again, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
		{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: brunelloDescriptor},
	}})
	if err != nil {
		t.Fatalf("§12: replaying the resolution after the list is removed must not fail: %v", err)
	}
	if nm := planNodeMigration(again, "a.md"); nm != nil {
		t.Errorf("§12: a replayed resolution plans no edits, got %+v", *nm)
	}
	if again.Resolved != 0 {
		t.Errorf("a replay converts nothing: Resolved = %d, want 0", again.Resolved)
	}
	_, err = PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
		{Path: "a.md", Kind: JudgmentProseCitation, Context: descriptorEntry, Resource: "some other descriptor"},
	}})
	if err == nil {
		t.Fatalf("a resolution that disagrees with the held source must fail the plan")
	}
	if !strings.Contains(err.Error(), "already resolved") || !strings.Contains(err.Error(), brunelloDescriptor) {
		t.Errorf("the error names the resource the entry already has, got: %v", err)
	}
}

// §12.1: the same replay on a removed list, resolved to a root-relative
// bundle path from a node below the root, matches the `/`-rooted source.
func TestConformance_MigrateResolve_Section12_ReplayedPathOnRemovedListIsNoop(t *testing.T) {
	const entry = "[5] VERIFIED (internal) — the spec page in this bundle."
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n",
		"refs/spec.md": "---\ntype: Concept\n---\n\n# Spec\n\nText.\n",
		"sub/a.md":     "---\ntype: Concept\n---\n\n# Def\n\nThe rule holds [5].\n\n# Citations\n\n" + entry + "\n",
	})
	resolve := func() (*Bundle, MigratePlan, error) {
		b := loadMigrate(t, dir)
		plan, err := PlanMigrationWith(b, MigrateOptions{Resolutions: []JudgmentItem{
			{Path: "sub/a.md", Kind: JudgmentProseCitation, Context: entry, Resource: "refs/spec.md"},
		}})
		return b, plan, err
	}
	b, plan, err := resolve()
	if err != nil {
		t.Fatalf("PlanMigrationWith: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "sub/a.md")
	if strings.Contains(out, "# Citations") {
		t.Fatalf("fixture: every entry is held, so the list goes:\n%s", out)
	}
	wantIn(t, out, "resource: /refs/spec.md")
	_, again, err := resolve()
	if err != nil {
		t.Fatalf("§12: replaying the resolution after the list is removed must not fail: %v", err)
	}
	if nm := planNodeMigration(again, "sub/a.md"); nm != nil {
		t.Errorf("§12: a replayed resolution plans no edits, got %+v", *nm)
	}
	if again.Resolved != 0 {
		t.Errorf("a replay converts nothing: Resolved = %d, want 0", again.Resolved)
	}
}
