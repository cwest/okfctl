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
	"os"
	"path/filepath"
	"testing"
)

// mkMigrateBundle writes rel->content files under a temp dir and returns the dir.
func mkMigrateBundle(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func loadMigrate(t *testing.T, dir string) *Bundle {
	t.Helper()
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

// planNodeMigration returns the NodeMigration for rel, or nil when the plan
// carries none for it.
func planNodeMigration(plan MigratePlan, rel string) *NodeMigration {
	for i := range plan.Nodes {
		if plan.Nodes[i].Path == rel {
			return &plan.Nodes[i]
		}
	}
	return nil
}

func findNodeMigration(t *testing.T, plan MigratePlan, rel string) *NodeMigration {
	t.Helper()
	nm := planNodeMigration(plan, rel)
	if nm == nil {
		t.Fatalf("no NodeMigration planned for %s: %+v", rel, plan.Nodes)
	}
	return nm
}

func hasJudgment(plan MigratePlan, rel string, kind JudgmentKind) bool {
	for _, ji := range plan.Judgment {
		if ji.Path == rel && ji.Kind == kind {
			return true
		}
	}
	return false
}

func findJudgment(t *testing.T, plan MigratePlan, rel string, kind JudgmentKind) JudgmentItem {
	t.Helper()
	for _, ji := range plan.Judgment {
		if ji.Path == rel && ji.Kind == kind {
			return ji
		}
	}
	t.Fatalf("no %q judgment item for %s: %+v", kind, rel, plan.Judgment)
	return JudgmentItem{}
}

// --- §13.1 timestamp -> generated.at (deterministic when actor supplied) ------

func TestMigratePlan_TimestampRenameWithActor_Section13_1(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md":     "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n---\n\n# A\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "reference_agent/gemini-2.5-pro")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	nm := findNodeMigration(t, plan, "a.md")
	// §13.1: timestamp becomes generated.at verbatim; §7: by = the supplied actor.
	if nm.Generated == nil {
		t.Fatalf("a.md: no generated edit planned")
	}
	if nm.Generated.At != "2026-05-28T22:53:05+00:00" {
		t.Errorf("generated.at = %q, want the legacy timestamp verbatim", nm.Generated.At)
	}
	if nm.Generated.By != "reference_agent/gemini-2.5-pro" {
		t.Errorf("generated.by = %q, want the supplied actor", nm.Generated.By)
	}
	// The judgment list is empty — the actor was supplied.
	if len(plan.Judgment) != 0 {
		t.Errorf("judgment items = %d, want 0 when actor supplied: %+v", len(plan.Judgment), plan.Judgment)
	}
}

func TestMigratePlan_TimestampRenameWithoutActorIsJudgment_Section7(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md":     "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n---\n\n# A\n",
	})
	b := loadMigrate(t, dir)
	// §7/§13.1: no actor supplied, none inferable -> report & skip, never guess.
	plan, err := PlanMigration(b, "")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	nm := planNodeMigration(plan, "a.md")
	if nm != nil && nm.Generated != nil {
		t.Fatalf("a.md: generated must NOT be planned deterministically without an actor")
	}
	if !hasJudgment(plan, "a.md", JudgmentMissingActor) {
		t.Fatalf("expected a missing-actor judgment item for a.md, got %+v", plan.Judgment)
	}
}

// --- §13.1 # Citations -> sources (URL-bearing = deterministic) ---------------

func TestMigratePlan_UrlCitationsBecomeSources_Section5_1(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\n---\n\n# Def\n\n# Citations\n" +
			"- https://wiki.acme/finance/fpa-handbook\n" +
			"- https://wiki.acme/finance/revenue-recognition\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Sources) != 2 {
		t.Fatalf("sources planned = %d, want 2: %+v", len(nm.Sources), nm.Sources)
	}
	// §5.1: resource per item; order preserved (body order).
	if nm.Sources[0].Resource != "https://wiki.acme/finance/fpa-handbook" {
		t.Errorf("sources[0].resource = %q", nm.Sources[0].Resource)
	}
	if nm.Sources[1].Resource != "https://wiki.acme/finance/revenue-recognition" {
		t.Errorf("sources[1].resource = %q", nm.Sources[1].Resource)
	}
}

func TestMigratePlan_ProseFindingIsJudgmentNeverFabricated_Section5_1(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\n---\n\n# Def\n\n# Citations\n" +
			"[1] VERIFIED — checked on-box, no URL here.\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	// §5.1: resource is REQUIRED; a prose finding has none -> judgment, never a
	// fabricated resource in sources.
	nm := planNodeMigration(plan, "a.md")
	if nm != nil {
		for _, s := range nm.Sources {
			if s.Resource == "" {
				t.Fatalf("a.md: fabricated a source with empty resource: %+v", s)
			}
		}
	}
	if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("expected a prose-citation judgment item for a.md, got %+v", plan.Judgment)
	}
	// The judgment item carries the verbatim item text as context.
	ji := findJudgment(t, plan, "a.md", JudgmentProseCitation)
	if ji.Context == "" {
		t.Fatalf("prose-citation judgment item carries no context: %+v", ji)
	}
}

