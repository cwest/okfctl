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
