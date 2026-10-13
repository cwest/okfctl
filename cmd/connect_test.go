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

	"github.com/cwest/okfctl/internal/okf"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// seedBareBundleSource builds a bare git repo containing a scaffolded (valid)
// OKF bundle and returns its path. Skips the test when git is unavailable.
func seedBareBundleSource(t *testing.T, root string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	bare := filepath.Join(root, "src.git")
	git(t, root, "init", "--bare", "-q", "-b", "main", bare)

	work := filepath.Join(root, "seed-work")
	if err := okf.Scaffold(work); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	git(t, work, "init", "-q", "-b", "main")
	git(t, work, "config", "commit.gpgsign", "false")
	git(t, work, "add", "-A")
	git(t, work, "commit", "-qm", "seed bundle")
	git(t, work, "remote", "add", "origin", bare)
	git(t, work, "push", "-q", "origin", "main")
	return bare
}

func TestConnect_ClonesRegisteredSource(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	if _, err := runOKF(t, "registry", "add", "kb", bare); err != nil {
		t.Fatalf("registry add: %v", err)
	}

	dst := filepath.Join(root, "local-kb")
	if _, err := runOKF(t, "connect", "kb", dst); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "index.md")); err != nil {
		t.Fatalf("connected bundle missing index.md: %v", err)
	}
	if _, err := runOKF(t, "validate", dst); err != nil {
		t.Fatalf("connected bundle failed validate: %v", err)
	}
}

func TestConnect_AdHocURL(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	dst := filepath.Join(root, "adhoc")
	if _, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("connect ad-hoc url: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "index.md")); err != nil {
		t.Fatalf("ad-hoc connect missing index.md: %v", err)
	}
}

func TestConnect_SecondRunFastForwards(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	dst := filepath.Join(root, "kb")
	if _, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("first connect: %v", err)
	}

	// Advance the source with a new node via a scratch clone.
	adv := filepath.Join(root, "adv")
	git(t, root, "clone", "-q", bare, adv)
	git(t, adv, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(adv, "new.md"), []byte("---\ntype: Concept\n---\n\n# New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, adv, "add", "-A")
	git(t, adv, "commit", "-qm", "add new node")
	git(t, adv, "push", "-q", "origin", "main")

	// Second connect fast-forwards the existing checkout.
	if _, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("second connect (ff): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "new.md")); err != nil {
		t.Fatalf("fast-forward did not pull new node: %v", err)
	}
}

func TestConnect_RefusesNonRepoNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	dst := filepath.Join(root, "occupied")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "connect", bare, dst); err == nil {
		t.Fatalf("connect into a non-repo non-empty dir must error")
	}
}

func TestConnect_UnknownNameAndNoSlashErrors(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	// An argument that is neither a registered name nor a plausible git URL
	// (no scheme, no ://, no host:path) is a mistake, not an ad-hoc URL.
	if _, err := runOKF(t, "connect", "definitely-not-registered", filepath.Join(t.TempDir(), "d")); err == nil {
		t.Fatalf("connect with an unknown bareword source must error")
	}
}

