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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

const resolveDescriptor = "on-box check by the author, 2026-05-28 (no public source)"

// resolvePlanFile writes the prose-citation item of the plan at path back
// with resource filled in, the way a person resolves it in an editor.
func resolvePlanFile(t *testing.T, path, resource string) {
	t.Helper()
	var plan okf.MigratePlan
	if err := json.Unmarshal([]byte(readFileStr(t, path)), &plan); err != nil {
		t.Fatalf("parse plan: %v", err)
	}
	n := 0
	for i := range plan.Judgment {
		if plan.Judgment[i].Kind == okf.JudgmentProseCitation {
			plan.Judgment[i].Resource = resource
			n++
		}
	}
	if n != 1 {
		t.Fatalf("fixture plan has %d prose-citation items, want 1: %+v", n, plan.Judgment)
	}
	data, _ := json.MarshalIndent(plan, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// §5.1: a person resolves a prose-citation item by writing a resource (here a
// scope descriptor) on it in the plan file; `migrate --resolve <plan>` plans
// the entry as a source, apply writes it, and the declaration is no longer
// held for it. A fresh plan of the result has no edits.
func TestMigrate_ResolveProseCitationFromPlanFile_Section5_1(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	resolvePlanFile(t, planPath, resolveDescriptor)

	out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--resolve", planPath, "--generated-by", "human:casey")
	if err != nil {
		t.Fatalf("plan --resolve: %v\n%s", err, out)
	}
	if !strings.Contains(out, "resolved: 1 prose-citation item(s)") {
		t.Errorf("summary must count the resolution:\n%s", out)
	}
	if !strings.Contains(out, "-> declare v0.3") {
		t.Errorf("§12.1: nothing is left on the fallback, so the plan declares v0.3:\n%s", out)
	}
	if strings.Contains(readFileStr(t, planPath), `"prose-citation"`) {
		t.Errorf("the resolved item is gone from the new plan:\n%s", readFileStr(t, planPath))
	}
	if out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	node := readFileStr(t, filepath.Join(dir, "a.md"))
	for _, w := range []string{"resource: \"" + resolveDescriptor + "\"", "[^1]: VERIFIED — checked on-box, no URL here."} {
		if !strings.Contains(node, w) {
			t.Errorf("a.md missing %q:\n%s", w, node)
		}
	}
	if out, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	again := filepath.Join(t.TempDir(), "again.json")
	out, err = runOKF(t, "migrate", dir, "--plan", again)
	if err != nil {
		t.Fatalf("re-plan: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0 file(s) with deterministic edits") || strings.Contains(readFileStr(t, again), `"prose-citation"`) {
		t.Errorf("§12: a re-plan of the result is clean:\n%s\n%s", out, readFileStr(t, again))
	}
}

// Negative control: apply never acts on a resolution. A plan whose item
// carries a resource it has not planned is refused with the command that
// plans it, and nothing is written.
func TestMigrate_ApplyRefusesUnplannedResolution_Section5_1(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	resolvePlanFile(t, planPath, resolveDescriptor)
	before := cliTreeHash(t, dir)
	for _, args := range [][]string{
		{"migrate", dir, "--apply", "--plan", planPath},
		{"migrate", dir, "--apply", "--plan", planPath, "--dry-run"},
		{"migrate", dir, "--apply", "--plan", planPath, "--resolve", planPath},
	} {
		out, err := runOKF(t, args...)
		if err == nil || !strings.Contains(err.Error()+out, "--resolve") {
			t.Errorf("%v must fail naming --resolve: err=%v out=%q", args, err, out)
		}
	}
	if cliTreeHash(t, dir) != before {
		t.Errorf("a refused apply wrote to the bundle")
	}
}

// Negative control: a resolution whose entry is gone fails the plan, and the
// plan file given to --resolve is left as it was.
func TestMigrate_ResolveStaleItemFails_Section5_1(t *testing.T) {
	dir := mkPromoteCLIBundle(t, migrateFixtureFiles())
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	resolvePlanFile(t, planPath, resolveDescriptor)
	resolved := readFileStr(t, planPath)
	a := filepath.Join(dir, "a.md")
	if err := os.WriteFile(a, []byte(strings.Replace(readFileStr(t, a), "no URL here", "no link here", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--resolve", planPath, "--generated-by", "human:casey")
	if err == nil || !strings.Contains(err.Error()+out, "match no prose-citation item") {
		t.Errorf("a stale resolution must fail the plan: err=%v out=%q", err, out)
	}
	if readFileStr(t, planPath) != resolved {
		t.Errorf("a failed plan must not overwrite the resolved plan file")
	}
}

// The resolved line reports what the plan converted. A replay after the
// apply converts nothing, so it prints no resolved line beside its
// 0-edit summary.
func TestMigrate_ResolveReplayReportsNothingResolved_Section5_1(t *testing.T) {
	files := migrateFixtureFiles()
	files["a.md"] = strings.Replace(files["a.md"], "# Citations\n", "# Citations\nStatus key: VERIFIED = read against the source.\n\n", 1)
	dir := mkPromoteCLIBundle(t, files)
	planPath := filepath.Join(t.TempDir(), "migrate-plan.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	resolvePlanFile(t, planPath, resolveDescriptor)
	resolutions := filepath.Join(t.TempDir(), "resolutions.json")
	if err := os.WriteFile(resolutions, []byte(readFileStr(t, planPath)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--resolve", resolutions, "--generated-by", "human:casey"); err != nil {
		t.Fatalf("plan --resolve: %v\n%s", err, out)
	}
	if out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	again := filepath.Join(t.TempDir(), "again.json")
	out, err := runOKF(t, "migrate", dir, "--plan", again, "--resolve", resolutions, "--generated-by", "human:casey")
	if err != nil {
		t.Fatalf("replay: %v\n%s", err, out)
	}
	if !strings.Contains(out, "0 file(s) with deterministic edits") {
		t.Errorf("fixture: the replay should plan no edits:\n%s", out)
	}
	if strings.Contains(out, "resolved:") {
		t.Errorf("a no-op replay resolves nothing, so it prints no resolved line:\n%s", out)
	}
}
