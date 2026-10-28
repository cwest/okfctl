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

// TestLooksLikeGitURL covers schemes, scp-like syntax, local paths, Windows
// drive-letter and UNC paths (accepted), and barewords / bare drive letters
// (rejected). Windows paths git-clones, so connect must treat them as URLs
// (issue #193, bug 2).
func TestLooksLikeGitURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// Schemes and scp-like syntax (unchanged behavior).
		{"https://github.com/acme/kb.git", true},
		{"git://host/repo.git", true},
		{"ssh://git@host/repo.git", true},
		{"file:///srv/repo.git", true},
		{"git@github.com:owner/repo.git", true},
		// POSIX local paths (unchanged).
		{"/srv/repo", true},
		{"./kb", true},
		{"~/repo", true},
		// Windows drive-letter paths (new): both separators.
		{`C:\repos\docs`, true},
		{"C:/repos/docs", true},
		{`D:\a`, true},
		{"Z:/a", true},
		// Windows UNC paths (new).
		{`\\server\share\repo`, true},
		{`\\srv\share\r`, true},
		// Rejections — a bare word is still a mistyped remote name, not a URL.
		{"knowledge", false},
		{"foo", false},
		{"", false},
		// A bare drive letter with nothing after the colon is not a path.
		{"C:", false},
		// A single backslash is not a UNC path.
		{`\only-one`, false},
	}
	for _, tc := range cases {
		if got := looksLikeGitURL(tc.in); got != tc.want {
			t.Errorf("looksLikeGitURL(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestConnect_SubdirOfAnotherRepoLeftUntouched is the serious-bug control from
// issue #193: connecting a source INTO a subdirectory of an unrelated repo must
// refuse (clear error, non-zero exit) and must NOT fast-forward the enclosing
// repo. We prove the enclosing repo's HEAD is unchanged.
func TestConnect_SubdirOfAnotherRepoLeftUntouched(t *testing.T) {
	root := t.TempDir()
	src := seedBareBundleSource(t, root)
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	// An unrelated "other" repo with a committed subdir.
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, other, "init", "-q", "-b", "main")
	git(t, other, "config", "commit.gpgsign", "false")
	sub := filepath.Join(other, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, other, "add", "-A")
	git(t, other, "commit", "-qm", "seed other")
	before := gitHEAD(t, other)

	// Connecting the source into other/sub must be refused, not fast-forwarded.
	_, err := runOKF(t, "connect", src, sub)
	if err == nil {
		t.Fatalf("connect into a subdir of another repo must error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected a refuse-to-overwrite error, got: %v", err)
	}
	if after := gitHEAD(t, other); after != before {
		t.Fatalf("enclosing repo HEAD moved: before=%s after=%s", before, after)
	}
}

// TestConnect_DifferentOriginCheckoutLeftUntouched: a destination that is a
// genuine top-level checkout but of a DIFFERENT origin must be refused, not
// fast-forwarded (issue #193, bug 1, second half).
func TestConnect_DifferentOriginCheckoutLeftUntouched(t *testing.T) {
	root := t.TempDir()
	srcA := seedBareBundleSource(t, mustMkdirCmd(t, root, "a"))
	srcB := seedBareBundleSource(t, mustMkdirCmd(t, root, "b"))
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	// Clone srcA into dst, then try to connect srcB into it.
	dst := filepath.Join(root, "checkout-a")
	if _, err := runOKF(t, "connect", srcA, dst); err != nil {
		t.Fatalf("seed connect of srcA: %v", err)
	}
	before := gitHEAD(t, dst)

	_, err := runOKF(t, "connect", srcB, dst)
	if err == nil {
		t.Fatalf("connect of a different origin into an existing checkout must error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected a refuse-to-overwrite error, got: %v", err)
	}
	if after := gitHEAD(t, dst); after != before {
		t.Fatalf("existing checkout HEAD moved: before=%s after=%s", before, after)
	}
}

// TestConnect_SameSourceStillFastForwards guards the kept behavior: a genuine
// same-source checkout is still fast-forwarded on a second connect.
func TestConnect_SameSourceStillFastForwards(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(adv, "ff.md"), []byte("---\ntype: Concept\n---\n\n# FF\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, adv, "add", "-A")
	git(t, adv, "commit", "-qm", "advance")
	git(t, adv, "push", "-q", "origin", "main")

	if _, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("second connect (same-source ff): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "ff.md")); err != nil {
		t.Fatalf("same-source fast-forward did not pull new node: %v", err)
	}
}

func mustMkdirCmd(t *testing.T, root, name string) string {
	t.Helper()
	d := filepath.Join(root, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func gitHEAD(t *testing.T, dir string) string {
	t.Helper()
	c := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse HEAD in %s: %v\n%s", dir, err, out)
	}
	return strings.TrimSpace(string(out))
}
