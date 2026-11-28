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

// AC-A4: every node-mutation command records its change through AppendLog, so a
// bundle maintained entirely with node commands must end up with a §9-conformant
// log — and `validate` must pass it (the load-bearing invariant: validate never
// rejects a log okfctl wrote). This exercises the real command path (node new →
// maintainOnCreate → AppendLog), not AppendLog directly.
func TestNodeCommands_ProduceSection9Log(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}
	if _, err := runOKF(t, "node", "new", "wine/acidity.md", "--type", "Reference", "--title", "Acidity", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}

	// The log okfctl just wrote must be a §9 date-grouped log.
	raw, err := os.ReadFile(filepath.Join(dir, "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "## ") {
		t.Errorf("AC-A4: node commands must write a §9 `## <date>` heading; got:\n%s", body)
	}
	if strings.Contains(body, " — ") {
		t.Errorf("AC-A4: node commands must not write legacy inline-dated bullets; got:\n%s", body)
	}
	if !strings.Contains(body, "* created wine/tannin.md") || !strings.Contains(body, "* created wine/acidity.md") {
		t.Errorf("AC-A4: expected both `* created …` entries; got:\n%s", body)
	}

	// And validate must pass the whole bundle (exit 0).
	if out, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("AC-A4: validate must pass a node-maintained bundle; err=%v out=%q", err, out)
	}
}

// §9 end to end: a legacy `- YYYY-MM-DD — msg` bullet left inside a date group
// fails `validate --strict`; `log append` regroups it under its own date, after
// which `validate --strict` passes.
func TestValidateStrict_Section9_LegacyBulletInsideGroupThenLogAppend(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	seed := "# Change Log\n\n## 2026-11-27\n* **Update**: migrated.\n\n## 2026-11-26\n* **Sweep**: grouped.\n- 2026-11-27 — misplaced sweep\n"
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "validate", "--strict", dir)
	if err == nil {
		t.Fatalf("§9: validate --strict must fail on a legacy bullet inside a date group; out=%q", out)
	}
	if !strings.Contains(out, "FAIL log.md: log has legacy dated bullets not grouped under date headings (§9)") {
		t.Errorf("§9: expected the legacy-bullet finding; out=%q", out)
	}

	if _, err := runOKF(t, "log", "append", "--message", "regrouped", dir); err != nil {
		t.Fatalf("log append: %v", err)
	}
	if out, err := runOKF(t, "validate", "--strict", dir); err != nil {
		t.Fatalf("§9: validate --strict must pass after log append regroups; err=%v out=%q", err, out)
	}
}
