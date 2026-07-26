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
	"time"
)

// initRepo makes dir a git repo with a deterministic identity so commits carry
// a known author/committer date. Skips the test if git is unavailable.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// commitAt stages everything and commits with both author and committer date
// pinned to `at`, so GitLastCommitDate has a deterministic value to read.
func commitAt(t *testing.T, dir, msg string, at time.Time) {
	t.Helper()
	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	iso := at.Format(time.RFC3339)
	c := exec.Command("git", "commit", "-q", "-m", msg)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+iso,
		"GIT_COMMITTER_DATE="+iso,
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestGitLastCommitDate(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	at := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: Concept\n---\n\n# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "add a", at)

	got, ok, err := GitLastCommitDate(root, "a.md")
	if err != nil {
		t.Fatalf("GitLastCommitDate err: %v", err)
	}
	if !ok {
		t.Fatalf("ok=false for a committed file")
	}
	if !got.UTC().Equal(at) {
		t.Fatalf("date = %v, want %v", got.UTC(), at)
	}
}

func TestGitLastCommitDateUntracked(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "u.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := GitLastCommitDate(root, "u.md")
	if err != nil {
		t.Fatalf("untracked file must not error, got %v", err)
	}
	if ok {
		t.Fatalf("ok=true for an untracked file; want false")
	}
}

func TestGitLastCommitDateNotARepo(t *testing.T) {
	root := t.TempDir() // no git init
	if err := os.WriteFile(filepath.Join(root, "n.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := GitLastCommitDate(root, "n.md")
	if err != nil {
		t.Fatalf("non-repo must not error, got %v", err)
	}
	if ok {
		t.Fatalf("ok=true outside a git repo; want false")
	}
}
