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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// migrateFixtureFiles is a v0.1 bundle: one node with a legacy timestamp and a
// # Citations list carrying a URL source and a prose finding, plus an
// unrecognized frontmatter key that must survive (additive-only).
func migrateFixtureFiles() map[string]string {
	return map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n",
		"log.md":   "# Change Log\n\n_No entries yet._\n",
		"a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\nkeep_me: preserved\n---\n\n" +
			"# Definition\n\nThe metric.\n\n# Citations\n" +
			"- https://wiki.acme/finance/fpa-handbook\n" +
			"[1] VERIFIED — checked on-box, no URL here.\n",
	}
}

// `migrate --plan` writes a plan file and touches NO node (pure read).
func TestMigrate_PlanWritesPlanFileOnly(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	before := cliTreeHash(t, dir)
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")

	out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey")
	if err != nil {
		t.Fatalf("migrate --plan must exit 0: err=%v out=%q", err, out)
	}
	// No node touched — planning is pure read.
	if after := cliTreeHash(t, dir); before != after {
		t.Fatalf("migrate --plan mutated the bundle; it must be pure read")
	}
	// The plan file exists and carries the deterministic + judgment split.
	planJSON := readFileStr(t, planPath)
	if !strings.Contains(planJSON, "\"target_version\": \"0.2\"") {
		t.Fatalf("plan missing target_version 0.2:\n%s", planJSON)
	}
	if !strings.Contains(planJSON, "fpa-handbook") {
		t.Fatalf("plan missing the URL source:\n%s", planJSON)
	}
	if !strings.Contains(planJSON, "prose-citation") {
		t.Fatalf("plan missing the prose-citation judgment item:\n%s", planJSON)
	}
}

// `migrate --apply` reads the plan back, applies it, and leaves a bundle that
// validates clean AS v0.2; the unrecognized key survives (additive-only).
func TestMigrate_ApplyProducesValidV02(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")

	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan phase must exit 0: err=%v out=%q", err, out)
	}
	if out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil {
		t.Fatalf("apply phase must exit 0: err=%v out=%q", err, out)
	}

	// validate exits 0 (bundle now reads as v0.2).
	if out, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate must pass after migrate: err=%v out=%q", err, out)
	}
	node := readFileStr(t, filepath.Join(dir, "a.md"))
	if strings.Contains(node, "timestamp:") {
		t.Errorf("a.md still carries a legacy timestamp:\n%s", node)
	}
	if !strings.Contains(node, "generated:") {
		t.Errorf("a.md missing generated after migrate:\n%s", node)
	}
	if !strings.Contains(node, "keep_me: preserved") {
		t.Errorf("a.md dropped the unrecognized keep_me key (additive-only broken):\n%s", node)
	}
	if !strings.Contains(node, "fpa-handbook") {
		t.Errorf("a.md missing the migrated source:\n%s", node)
	}
	// .okf sidecar bumped to 0.2.
	okf := readFileStr(t, filepath.Join(dir, ".okf"))
	if !strings.Contains(okf, "0.2") {
		t.Errorf(".okf not bumped to 0.2:\n%s", okf)
	}
}

// `migrate --apply --dry-run` is BYTE-IDENTICAL to a real apply: the property
// PR #38 gated on for bulk promote. Applying twice from the same plan — once
// dry, once real — leaves the same bytes as the real run alone.
func TestMigrate_ApplyDryRunByteIdentical(t *testing.T) {
	// Real run on a first copy.
	realDir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	realPlan := filepath.Join(t.TempDir(), "p.json")
	if _, err := runOKF(t, "migrate", realDir, "--plan", realPlan, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("real plan: %v", err)
	}
	if _, err := runOKF(t, "migrate", realDir, "--apply", "--plan", realPlan); err != nil {
		t.Fatalf("real apply: %v", err)
	}
	realHash := cliTreeHash(t, realDir)

	// Dry run then real run on a second identical copy.
	dryDir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	dryPlan := filepath.Join(t.TempDir(), "p.json")
	if _, err := runOKF(t, "migrate", dryDir, "--plan", dryPlan, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("dry plan: %v", err)
	}
	before := cliTreeHash(t, dryDir)
	if _, err := runOKF(t, "migrate", dryDir, "--apply", "--plan", dryPlan, "--dry-run"); err != nil {
		t.Fatalf("dry apply: %v", err)
	}
	if after := cliTreeHash(t, dryDir); before != after {
		t.Fatalf("--dry-run wrote to disk: tree hash changed")
	}
	// Now a real apply on the dry copy must reach byte-identical state.
	if _, err := runOKF(t, "migrate", dryDir, "--apply", "--plan", dryPlan); err != nil {
		t.Fatalf("dry-then-real apply: %v", err)
	}
	if cliTreeHash(t, dryDir) != realHash {
		t.Fatalf("dry-run+real diverged from real-only: not byte-identical")
	}
}

// A second plan+apply cycle produces ZERO diff (idempotent).
func TestMigrate_Idempotent(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	p1 := filepath.Join(t.TempDir(), "p1.json")
	if _, err := runOKF(t, "migrate", dir, "--plan", p1, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan1: %v", err)
	}
	if _, err := runOKF(t, "migrate", dir, "--apply", "--plan", p1); err != nil {
		t.Fatalf("apply1: %v", err)
	}
	after1 := cliTreeHash(t, dir)

	p2 := filepath.Join(t.TempDir(), "p2.json")
	if _, err := runOKF(t, "migrate", dir, "--plan", p2, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan2: %v", err)
	}
	if _, err := runOKF(t, "migrate", dir, "--apply", "--plan", p2); err != nil {
		t.Fatalf("apply2: %v", err)
	}
	if cliTreeHash(t, dir) != after1 {
		t.Fatalf("migrate is not idempotent: second cycle changed the tree")
	}
}

// Negative control: `migrate` against an already-v0.2 bundle changes nothing.
func TestMigrate_AlreadyV02NoChange(t *testing.T) {
	dir := mkPromoteCLIBundle(t, map[string]string{
		".okf":     "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Metric\nstatus: stable\ngenerated: { by: human:casey, at: 2026-06-20T22:53:05Z }\n" +
			"sources:\n  - id: x\n    resource: https://wiki.acme/x\n---\n\n# Def\n",
	})
	before := cliTreeHash(t, dir)
	planPath := filepath.Join(t.TempDir(), "p.json")
	if _, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cliTreeHash(t, dir) != before {
		t.Fatalf("migrate changed an already-v0.2 bundle (negative control failed)")
	}
}

// apply without a plan file is a clear error, not a silent no-op.
func TestMigrate_ApplyWithoutPlanErrors(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	out, err := runOKF(t, "migrate", dir, "--apply", "--plan", missing)
	if err == nil {
		t.Fatalf("apply with a missing plan must error; out=%q", out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "a.md")); statErr != nil {
		t.Fatalf("node vanished on a failed apply: %v", statErr)
	}
}
