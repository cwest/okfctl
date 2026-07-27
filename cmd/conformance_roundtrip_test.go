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

// Cold-start conformance round trip driven through the REAL CLI entry points:
// the whole reserved-file surface must chain from an empty dir with exit 0 at
// every step, and okfctl's own output must validate clean. This proves the
// closed loop (produce → validate) at the command boundary, not just in the
// core model — a spec-governed producer that never runs its own validator on
// its own output is how the parent index defect stayed invisible.

func TestConformance_ColdStartRoundTrip_ExitsZeroThroughout(t *testing.T) {
	dir := t.TempDir()

	steps := [][]string{
		{"bundle", "init", dir},
		{"node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir},
		{"node", "new", "lifting/squat.md", "--type", "Playbook", "--title", "Squat", "--bundle", dir},
		{"index", "build", dir},
		{"log", "append", dir, "--message", "Seeded the knowledge base."},
		{"validate", dir},
		{"index", "check", dir},
	}
	for _, args := range steps {
		if out, err := runOKF(t, args...); err != nil {
			t.Fatalf("`okfctl %s` must exit 0; err=%v out=%q", strings.Join(args, " "), err, out)
		}
	}

	// validate emitted the clean-floor banner (defensive: exit 0 already
	// asserted above, but confirm it did not silently pass a drift-only path).
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("validate must exit 0 on the generated bundle; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "OK: bundle conforms to the OKF spec floor") {
		t.Errorf("validate must report the clean-floor banner; got:\n%s", out)
	}
}

// A regressed index — the pre-fix `type: Index` shape written by hand to
// simulate a generator that violates §6/§11 — must be FLAGGED by validate
// (nonzero exit). This is the guard the parent defect lacked at the CLI seam:
// before the validator learned §6/§11, this bundle passed.
func TestConformance_ValidateFlagsSpecViolatingIndex_CLI(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// Overwrite the index with the non-conformant shape, bypassing okfctl.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntype: Index\n---\n\n# Knowledge Base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "validate", dir)
	if err == nil {
		t.Fatalf("validate must exit nonzero on an index carrying non-okf_version frontmatter; out=%q", out)
	}
	if !strings.Contains(out, "index.md") {
		t.Errorf("validate output must name the offending index.md; got:\n%s", out)
	}
}
