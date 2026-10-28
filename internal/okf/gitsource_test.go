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

package okf

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRun runs a git command in dir, failing the test on error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// seedBareSource builds a bare git repo at <root>/src.git with one committed
// file and returns its path. It stages the commit in a scratch work tree, then
// pushes into the bare repo, so the bare repo is a valid clone source with a
// default branch. Skips the test if git is unavailable.
func seedBareSource(t *testing.T, root, filename, content string) string {
	t.Helper()
	if !GitAvailable() {
		t.Skip("git not available")
	}
	bare := filepath.Join(root, "src.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, bare, "init", "--bare", "-q", "-b", "main")

	work := filepath.Join(root, "seed-work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "init", "-q", "-b", "main")
	gitRun(t, work, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(work, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "seed")
	gitRun(t, work, "remote", "add", "origin", bare)
	gitRun(t, work, "push", "-q", "origin", "main")
	return bare
}

func TestClone_FromLocalBareRepo(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "seed\n")

	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "index.md"))
	if err != nil {
		t.Fatalf("cloned file missing: %v", err)
	}
	if string(got) != "seed\n" {
		t.Fatalf("cloned content = %q, want %q", got, "seed\n")
	}
	if !IsGitWorkTree(dst) {
		t.Fatalf("clone dst is not reported as a git work tree")
	}
}

func TestPullFastForward_PicksUpNewCommit(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "v1\n")

	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Advance the source with a second commit via a fresh work tree.
	work := filepath.Join(root, "advance")
	gitRun(t, root, "clone", "-q", bare, work) // clone the source into a scratch work tree
	gitRun(t, work, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(work, "index.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "commit", "-aqm", "v2")
	gitRun(t, work, "push", "-q", "origin", "HEAD:main")

	if err := PullFastForward(dst); err != nil {
		t.Fatalf("PullFastForward: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2\n" {
		t.Fatalf("after ff pull content = %q, want %q", got, "v2\n")
	}
}

func TestIsGitWorkTree_FalseOutsideRepo(t *testing.T) {
	if !GitAvailable() {
		t.Skip("git not available")
	}
	if IsGitWorkTree(t.TempDir()) {
		t.Fatalf("IsGitWorkTree true for a non-repo dir")
	}
}

// TestIsCheckoutOf_GenuineSameSource is the positive control: a clone of the
// source, at its own top level, IS reported as a checkout of that source.
func TestIsCheckoutOf_GenuineSameSource(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "v1\n")

	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	ok, err := IsCheckoutOf(dst, bare)
	if err != nil {
		t.Fatalf("IsCheckoutOf: %v", err)
	}
	if !ok {
		t.Fatalf("IsCheckoutOf(genuine same-source clone) = false, want true")
	}
}

// TestIsCheckoutOf_SubdirOfRepoNotQualified is the serious-bug control: a
// SUBDIRECTORY of a checkout is inside a work tree but is NOT the checkout's
// top level, so it must NOT qualify — otherwise connect fast-forwards an
// enclosing repo the user never named.
func TestIsCheckoutOf_SubdirOfRepoNotQualified(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "v1\n")

	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	sub := filepath.Join(dst, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Even querying against the SAME source URL, a subdir is not the top level.
	ok, err := IsCheckoutOf(sub, bare)
	if err != nil {
		t.Fatalf("IsCheckoutOf: %v", err)
	}
	if ok {
		t.Fatalf("IsCheckoutOf(subdir of a checkout) = true, want false")
	}
}

// TestIsCheckoutOf_DifferentOriginNotQualified is the other half of the bug: a
// checkout whose origin is a DIFFERENT source must not qualify, so connect does
// not fast-forward an unrelated repo that merely occupies the destination.
func TestIsCheckoutOf_DifferentOriginNotQualified(t *testing.T) {
	root := t.TempDir()
	srcA := seedBareSource(t, mustMkdir(t, root, "a"), "index.md", "a\n")
	srcB := seedBareSource(t, mustMkdir(t, root, "b"), "index.md", "b\n")

	dst := filepath.Join(root, "cloned-a")
	if err := Clone(srcA, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	// dst is a genuine top-level checkout, but of srcA — querying srcB must fail.
	ok, err := IsCheckoutOf(dst, srcB)
	if err != nil {
		t.Fatalf("IsCheckoutOf: %v", err)
	}
	if ok {
		t.Fatalf("IsCheckoutOf(checkout of a DIFFERENT origin) = true, want false")
	}
}

// TestIsCheckoutOf_NonRepoDir: a plain directory is not a checkout of anything.
func TestIsCheckoutOf_NonRepoDir(t *testing.T) {
	if !GitAvailable() {
		t.Skip("git not available")
	}
	ok, err := IsCheckoutOf(t.TempDir(), "https://example.com/x.git")
	if err != nil {
		t.Fatalf("IsCheckoutOf: %v", err)
	}
	if ok {
		t.Fatalf("IsCheckoutOf(non-repo dir) = true, want false")
	}
}

func mustMkdir(t *testing.T, root, name string) string {
	t.Helper()
	d := filepath.Join(root, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSameGitURL(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", "https://h/o/r.git", "https://h/o/r.git", true},
		{"trailing .git on one", "https://h/o/r", "https://h/o/r.git", true},
		{"trailing slash", "https://h/o/r.git/", "https://h/o/r.git", true},
		{"slash and .git mixed", "https://h/o/r/", "https://h/o/r.git", true},
		{"different repo", "https://h/o/r.git", "https://h/o/other.git", false},
		{"scp-like equal", "git@h:o/r.git", "git@h:o/r", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameGitURL(tc.a, tc.b); got != tc.want {
				t.Fatalf("sameGitURL(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
