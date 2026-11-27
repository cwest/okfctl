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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitAt runs a git command in dir with author and committer dates pinned to at.
func gitAt(t *testing.T, dir string, at time.Time, args ...string) {
	t.Helper()
	iso := at.Format(time.RFC3339)
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+iso, "GIT_COMMITTER_DATE="+iso)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeAbs(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// allCommits lists every commit reachable from HEAD.
func allCommits(t *testing.T, dir string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-list", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	return strings.Fields(string(out))
}

// branchyRepo builds a repo whose bundle lives in a subdirectory and whose
// history holds the shapes per-path `git log` simplification treats specially:
// a merge TREESAME to only its second parent for one file, a merge that is
// TREESAME to neither parent (an evil merge), a file deleted and re-added, a
// side branch committed with older dates than main, and an untracked file.
// It returns the repo root and the bundle root.
func branchyRepo(t *testing.T) (repo, bundle string) {
	t.Helper()
	repo = t.TempDir()
	initRepo(t, repo)
	bundle = filepath.Join(repo, "bundles", "kb")
	day := func(d int) time.Time { return time.Date(2026, 3, d, 12, 0, 0, 0, time.UTC) }

	writeAbs(t, filepath.Join(bundle, "a.md"), "a1\n")
	writeAbs(t, filepath.Join(bundle, "b.md"), "b1\n")
	writeAbs(t, filepath.Join(bundle, "sub", "c.md"), "c1\n")
	writeAbs(t, filepath.Join(repo, "outside.md"), "o1\n")
	gitAt(t, repo, day(1), "add", "-A")
	gitAt(t, repo, day(1), "commit", "-q", "-m", "root")
	gitAt(t, repo, day(1), "branch", "-M", "main")

	// Side branch with dates OLDER than the main-line commits that follow it.
	gitAt(t, repo, day(2), "checkout", "-q", "-b", "side")
	writeAbs(t, filepath.Join(bundle, "a.md"), "a-side\n")
	gitAt(t, repo, day(2), "commit", "-q", "-am", "side: a")
	writeAbs(t, filepath.Join(bundle, "sub", "c.md"), "c-side\n")
	gitAt(t, repo, day(3), "commit", "-q", "-am", "side: c")

	gitAt(t, repo, day(4), "checkout", "-q", "main")
	writeAbs(t, filepath.Join(bundle, "b.md"), "b2\n")
	gitAt(t, repo, day(5), "commit", "-q", "-am", "main: b")
	writeAbs(t, filepath.Join(repo, "outside.md"), "o2\n")
	gitAt(t, repo, day(6), "commit", "-q", "-am", "main: outside only")

	// Merge: a.md and c.md TREESAME to the second parent only; b.md TREESAME
	// to the first parent only.
	gitAt(t, repo, day(7), "merge", "-q", "--no-ff", "--no-edit", "side")

	// Evil merge: a second side branch and main both edit d.md, and the merge
	// resolves to content that matches neither parent.
	writeAbs(t, filepath.Join(bundle, "d.md"), "d1\n")
	gitAt(t, repo, day(8), "add", "-A")
	gitAt(t, repo, day(8), "commit", "-q", "-m", "add d")
	gitAt(t, repo, day(9), "checkout", "-q", "-b", "side2")
	writeAbs(t, filepath.Join(bundle, "d.md"), "d-side2\n")
	// Same committer date as main's edit below: the walk's tie order counts.
	gitAt(t, repo, day(10), "commit", "-q", "-am", "side2: d")
	gitAt(t, repo, day(10), "checkout", "-q", "main")
	writeAbs(t, filepath.Join(bundle, "d.md"), "d-main\n")
	gitAt(t, repo, day(10), "commit", "-q", "-am", "main: d")
	mergeCmd := exec.Command("git", "merge", "-q", "--no-ff", "--no-commit", "side2")
	mergeCmd.Dir = repo
	_ = mergeCmd.Run() // conflicts by design; resolved below
	writeAbs(t, filepath.Join(bundle, "d.md"), "d-resolved\n")
	gitAt(t, repo, day(11), "add", "-A")
	gitAt(t, repo, day(11), "commit", "-q", "--no-edit")

	// Delete and re-add a file.
	gitAt(t, repo, day(12), "rm", "-q", filepath.Join("bundles", "kb", "b.md"))
	gitAt(t, repo, day(12), "commit", "-q", "-m", "drop b")
	writeAbs(t, filepath.Join(bundle, "b.md"), "b3\n")
	gitAt(t, repo, day(13), "add", "-A")
	gitAt(t, repo, day(13), "commit", "-q", "-m", "re-add b")

	// A merge that takes the side's version of e.md over a NEWER main-line
	// edit: per-path log hides the main edit (the merge is TREESAME to its
	// second parent), so e.md's last commit is the older side commit.
	writeAbs(t, filepath.Join(bundle, "e.md"), "e1\n")
	gitAt(t, repo, day(14), "add", "-A")
	gitAt(t, repo, day(14), "commit", "-q", "-m", "add e")
	gitAt(t, repo, day(15), "checkout", "-q", "-b", "side3")
	writeAbs(t, filepath.Join(bundle, "e.md"), "e-side3\n")
	gitAt(t, repo, day(15), "commit", "-q", "-am", "side3: e")
	gitAt(t, repo, day(16), "checkout", "-q", "main")
	writeAbs(t, filepath.Join(bundle, "e.md"), "e-main\n")
	gitAt(t, repo, day(16), "commit", "-q", "-am", "main: e")
	mergeCmd = exec.Command("git", "merge", "-q", "--no-ff", "--no-commit", "side3")
	mergeCmd.Dir = repo
	_ = mergeCmd.Run()
	writeAbs(t, filepath.Join(bundle, "e.md"), "e-side3\n")
	gitAt(t, repo, day(17), "add", "-A")
	gitAt(t, repo, day(17), "commit", "-q", "--no-edit")

	// An evil merge over h.md and i.md whose parent branches carry different
	// dates: with the merge ignored, the walk must pick the NEWER of the two
	// branch edits (side for h, main for i), which only date order gets right.
	writeAbs(t, filepath.Join(bundle, "h.md"), "h1\n")
	writeAbs(t, filepath.Join(bundle, "i.md"), "i1\n")
	gitAt(t, repo, day(18), "add", "-A")
	gitAt(t, repo, day(18), "commit", "-q", "-m", "add h i")
	gitAt(t, repo, day(19), "checkout", "-q", "-b", "side4")
	writeAbs(t, filepath.Join(bundle, "h.md"), "h-side4\n")
	writeAbs(t, filepath.Join(bundle, "i.md"), "i-side4\n")
	gitAt(t, repo, day(20), "commit", "-q", "-am", "side4: h i")
	gitAt(t, repo, day(19), "checkout", "-q", "main")
	writeAbs(t, filepath.Join(bundle, "h.md"), "h-main\n")
	gitAt(t, repo, day(19), "commit", "-q", "-am", "main: h")
	writeAbs(t, filepath.Join(bundle, "i.md"), "i-main\n")
	gitAt(t, repo, day(21), "commit", "-q", "-am", "main: i")
	mergeCmd = exec.Command("git", "merge", "-q", "--no-ff", "--no-commit", "side4")
	mergeCmd.Dir = repo
	_ = mergeCmd.Run()
	writeAbs(t, filepath.Join(bundle, "h.md"), "h-resolved\n")
	writeAbs(t, filepath.Join(bundle, "i.md"), "i-resolved\n")
	gitAt(t, repo, day(22), "add", "-A")
	gitAt(t, repo, day(22), "commit", "-q", "--no-edit")

	// Untracked file: never committed.
	writeAbs(t, filepath.Join(bundle, "untracked.md"), "u\n")
	return repo, bundle
}

type lastCommit struct {
	day string
	sha string
	ok  bool
}

func (l lastCommit) String() string { return fmt.Sprintf("{%s %s %v}", l.day, l.sha, l.ok) }

// The batched history must answer exactly what per-path `git log` answers, for
// every file and for every single-commit ignore set (so the walk-back order past
// an ignored rev is checked too). Per-path `git log` is the oracle: the batched
// walk replays its default history simplification, and this pins that it does.
func TestGitHistory_MatchesPerPathGitLog(t *testing.T) {
	repo, bundle := branchyRepo(t)
	files := []string{"a.md", "b.md", "sub/c.md", "d.md", "e.md", "h.md", "i.md", "sub", "untracked.md", "missing.md"}

	h, err := loadGitHistory(bundle)
	if err != nil || h == nil {
		t.Fatalf("loadGitHistory: h=%v err=%v", h, err)
	}

	ignoreSets := []map[string]bool{nil}
	for _, sha := range allCommits(t, repo) {
		ignoreSets = append(ignoreSets, map[string]bool{sha: true})
	}
	for _, ignore := range ignoreSets {
		for _, f := range files {
			wd, wsha, wok, werr := GitLastCommitDateIgnoring(bundle, f, ignore)
			gd, gsha, gok, gerr := h.lastCommit(f, ignore)
			if werr != nil || gerr != nil {
				t.Fatalf("%s ignore=%v: errs per-path=%v batched=%v", f, ignore, werr, gerr)
			}
			want := lastCommit{wd.Format(time.RFC3339), wsha, wok}
			got := lastCommit{gd.Format(time.RFC3339), gsha, gok}
			if want != got {
				t.Errorf("%s ignore=%v: batched %v, per-path git log %v", f, ignore, got, want)
			}
		}
	}
}

// Outside a git repository the batched history is "no answer" (nil, nil), the
// same degrade the per-path lookup gives — never an error.
func TestGitHistory_NotARepoDegrades(t *testing.T) {
	if !GitAvailable() {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	h, err := loadGitHistory(dir)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if h != nil {
		t.Fatalf("h = %v, want nil outside a repo", h)
	}
}

// A repo with no commits yet has no HEAD: no answer, not an error.
func TestGitHistory_EmptyRepoDegrades(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	h, err := loadGitHistory(dir)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if h != nil {
		t.Fatalf("h = %v, want nil for a repo with no commits", h)
	}
}

// The drift scan must cost a constant number of git processes, not one per
// node: that per-node spawn is what made `validate --strict` take minutes on a
// few-hundred-node bundle.
func TestScanDrift_ConstantGitInvocations(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	for i := 0; i < 25; i++ {
		writeDriftNode(t, root, fmt.Sprintf("n%02d.md", i), "2026-01-01T00:00:00Z")
	}
	commitAt(t, root, "add nodes", time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	prev := gitOutput
	gitOutput = func(c *exec.Cmd) ([]byte, error) { calls++; return prev(c) }
	t.Cleanup(func() { gitOutput = prev })

	if got := len(DriftFindings(b)); got != 25 {
		t.Fatalf("drift findings = %d, want 25", got)
	}
	if calls > 3 {
		t.Fatalf("drift scan spawned %d git processes for 25 nodes; want a constant <= 3", calls)
	}
}

// When the user's git config turns on log.follow, per-path `git log` follows
// renames, which the batched walk does not model. The scan then keeps the
// per-path lookup so its answer is unchanged. With the rename commit listed in
// the ignore file, only rename-following reaches the pre-rename authoring
// commit, so the node drifts only if the scan honored log.follow.
func TestScanDrift_LogFollowKeepsPerPathAnswer(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "old.md", "2026-01-01T00:00:00Z")
	commitAt(t, root, "author", time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))
	gitAt(t, root, time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC), "mv", "old.md", "new.md")
	commitAt(t, root, "rename", time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC))
	writeAbs(t, filepath.Join(root, DriftIgnoreRevsFile), headSHA(t, root)+"\n")
	gitAt(t, root, time.Now(), "config", "log.follow", "true")

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	drift := DriftFindings(b)
	if len(drift) != 1 || drift[0].Path != "new.md" {
		t.Fatalf("drift = %+v; want new.md drifting against its pre-rename commit (log.follow honored)", drift)
	}
}
