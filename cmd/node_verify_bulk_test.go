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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// verifyMultiBundle builds a bundle (via bundle init) with n conformant nodes,
// none carrying a verified key. Returns the dir and the node rel paths.
func verifyMultiBundle(t *testing.T, n int) (dir string, rels []string) {
	t.Helper()
	dir = t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("wine/n%02d.md", i)
		rels = append(rels, rel)
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("---\ntype: Concept\ntitle: N%02d\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\n---\n\n# N%02d\n", i, i)
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, rels
}

// Bulk mode (no path) is DRY-RUN by default: it prints a plan and writes NOTHING
// without the write flag. The plan names each node that would be stamped.
func TestNodeVerify_Bulk_DryRunByDefault(t *testing.T) {
	dir, rels := verifyMultiBundle(t, 3)
	snapshot := map[string]string{}
	for _, rel := range rels {
		snapshot[rel] = readFileStr(t, filepath.Join(dir, filepath.FromSlash(rel)))
	}

	out, err := runOKF(t, "node", "verify", dir, "--by", "human:casey", "--all")
	if err != nil {
		t.Fatalf("bulk dry-run (default) must exit 0; err=%v out=%q", err, out)
	}
	for _, rel := range rels {
		if !strings.Contains(out, rel) {
			t.Fatalf("plan should list %s; got:\n%s", rel, out)
		}
		if after := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rel))); after != snapshot[rel] {
			t.Fatalf("dry-run wrote to %s", rel)
		}
	}
}

// Whole-bundle stamping is REFUSED without an explicit override (--all): the
// refusal exits non-zero, writes nothing, and explains that a bulk rubber-stamp
// converts a trust signal into noise.
func TestNodeVerify_Bulk_RefusedWithoutOverride(t *testing.T) {
	dir, rels := verifyMultiBundle(t, 3)
	before := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rels[0])))

	out, err := runOKF(t, "node", "verify", dir, "--by", "human:casey")
	if err == nil {
		t.Fatalf("whole-bundle verify must be refused without --all; out=%q", out)
	}
	msg := err.Error() + out
	low := strings.ToLower(msg)
	if !strings.Contains(low, "--all") {
		t.Fatalf("refusal must name the --all override; got:\n%s", msg)
	}
	if !strings.Contains(low, "noise") && !strings.Contains(low, "rubber") {
		t.Fatalf("refusal must explain the trust-into-noise reasoning; got:\n%s", msg)
	}
	if after := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rels[0]))); after != before {
		t.Fatalf("refused bulk verify must not write to disk")
	}
}

// --all --write actually stamps every node and records each in log.md.
func TestNodeVerify_Bulk_WriteStampsAll(t *testing.T) {
	pinVerifyClock(t, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	dir, rels := verifyMultiBundle(t, 3)

	out, err := runOKF(t, "node", "verify", dir, "--by", "human:casey", "--all", "--write")
	if err != nil {
		t.Fatalf("--all --write must exit 0; err=%v out=%q", err, out)
	}
	for _, rel := range rels {
		got := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rel)))
		if !strings.Contains(got, "by: human:casey") || !strings.Contains(got, "at: 2026-08-20T12:00:00Z") {
			t.Fatalf("node %s not stamped:\n%s", rel, got)
		}
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	for _, rel := range rels {
		if !strings.Contains(log, rel) {
			t.Fatalf("log.md should record %s; got:\n%s", rel, log)
		}
	}
}

// Bulk mode reports its SKIPS: a reserved file and an already-verified node are
// not stamped and the plan says so. (index.md/log.md are reserved; a node that
// already carries a verified entry is skipped to avoid a duplicate rubber-stamp.)
func TestNodeVerify_Bulk_ReportsSkips(t *testing.T) {
	pinVerifyClock(t, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	dir, rels := verifyMultiBundle(t, 2)
	// Pre-verify the first node so bulk mode must skip it.
	if _, err := runOKF(t, "node", "verify", dir, rels[0], "--by", "human:prior"); err != nil {
		t.Fatalf("pre-verify: %v", err)
	}

	out, err := runOKF(t, "node", "verify", dir, "--by", "human:casey", "--all")
	if err != nil {
		t.Fatalf("bulk dry-run: %v", err)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "skip") {
		t.Fatalf("bulk plan must report skips; got:\n%s", out)
	}
	if !strings.Contains(out, rels[0]) {
		t.Fatalf("plan should name the skipped already-verified node %s; got:\n%s", rels[0], out)
	}
}

// --write without --all on the whole bundle is still refused: --write is the
// intent-to-write flag, --all is the whole-corpus override; both are required to
// stamp the entire bundle.
func TestNodeVerify_Bulk_WriteWithoutAllRefused(t *testing.T) {
	dir, rels := verifyMultiBundle(t, 2)
	before := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rels[0])))

	out, err := runOKF(t, "node", "verify", dir, "--by", "human:casey", "--write")
	if err == nil {
		t.Fatalf("--write without --all on the whole bundle must be refused; out=%q", out)
	}
	if after := readFileStr(t, filepath.Join(dir, filepath.FromSlash(rels[0]))); after != before {
		t.Fatalf("refused bulk verify must not write")
	}
}
