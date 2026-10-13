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

// A fresh clone is a checkout of its own source (positive control), including
// when the source is spelled with the other path separator.
func TestIsCheckoutOf_TrueForCloneOfSource(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "seed\n")
	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	for _, src := range []string{bare, filepath.ToSlash(bare), bare + string(filepath.Separator)} {
		if !IsCheckoutOf(dst, src) {
			t.Errorf("IsCheckoutOf(clone, %q) = false, want true", src)
		}
	}
}

// A subdirectory of a checkout is inside a work tree but is not itself a
// checkout: fast-forwarding it would pull the enclosing repo.
func TestIsCheckoutOf_FalseForSubdirOfCheckout(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "seed\n")
	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	sub := filepath.Join(dst, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsGitWorkTree(sub) {
		t.Fatalf("precondition: subdir should be inside the work tree")
	}
	if IsCheckoutOf(sub, bare) {
		t.Fatalf("IsCheckoutOf(subdir of checkout, its origin) = true, want false")
	}
}

// A checkout of a different origin is a work tree top level but not a
// checkout of this source.
func TestIsCheckoutOf_FalseForDifferentOrigin(t *testing.T) {
	root := t.TempDir()
	bare := seedBareSource(t, root, "index.md", "seed\n")
	dst := filepath.Join(root, "cloned")
	if err := Clone(bare, dst); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	other := filepath.Join(root, "other.git")
	if IsCheckoutOf(dst, other) {
		t.Fatalf("IsCheckoutOf(checkout of src.git, other.git) = true, want false")
	}
	if IsCheckoutOf(dst, "https://example.com/acme/kb.git") {
		t.Fatalf("IsCheckoutOf(local checkout, remote URL) = true, want false")
	}
}

// A repo with no origin remote is not a checkout of any source.
func TestIsCheckoutOf_FalseWithoutOrigin(t *testing.T) {
	if !GitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	if IsCheckoutOf(dir, dir) {
		t.Fatalf("IsCheckoutOf(repo without origin) = true, want false")
	}
	if IsCheckoutOf(filepath.Join(dir, "missing"), dir) {
		t.Fatalf("IsCheckoutOf(missing dir) = true, want false")
	}
}

func TestIsLocalSource(t *testing.T) {
	cases := map[string]bool{
		"/srv/kb.git":                true,
		"./kb":                       true,
		"../kb":                      true,
		"kb":                         true,
		`C:\repos\docs`:              true,
		"C:/repos/docs":              true,
		`\\server\share\repo`:        true,
		"file:///srv/kb.git":         true,
		"./dir:with-colon":           true,
		"https://github.com/acme/kb": false,
		"ssh://git@host/acme/kb.git": false,
		"git@github.com:acme/kb.git": false,
		"host:acme/kb.git":           false,
	}
	for s, want := range cases {
		if got := IsLocalSource(s); got != want {
			t.Errorf("IsLocalSource(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestNormalizeSourceURL_RemoteEquivalences(t *testing.T) {
	same := [][2]string{
		{"https://github.com/acme/kb.git", "https://github.com/acme/kb"},
		{"https://github.com/acme/kb/", "HTTPS://GitHub.com/acme/kb.git"},
		{"git@github.com:acme/kb.git", "git@GITHUB.com:acme/kb"},
	}
	for _, p := range same {
		if a, b := NormalizeSourceURL(p[0], ""), NormalizeSourceURL(p[1], ""); a != b {
			t.Errorf("NormalizeSourceURL(%q)=%q != NormalizeSourceURL(%q)=%q", p[0], a, p[1], b)
		}
	}
	// Negative control: the path stays case-sensitive and distinct repos stay
	// distinct.
	differ := [][2]string{
		{"https://github.com/acme/kb", "https://github.com/acme/KB"},
		{"https://github.com/acme/kb", "https://github.com/acme/other"},
		{"https://github.com/acme/kb", "git@github.com:acme/kb"},
	}
	for _, p := range differ {
		if a, b := NormalizeSourceURL(p[0], ""), NormalizeSourceURL(p[1], ""); a == b {
			t.Errorf("NormalizeSourceURL(%q) == NormalizeSourceURL(%q) = %q, want distinct", p[0], p[1], a)
		}
	}
}

func TestHasWindowsPathPrefix(t *testing.T) {
	cases := map[string]bool{
		`C:\repos\docs`:       true,
		"c:/repos/docs":       true,
		`\\server\share\repo`: true,
		`\single\backslash`:   false,
		"C:":                  false,
		"C:repos":             false,
		"knowledge":           false,
		"/srv/kb":             false,
		"git@host:kb":         false,
		"1:/x":                false,
	}
	for s, want := range cases {
		if got := HasWindowsPathPrefix(s); got != want {
			t.Errorf("HasWindowsPathPrefix(%q) = %v, want %v", s, got, want)
		}
	}
}
