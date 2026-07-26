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

// gitRefreshBundle builds a temp git bundle (via `bundle init` so log.md/index.md
// exist) holding one conformant node whose frontmatter modified predates its git
// last-commit date, so `node refresh` has honest drift to fix. Returns the dir
// and the node's bundle-relative path.
func gitRefreshBundle(t *testing.T) (dir, rel string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
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

	rel = "wine/tannin.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	node := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\n---\n\n# Tannin\n"
	if err := os.WriteFile(abs, []byte(node), 0o644); err != nil {
		t.Fatal(err)
	}
	iso := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	run(nil, "add", "-A")
	run([]string{"GIT_AUTHOR_DATE=" + iso, "GIT_COMMITTER_DATE=" + iso}, "commit", "-q", "-m", "add tannin")
	return dir, rel
}

// `node refresh <bundle>` rewrites modified to the git last-commit day for every
// drifting node, leaves created untouched, and exits 0.
func TestNodeRefresh_FixesDriftExitsZero(t *testing.T) {
	dir, rel := gitRefreshBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))

	out, err := runOKF(t, "node", "refresh", dir)
	if err != nil {
		t.Fatalf("refresh must exit 0 on found-and-fixed drift; err=%v out=%q", err, out)
	}
	got := readFileStr(t, abs)
	if !strings.Contains(got, "modified: 2026-07-20T00:00:00Z") {
		t.Fatalf("modified not refreshed to git day:\n%s", got)
	}
	if !strings.Contains(got, "created: 2026-01-01T00:00:00Z") {
		t.Fatalf("created must be immutable:\n%s", got)
	}
	// The node no longer drifts.
	if vout, verr := runOKF(t, "validate", dir); verr != nil {
		t.Fatalf("validate must pass after refresh; err=%v out=%q", verr, vout)
	} else if strings.Contains(vout, "disagrees") {
		t.Fatalf("drift warning remains after refresh:\n%s", vout)
	}
	// The change is recorded in log.md.
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, rel) {
		t.Fatalf("log.md should record the refresh; got:\n%s", log)
	}
}

// --dry-run lists what would change, exits 0, and writes NOTHING.
func TestNodeRefresh_DryRunListsAndWritesNothing(t *testing.T) {
	dir, rel := gitRefreshBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	before := readFileStr(t, abs)

	out, err := runOKF(t, "node", "refresh", "--dry-run", dir)
	if err != nil {
		t.Fatalf("dry-run must exit 0; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, rel) {
		t.Fatalf("dry-run should list the drifting node; got:\n%s", out)
	}
	if !strings.Contains(out, "2026-07-01") || !strings.Contains(out, "2026-07-20") {
		t.Fatalf("dry-run should show old->new dates; got:\n%s", out)
	}
	// File is untouched.
	if after := readFileStr(t, abs); after != before {
		t.Fatalf("dry-run wrote to disk:\nBEFORE %q\nAFTER %q", before, after)
	}
}

// The single-node form refreshes only the named node.
func TestNodeRefresh_SingleNode(t *testing.T) {
	dir, rel := gitRefreshBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))

	out, err := runOKF(t, "node", "refresh", dir, rel)
	if err != nil {
		t.Fatalf("single-node refresh must exit 0; err=%v out=%q", err, out)
	}
	got := readFileStr(t, abs)
	if !strings.Contains(got, "modified: 2026-07-20T00:00:00Z") {
		t.Fatalf("single-node modified not refreshed:\n%s", got)
	}
}

// The single-node form accepts a path without the .md extension.
func TestNodeRefresh_SingleNodeNoExt(t *testing.T) {
	dir, rel := gitRefreshBundle(t)
	abs := filepath.Join(dir, filepath.FromSlash(rel))

	noExt := strings.TrimSuffix(rel, ".md")
	if _, err := runOKF(t, "node", "refresh", dir, noExt); err != nil {
		t.Fatalf("refresh with no .md ext must work; err=%v", err)
	}
	if !strings.Contains(readFileStr(t, abs), "modified: 2026-07-20T00:00:00Z") {
		t.Fatalf("no-ext single-node refresh did not touch the node")
	}
}

// Outside a git repo there is no drift and refresh is a clean no-op exiting 0.
func TestNodeRefresh_NoGitNoOpExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	abs := filepath.Join(dir, "wine", "x.md")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := "---\ntype: Concept\ntitle: X\ncreated: 2026-01-01T00:00:00Z\nmodified: 2020-01-01T00:00:00Z\n---\n\n# X\n"
	if err := os.WriteFile(abs, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "node", "refresh", dir)
	if err != nil {
		t.Fatalf("no-git refresh must exit 0; err=%v out=%q", err, out)
	}
	if after := readFileStr(t, abs); after != orig {
		t.Fatalf("no-git refresh wrote to disk:\n%s", after)
	}
}

// Refreshing when nothing drifts exits 0 and writes nothing.
func TestNodeRefresh_NothingToDoExitsZero(t *testing.T) {
	dir, _ := gitRefreshBundle(t)
	// First refresh fixes the one drifting node.
	if _, err := runOKF(t, "node", "refresh", dir); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	// Second refresh has nothing to do.
	out, err := runOKF(t, "node", "refresh", dir)
	if err != nil {
		t.Fatalf("no-drift refresh must exit 0; err=%v out=%q", err, out)
	}
}

// A single-node path not present in the bundle is a real error (nonzero exit).
func TestNodeRefresh_UnknownNodeErrors(t *testing.T) {
	dir, _ := gitRefreshBundle(t)
	if _, err := runOKF(t, "node", "refresh", dir, "wine/nope.md"); err == nil {
		t.Fatalf("unknown node path must error")
	}
}
