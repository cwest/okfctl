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
