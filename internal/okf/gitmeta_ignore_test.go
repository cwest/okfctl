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

// headSHA returns the current HEAD commit SHA of the repo at dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	c := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := c.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return string(out[:len(out)-1]) // strip trailing newline
}

// With no ignored revs, GitLastCommitDateIgnoring behaves exactly like
// GitLastCommitDate: it returns the most recent commit that touched the file,
// and reports that commit's SHA.
func TestGitLastCommitDateIgnoring_NoIgnoreReturnsHead(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	at := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: Concept\n---\n\n# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "add a", at)
	want := headSHA(t, root)

	got, sha, ok, err := GitLastCommitDateIgnoring(root, "a.md", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("ok=false for a committed file")
	}
	if !got.UTC().Equal(at) {
		t.Fatalf("date = %v, want %v", got.UTC(), at)
	}
	if sha != want {
		t.Fatalf("sha = %q, want %q", sha, want)
	}
}

// The core walk-back: when the file's LAST commit SHA is in the ignore set, the
// comparison falls through to the PRIOR commit that touched the file — its date
// and SHA are what get returned. This is what lets a bulk mechanical commit opt
// out of git drift without erasing the real authoring history.
func TestGitLastCommitDateIgnoring_WalksBackPastIgnored(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	// Real authoring commit.
	realAt := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: Concept\nmodified: 2026-06-15T00:00:00Z\n---\n\n# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "author a", realAt)
	realSHA := headSHA(t, root)

	// Bulk mechanical commit that touches the same file (adds a key).
	bulkAt := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: Concept\nmodified: 2026-06-15T00:00:00Z\nstatus: verified\n---\n\n# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "bulk: add status key", bulkAt)
	bulkSHA := headSHA(t, root)

	// Without the ignore set, the last commit (the bulk one) wins.
	d, sha, ok, err := GitLastCommitDateIgnoring(root, "a.md", nil)
	if err != nil || !ok {
		t.Fatalf("baseline: err=%v ok=%v", err, ok)
	}
	if sha != bulkSHA || !sameCalendarDay(d, bulkAt) {
		t.Fatalf("baseline should return bulk commit; got sha=%q date=%v", sha, d)
	}

	// With the bulk SHA ignored, we walk back to the real authoring commit.
	ignore := map[string]bool{bulkSHA: true}
	d, sha, ok, err = GitLastCommitDateIgnoring(root, "a.md", ignore)
	if err != nil || !ok {
		t.Fatalf("walk-back: err=%v ok=%v", err, ok)
	}
	if sha != realSHA {
		t.Fatalf("walk-back sha = %q, want real authoring %q", sha, realSHA)
	}
	if !sameCalendarDay(d, realAt) {
		t.Fatalf("walk-back date = %v, want real authoring day %v", d, realAt)
	}
}

// An abbreviated SHA in the ignore set still matches the full commit SHA, the
// way `git blame --ignore-revs-file` tolerates either spelling.
func TestGitLastCommitDateIgnoring_AbbreviatedSHAMatches(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	realAt := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "author a", realAt)
	realSHA := headSHA(t, root)

	bulkAt := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "bulk", bulkAt)
	bulkSHA := headSHA(t, root)

	ignore := map[string]bool{bulkSHA[:8]: true} // abbreviated
	d, sha, ok, err := GitLastCommitDateIgnoring(root, "a.md", ignore)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	if sha != realSHA || !sameCalendarDay(d, realAt) {
		t.Fatalf("abbreviated ignore did not walk back; sha=%q date=%v", sha, d)
	}
}

// When EVERY commit that touched the file is ignored, there is no prior real
// commit to compare against — degrade to "no answer" (ok=false), exactly like an
// untracked file, so the drift check simply skips the node rather than erroring.
func TestGitLastCommitDateIgnoring_AllIgnoredDegrades(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "only commit", time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	only := headSHA(t, root)

	_, _, ok, err := GitLastCommitDateIgnoring(root, "a.md", map[string]bool{only: true})
	if err != nil {
		t.Fatalf("all-ignored must not error, got %v", err)
	}
	if ok {
		t.Fatalf("ok=true when every touching commit is ignored; want false")
	}
}
