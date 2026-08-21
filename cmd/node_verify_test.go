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

// pinVerifyClock pins the cmd-layer clock to a fixed instant for the test's
// lifetime, so a written `verified.at` is deterministic.
func pinVerifyClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := nowUTCcmd
	nowUTCcmd = func() time.Time { return at }
	t.Cleanup(func() { nowUTCcmd = prev })
}

// verifyBundle builds a temp bundle (via `bundle init` so log.md/index.md exist)
// with one conformant node carrying created/modified but no verified. Returns
// the bundle dir and the node's bundle-relative path.
func verifyBundle(t *testing.T) (dir, rel string) {
	t.Helper()
	dir = t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	rel = "wine/tannin.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	node := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\n---\n\n# Tannin\n"
	if err := os.WriteFile(abs, []byte(node), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, rel
}

// `node verify <bundle> <path> --by <actor>` appends a verified entry with the
// actor and an RFC3339 timestamp, leaving created/modified untouched, and exits
// 0. Reserved-file lifecycle runs, so log.md records the change.
func TestNodeVerify_SingleNode_AppendsAndLogs(t *testing.T) {
	pinVerifyClock(t, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	dir, rel := verifyBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))

	out, err := runOKF(t, "node", "verify", dir, rel, "--by", "human:casey")
	if err != nil {
		t.Fatalf("verify must exit 0; err=%v out=%q", err, out)
	}
	got := readFileStr(t, abs)
	if !strings.Contains(got, "verified:") || !strings.Contains(got, "by: human:casey") {
		t.Fatalf("verified entry not appended:\n%s", got)
	}
	if !strings.Contains(got, "at: 2026-08-20T12:00:00Z") {
		t.Fatalf("verified.at not stamped RFC3339:\n%s", got)
	}
	if !strings.Contains(got, "created: 2026-01-01T00:00:00Z") {
		t.Fatalf("created must be immutable:\n%s", got)
	}
	if !strings.Contains(got, "modified: 2026-07-01T00:00:00Z") {
		t.Fatalf("modified must not be touched by verify:\n%s", got)
	}
	// The change is recorded in log.md (reserved-file lifecycle).
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, rel) {
		t.Fatalf("log.md should record the verify; got:\n%s", log)
	}
}

// --by is REQUIRED: omitting it is an error and nothing is written. A tool that
// guesses at who is a tool that manufactures trust.
func TestNodeVerify_ByRequired(t *testing.T) {
	dir, rel := verifyBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	before := readFileStr(t, abs)

	out, err := runOKF(t, "node", "verify", dir, rel)
	if err == nil {
		t.Fatalf("omitting --by must error; out=%q", out)
	}
	if after := readFileStr(t, abs); after != before {
		t.Fatalf("no write may happen when --by is missing")
	}
}

// --by is validated against the §7 actor forms: a bare id (no human:/process:
// prefix and no producer/version slash) is rejected and nothing is written.
func TestNodeVerify_ByValidatedAgainstSection7(t *testing.T) {
	dir, rel := verifyBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	before := readFileStr(t, abs)

	for _, bad := range []string{"casey", "me", "human:", "process:"} {
		out, err := runOKF(t, "node", "verify", dir, rel, "--by", bad)
		if err == nil {
			t.Fatalf("--by %q must be rejected (not a §7 form); out=%q", bad, out)
		}
		if after := readFileStr(t, abs); after != before {
			t.Fatalf("--by %q rejected but the node was written", bad)
		}
	}
}

// The actor is never inferred from git config: even inside a git repo with a
// configured user.name/user.email, omitting --by errors rather than reading it.
func TestNodeVerify_NoGitConfigInference(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir, rel := verifyBundle(t)
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "casey@geeknest.com")
	git("config", "user.name", "Casey West")

	out, err := runOKF(t, "node", "verify", dir, rel)
	if err == nil {
		t.Fatalf("--by must not be inferred from git config; out=%q", out)
	}
}

