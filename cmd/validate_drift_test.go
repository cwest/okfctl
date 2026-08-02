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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitBundleWithStaleModified builds a temp git bundle holding one conformant
// node whose frontmatter modified predates its git last-commit date, so the
// drift check has something honest to catch.
func gitBundleWithStaleModified(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), env...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(nil, "init", "-q")
	run(nil, "config", "user.email", "t@example.com")
	run(nil, "config", "user.name", "T")
	run(nil, "config", "commit.gpgsign", "false")

	node := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\n---\n\n# Tannin\n"
	if err := os.WriteFile(filepath.Join(dir, "tannin.md"), []byte(node), 0o644); err != nil {
		t.Fatal(err)
	}
	iso := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	run(nil, "add", "-A")
	run([]string{"GIT_AUTHOR_DATE=" + iso, "GIT_COMMITTER_DATE=" + iso}, "commit", "-q", "-m", "add tannin")
	return dir
}

// validate reports git drift as an advisory warning and still exits 0 (drift is
// advisory; it never fails the spec floor).
func TestValidateCmd_ReportsGitDriftAdvisory(t *testing.T) {
	dir := gitBundleWithStaleModified(t)
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("drift alone must not fail validate; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "tannin.md") || !strings.Contains(strings.ToLower(out), "modified") {
		t.Fatalf("expected a drift warning naming the node; got %q", out)
	}
}

// --strict escalates drift to a nonzero exit.
func TestValidateCmd_StrictFailsOnGitDrift(t *testing.T) {
	dir := gitBundleWithStaleModified(t)
	out, err := runOKF(t, "validate", "--strict", dir)
	if err == nil {
		t.Fatalf("--strict must fail on drift; out=%q", out)
	}
}

// gitBundleBulkTouched authors a node on its real day, then a single bulk
// mechanical commit adds a frontmatter key. Without an ignore list the node
// drifts (frontmatter day != bulk-commit day). Returns dir and the bulk SHA.
func gitBundleBulkTouched(t *testing.T) (dir, bulkSHA string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	git := func(env []string, args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), env...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(nil, "init", "-q")
	git(nil, "config", "user.email", "t@example.com")
	git(nil, "config", "user.name", "T")
	git(nil, "config", "commit.gpgsign", "false")

	abs := filepath.Join(dir, "tannin.md")
	if err := os.WriteFile(abs, []byte("---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-06-15T00:00:00Z\n---\n\n# Tannin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	realISO := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	git(nil, "add", "-A")
	git([]string{"GIT_AUTHOR_DATE=" + realISO, "GIT_COMMITTER_DATE=" + realISO}, "commit", "-q", "-m", "author tannin")

	if err := os.WriteFile(abs, []byte("---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-06-15T00:00:00Z\nstatus: verified\n---\n\n# Tannin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bulkISO := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	git(nil, "add", "-A")
	git([]string{"GIT_AUTHOR_DATE=" + bulkISO, "GIT_COMMITTER_DATE=" + bulkISO}, "commit", "-q", "-m", "bulk: key sweep")
	shaOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(shaOut))
}

// validate reports drift for a node touched only by a bulk mechanical commit
// when that commit is NOT opted out — the positive control at the validate
// surface where the reporter saw 1,166 warnings.
func TestValidateCmd_BulkTouchedNodeDriftsWithoutIgnore(t *testing.T) {
	dir, _ := gitBundleBulkTouched(t)
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("drift alone must not fail validate; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "tannin.md") || !strings.Contains(out, "disagrees") {
		t.Fatalf("expected a drift warning for the bulk-touched node; got %q", out)
	}
}

// With the bulk commit listed in .okf-drift-ignore-revs, validate reports ZERO
// drift for the node it touched — the comparison walks back to the real
// authoring commit, which agrees with the frontmatter. The negative control at
// the validate surface.
func TestValidateCmd_IgnoreRevsSilencesBulkDrift(t *testing.T) {
	dir, bulkSHA := gitBundleBulkTouched(t)
	if err := os.WriteFile(filepath.Join(dir, ".okf-drift-ignore-revs"), []byte("# migration\n"+bulkSHA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("validate must still exit 0; err=%v out=%q", err, out)
	}
	if strings.Contains(out, "disagrees") {
		t.Fatalf("listed bulk commit must silence drift at validate; got:\n%s", out)
	}
	// Even under --strict there is nothing to fail on.
	if _, serr := runOKF(t, "validate", "--strict", dir); serr != nil {
		t.Fatalf("with drift opted out, --strict must pass; err=%v", serr)
	}
}