func TestMigratePlan_BundleRelativeCitationKeepsRelativeForm_Section6(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"other.md": "---\ntype: Concept\n---\n\n# Other\n",
		"a.md": "---\ntype: Metric\n---\n\n# Def\n\n# Citations\n" +
			"- [Other concept](/other.md)\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Sources) != 1 {
		t.Fatalf("sources planned = %d, want 1: %+v", len(nm.Sources), nm.Sources)
	}
	// §6: a bundle-relative link keeps its relative form as the resource.
	if nm.Sources[0].Resource != "/other.md" {
		t.Errorf("sources[0].resource = %q, want the bundle-relative form", nm.Sources[0].Resource)
	}
}

// --- version markers (§8/§12) ------------------------------------------------

func TestMigratePlan_BumpsVersionMarkers_Section8_12(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md":     "---\ntype: Metric\n---\n\n# A\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	if plan.TargetVersion != "0.2" {
		t.Errorf("plan target version = %q, want 0.2", plan.TargetVersion)
	}
}

// --- apply: turns a v0.1 fixture into a bundle valid as v0.2 -----------------

func TestMigrateApply_ProducesValidV02Bundle(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\nkeep_me: yes\n---\n\n# Def\n\n" +
			"# Citations\n- https://wiki.acme/finance/fpa-handbook\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "reference_agent/gemini-2.5-pro")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}

	// Reload and read via the v0.2 reader.
	b2 := loadMigrate(t, dir)
	if b2.OkfVersion != "0.2" {
		t.Errorf("bundle okf_version = %q, want 0.2 after migrate", b2.OkfVersion)
	}
	n := b2.Nodes["a.md"]
	if n == nil {
		t.Fatalf("a.md missing after migrate")
	}
	// §13.1: timestamp is gone, generated present.
	if _, ok := n.Frontmatter["timestamp"]; ok {
		t.Errorf("a.md still carries a legacy timestamp key after migrate")
	}
	gen, ok := n.Generated()
	if !ok || gen.At.IsZero() {
		t.Errorf("a.md generated not readable after migrate: %+v ok=%v", gen, ok)
	}
	if string(gen.By) != "reference_agent/gemini-2.5-pro" {
		t.Errorf("a.md generated.by = %q after migrate", gen.By)
	}
	// §5.1: the URL citation is now a source.
	if got := n.Sources(); len(got) != 1 || got[0].Resource != "https://wiki.acme/finance/fpa-handbook" {
		t.Errorf("a.md sources after migrate = %+v", got)
	}
	// Additive-only: an unrecognized key survives.
	if n.Frontmatter["keep_me"] != "yes" {
		t.Errorf("a.md dropped the unrecognized keep_me key: %+v", n.Frontmatter)
	}
	// Validate clean as v0.2.
	if f := Validate(b2); len(f) != 0 {
		t.Errorf("validate after migrate: %d findings, want 0: %+v", len(f), f)
	}
}

// --- idempotence: a second cycle is a zero-diff no-op ------------------------

func TestMigrateApply_IsIdempotent(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n---\n\n# Def\n\n" +
			"# Citations\n- https://wiki.acme/x\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("first MigrateApply: %v", err)
	}
	after1 := treeHash(t, dir)

	// Second cycle: re-plan on the migrated bundle and re-apply.
	b2 := loadMigrate(t, dir)
	plan2, err := PlanMigration(b2, "human:casey")
	if err != nil {
		t.Fatalf("second MigratePlan: %v", err)
	}
	if err := MigrateApply(dir, b2, plan2); err != nil {
		t.Fatalf("second MigrateApply: %v", err)
	}
	after2 := treeHash(t, dir)
	if after1 != after2 {
		t.Fatalf("migrate is not idempotent: tree changed on the second cycle")
	}
}

// --- negative control: an already-v0.2 bundle is unchanged -------------------

func TestMigrateApply_AlreadyV02IsNoop(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\nstatus: stable\ngenerated: { by: human:casey, at: 2026-06-20T22:53:05Z }\n" +
			"sources:\n  - id: x\n    resource: https://wiki.acme/x\n---\n\n# Def\n",
	})
	before := treeHash(t, dir)
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("MigratePlan: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if after := treeHash(t, dir); before != after {
		t.Fatalf("migrate changed an already-v0.2 bundle (negative control failed)")
	}
}