// The single-node form accepts a path without the .md extension.
func TestNodeVerify_SingleNodeNoExt(t *testing.T) {
	pinVerifyClock(t, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	dir, rel := verifyBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	noExt := strings.TrimSuffix(rel, ".md")

	if _, err := runOKF(t, "node", "verify", dir, noExt, "--by", "human:casey"); err != nil {
		t.Fatalf("verify with no .md ext must work; err=%v", err)
	}
	if !strings.Contains(readFileStr(t, abs), "by: human:casey") {
		t.Fatalf("no-ext single-node verify did not stamp the node")
	}
}

// A single-node path not present in the bundle is a real error (nonzero exit).
func TestNodeVerify_UnknownNodeErrors(t *testing.T) {
	dir, _ := verifyBundle(t)
	if _, err := runOKF(t, "node", "verify", dir, "wine/nope.md", "--by", "human:casey"); err == nil {
		t.Fatalf("unknown node path must error")
	}
}

// A reserved file (index.md, log.md) is not a node and cannot be verified.
func TestNodeVerify_ReservedFileRefused(t *testing.T) {
	dir, _ := verifyBundle(t)
	if _, err := runOKF(t, "node", "verify", dir, "index.md", "--by", "human:casey"); err == nil {
		t.Fatalf("verifying a reserved file must error")
	}
}

// After a verify, `analyze` reports the stamped node as fresh via verified.at
// (the top of the freshness precedence), closing the loop.
func TestNodeVerify_AnalyzeReportsFresh(t *testing.T) {
	// Stamp verified "now" so the node is freshly verified regardless of an old
	// modified; analyze should then not flag it stale via the verified basis.
	pinVerifyClock(t, time.Now().UTC())
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	rel := "wine/aged.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	// modified is ancient (would be stale) — verifying now makes it fresh.
	node := "---\ntype: Concept\ntitle: Aged\ncreated: 2020-01-01T00:00:00Z\nmodified: 2020-01-01T00:00:00Z\n---\n\n# Aged\n"
	if err := os.WriteFile(abs, []byte(node), 0o644); err != nil {
		t.Fatal(err)
	}

	// Before verify: stale via the ancient modified.
	before, err := runOKF(t, "analyze", dir)
	if err != nil {
		t.Fatalf("analyze before: %v", err)
	}
	if !strings.Contains(before, rel) {
		t.Fatalf("expected the ancient node to be stale before verify; got:\n%s", before)
	}

	if _, err := runOKF(t, "node", "verify", dir, rel, "--by", "human:casey"); err != nil {
		t.Fatalf("verify: %v", err)
	}

	after, err := runOKF(t, "analyze", dir)
	if err != nil {
		t.Fatalf("analyze after: %v", err)
	}
	// The node is now fresh via verified.at — it should not appear in the stale
	// section anymore. (Grep the stale line region by the node path under the
	// freshness heading is overkill; the whole report simply should not list it
	// as stale.)
	if strings.Contains(afterStaleSection(after), rel) {
		t.Fatalf("after verify the node must be fresh (not stale); got:\n%s", after)
	}
}

// afterStaleSection returns the "Stale" portion of the analyze human report so a
// test can assert a node is/ isn't flagged stale without matching other sections
// that legitimately mention the path.
func afterStaleSection(report string) string {
	i := strings.Index(report, "Stale")
	if i < 0 {
		return ""
	}
	rest := report[i:]
	// Cut at the next "##" heading.
	if j := strings.Index(rest[2:], "\n## "); j >= 0 {
		return rest[:j+2]
	}
	return rest
}

// The long help text states plainly that the stamp asserts a human or named
// process actually checked the node.
func TestNodeVerify_LongHelpStatesSemantics(t *testing.T) {
	out, err := runOKF(t, "node", "verify", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "checked") && !strings.Contains(low, "confirm") {
		t.Fatalf("long help must state the stamp asserts an actual check; got:\n%s", out)
	}
	if !strings.Contains(low, "human") || !strings.Contains(low, "process") {
		t.Fatalf("long help must name the human/named-process actor semantics; got:\n%s", out)
	}
}
