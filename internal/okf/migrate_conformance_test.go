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

// Spec-conformance suite for the v0.1 -> v0.2 migrator. okfctl is the producer
// of a spec-governed format, so its migrator must be its own first consumer:
// the closed loop here is migrate(v0.1 bundle) -> Validate == 0 findings AND the
// v0.2 reader (PR #45) reads back every renamed family. These tests carry the
// `Conformance` name so `-run Conformance` exercises them, matching the house
// pattern in conformance_test.go and provenance_conformance_test.go.
//
//   - §13.1: the two breaking renames (timestamp -> generated.at; body
//     # Citations -> frontmatter sources).
//   - §11:   additive-only — an unrecognized key is never dropped.
//   - §5.1:  resource is REQUIRED, so a prose finding is never fabricated into a
//     source; it becomes a judgment item instead.
//   - §12:   an already-v0.2 bundle is a no-op (negative control).

// TestConformance_MigrateV01ToV02ValidatesAndReadsBack is the load-bearing
// closed loop: a v0.1 fixture, migrated, validates clean AND the v0.2 reader
// reads back generated (§5.2/§13.1) and sources (§5.1/§13.1).
func TestConformance_MigrateV01ToV02ValidatesAndReadsBack(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n---\n\n" +
			"# Def\n\n# Citations\n- https://wiki.acme/x\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "reference_agent/gemini-2.5-pro")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}

	b2 := loadMigrate(t, dir)
	if b2.OkfVersion != "0.2" {
		t.Fatalf("§12: bundle version = %q, want 0.2", b2.OkfVersion)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Fatalf("§11: migrated bundle has %d findings, want 0: %+v", len(f), f)
	}
	n := b2.Nodes["a.md"]
	// §5.2/§13.1: generated reads back with the carried-over instant and actor.
	gen, ok := n.Generated()
	if !ok || string(gen.By) != "reference_agent/gemini-2.5-pro" {
		t.Errorf("§5.2: generated = %+v ok=%v", gen, ok)
	}
	// §5.1/§13.1: the URL citation reads back as a source.
	if s := n.Sources(); len(s) != 1 || s[0].Resource != "https://wiki.acme/x" {
		t.Errorf("§5.1: sources = %+v", s)
	}
}

// TestConformance_MigrateIsAdditiveOnly_Section11 proves an unrecognized
// frontmatter key survives the round-trip untouched (§11 forbids rejecting
// unknown keys; a migrator that discards them is strictly worse).
func TestConformance_MigrateIsAdditiveOnly_Section11(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md":     "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\nvendor_ext: keep-me\n---\n\n# A\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	n := loadMigrate(t, dir).Nodes["a.md"]
	if n.Frontmatter["vendor_ext"] != "keep-me" {
		t.Fatalf("§11: additive-only broken — unrecognized key dropped: %+v", n.Frontmatter)
	}
}

// TestConformance_MigrateNeverFabricatesResource_Section5_1 proves a prose
// finding with no follow-able resource becomes a judgment item, never a source
// with an invented resource (§5.1 REQUIRES resource).
func TestConformance_MigrateNeverFabricatesResource_Section5_1(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md":     "---\ntype: Metric\n---\n\n# Def\n\n# Citations\n[1] VERIFIED — on-box probe, no URL.\n",
	})
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	for _, nm := range plan.Nodes {
		for _, s := range nm.Sources {
			if strings.TrimSpace(s.Resource) == "" {
				t.Fatalf("§5.1: fabricated a source with an empty resource: %+v", s)
			}
		}
	}
	if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: prose finding must be a judgment item, got %+v", plan.Judgment)
	}
}

// TestConformance_MigrateAlreadyV02IsNoop_Section12 is the negative control: a
// bundle already at v0.2 is left byte-identical (§12: a consumer at the declared
// version has nothing to migrate).
func TestConformance_MigrateAlreadyV02IsNoop_Section12(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\ngenerated: { by: human:casey, at: 2026-06-20T22:53:05Z }\n" +
			"sources:\n  - resource: https://wiki.acme/x\n---\n\n# Def\n",
	})
	before := treeHash(t, dir)
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "human:casey")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if treeHash(t, dir) != before {
		t.Fatalf("§12: already-v0.2 bundle was changed (negative control failed)")
	}
}