// headOf returns the commit HEAD points at in dir.
func headOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD in %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// advanceSource pushes one more commit onto bare via a scratch clone, so a
// checkout of bare has something a fast-forward would pull.
func advanceSource(t *testing.T, root, bare, name string) {
	t.Helper()
	adv := filepath.Join(root, "adv-"+name)
	git(t, root, "clone", "-q", bare, adv)
	git(t, adv, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(adv, name+".md"), []byte("---\ntype: Concept\n---\n\n# "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, adv, "add", "-A")
	git(t, adv, "commit", "-qm", "advance "+name)
	git(t, adv, "push", "-q", "origin", "main")
}

// otherRepoWithPendingUpdate clones a second, unrelated source into
// <root>/other and then advances that source, so a stray `git pull` in the
// checkout would move its HEAD. It returns the checkout and its HEAD.
func otherRepoWithPendingUpdate(t *testing.T, root string) (checkout, head string) {
	t.Helper()
	otherRoot := filepath.Join(root, "other-src")
	if err := os.MkdirAll(otherRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	otherBare := seedBareBundleSource(t, otherRoot)
	checkout = filepath.Join(root, "other")
	git(t, root, "clone", "-q", otherBare, checkout)
	advanceSource(t, otherRoot, otherBare, "pending")
	return checkout, headOf(t, checkout)
}

// Negative control for the wrong-repo pull: a non-empty subdirectory of an
// unrelated checkout is inside a work tree, but it is not a checkout of this
// source. connect must refuse and leave the enclosing repo untouched.
func TestConnect_RefusesSubdirOfAnotherRepo(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	other, before := otherRepoWithPendingUpdate(t, root)

	sub := filepath.Join(other, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runOKF(t, "connect", bare, sub)
	if err == nil || !strings.Contains(err.Error(), "not a checkout of this source") {
		t.Fatalf("connect into a subdir of another repo: err = %v, want the not-a-checkout refusal", err)
	}
	if after := headOf(t, other); after != before {
		t.Fatalf("enclosing repo was pulled: HEAD %s -> %s", before, after)
	}
	if _, err := os.Stat(filepath.Join(other, "pending.md")); err == nil {
		t.Fatalf("enclosing repo picked up the other source's pending commit")
	}
}

// Negative control for the wrong-repo pull: a checkout of a different origin
// is a work tree top level, but not of this source. connect must refuse and
// leave it untouched.
func TestConnect_RefusesCheckoutOfDifferentOrigin(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	other, before := otherRepoWithPendingUpdate(t, root)

	_, err := runOKF(t, "connect", bare, other)
	if err == nil || !strings.Contains(err.Error(), "not a checkout of this source") {
		t.Fatalf("connect into a checkout of a different origin: err = %v, want the not-a-checkout refusal", err)
	}
	if after := headOf(t, other); after != before {
		t.Fatalf("different-origin checkout was pulled: HEAD %s -> %s", before, after)
	}
}

// Positive control: the same source spelled differently (other path separator,
// trailing separator) still matches the checkout's origin and fast-forwards.
func TestConnect_SameSourceDifferentSpellingFastForwards(t *testing.T) {
	root := t.TempDir()
	bare := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	dst := filepath.Join(root, "kb")
	if _, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	advanceSource(t, root, bare, "fresh")

	out, err := runOKF(t, "connect", filepath.ToSlash(bare)+"/", dst)
	if err != nil {
		t.Fatalf("second connect with respelled source: %v", err)
	}
	if !strings.Contains(out, "Updated") {
		t.Fatalf("second connect did not report an update: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dst, "fresh.md")); err != nil {
		t.Fatalf("fast-forward did not pull the new node: %v", err)
	}
}

func TestLooksLikeGitURL(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/acme/kb.git": true,
		"git@github.com:acme/kb.git":     true,
		"file:///srv/kb.git":             true,
		"/srv/kb.git":                    true,
		"./kb":                           true,
		"~/kb":                           true,
		`C:\repos\docs`:                  true,
		"C:/repos/docs":                  true,
		`\\server\share\repo`:            true,
		// Negative control: a bare word is a mistyped remote name, not a URL.
		"knowledge": false,
		"C:repos":   false,
	}
	for s, want := range cases {
		if got := looksLikeGitURL(s); got != want {
			t.Errorf("looksLikeGitURL(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestDefaultDirForURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/kb.git": "kb",
		"git@github.com:acme/kb.git":     "kb",
		"/srv/docs.git":                  "docs",
		`C:\repos\docs`:                  "docs",
		`C:\repos\docs\`:                 "docs",
		"C:/repos/docs.git":              "docs",
		`\\server\share\repo`:            "repo",
	}
	for in, want := range cases {
		if got := defaultDirForURL(in); got != want {
			t.Errorf("defaultDirForURL(%q) = %q, want %q", in, got, want)
		}
	}
}
