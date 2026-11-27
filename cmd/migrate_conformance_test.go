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

// TestConformance_MigrateCommandV01ToV03Validates drives the FULL command path
// (plan then apply) end to end and asserts the migrated bundle validates clean
// (§11). The fixture keeps one prose citation, so the default run holds the
// declaration (§12.1) and says so; once that item is resolved by hand, a
// re-plan declares v0.3 (§8.1) and the bundle validates as v0.3. It carries
// the `Conformance` name so `go test ./cmd/ -run Conformance` exercises it.
func TestConformance_MigrateCommandV01ToV03Validates(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")

	out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey")
	if err != nil {
		t.Fatalf("plan phase must exit 0: err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "version: 0.1 -> hold (1 prose-citation/missing-actor item(s)") {
		t.Fatalf("§12.1: plan summary must report the hold:\n%s", out)
	}
	out, err = runOKF(t, "migrate", dir, "--apply", "--plan", planPath)
	if err != nil {
		t.Fatalf("apply phase must exit 0: err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "okf_version declaration held: bundle still declares v0.1") {
		t.Fatalf("§12.1: apply did not report the hold:\n%s", out)
	}
	if vout, verr := runOKF(t, "validate", dir); verr != nil {
		t.Fatalf("§11 validate must pass after migrate: err=%v out=%q", verr, vout)
	}

	// Resolve the judgment item (drop the unverifiable prose line), re-plan,
	// and apply: now nothing relies on the fallback, so v0.3 is declared.
	a := filepath.Join(dir, "a.md")
	if err := os.WriteFile(a, []byte(strings.Replace(readFileStr(t, a), "[1] VERIFIED — checked on-box, no URL here.\n", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath); err != nil {
		t.Fatalf("re-plan: %v\n%s", err, out)
	}
	out, err = runOKF(t, "migrate", dir, "--apply", "--plan", planPath)
	if err != nil || !strings.Contains(out, "bundle valid as v0.3") {
		t.Fatalf("§8.1: apply after resolving did not declare v0.3: err=%v\n%s", err, out)
	}
}
