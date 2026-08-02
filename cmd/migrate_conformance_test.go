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
	"path/filepath"
	"strings"
	"testing"
)

// TestConformance_MigrateCommandV01ToV02Validates drives the FULL command path
// (plan then apply) end to end and asserts the migrated bundle validates clean
// as v0.2 (§11/§13.1). It carries the `Conformance` name so the card's
// `go test ./cmd/ -run Conformance` gate exercises it.
func TestConformance_MigrateCommandV01ToV02Validates(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")

	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("§13.1 plan phase must exit 0: err=%v out=%q", err, out)
	}
	out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath)
	if err != nil {
		t.Fatalf("§13.1 apply phase must exit 0: err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "bundle valid as v0.2") {
		t.Fatalf("apply did not report v0.2 validity:\n%s", out)
	}
	if vout, verr := runOKF(t, "validate", dir); verr != nil {
		t.Fatalf("§11 validate must pass after migrate: err=%v out=%q", verr, vout)
	}
}
