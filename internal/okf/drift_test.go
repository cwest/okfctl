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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeDriftNode(t *testing.T, root, rel, modified string) {
	t.Helper()
	body := "---\ntype: Concept\ntitle: X\ncreated: 2026-01-01T00:00:00Z\nmodified: " + modified + "\n---\n\n# X\n"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A node whose frontmatter modified is an earlier calendar day than its git
// last-commit date is drifting (stale/lying) and must be reported.
func TestDriftFindingReportsStale(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/tannin.md", "2026-07-01T00:00:00Z")
	commitAt(t, root, "add tannin", time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	drift := DriftFindings(b)
	if len(drift) != 1 {
		t.Fatalf("want 1 drift finding, got %d: %+v", len(drift), drift)
	}
	if drift[0].Path != "wine/tannin.md" {
		t.Fatalf("drift path = %q", drift[0].Path)
	}
	if !strings.Contains(drift[0].Message, "modified") {
		t.Fatalf("drift message should mention modified: %q", drift[0].Message)
	}
}

// A node whose modified matches its git commit date (same calendar day) is
// honest and must NOT be reported.
func TestDriftFindingHonestNodeNoFinding(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/acid.md", "2026-07-20T00:00:00Z")
	commitAt(t, root, "add acid", time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("honest node reported drift: %+v", drift)
	}
}

// Outside a git repo, DriftFindings degrades to no findings (no crash): git is
// simply unavailable as a source of truth.
func TestDriftFindingNoGitDegradesCleanly(t *testing.T) {
	root := t.TempDir() // no git init
	writeDriftNode(t, root, "wine/x.md", "2020-01-01T00:00:00Z")
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("no-git bundle produced drift findings: %+v", drift)
	}
}

// A commit made late in the local day whose UTC instant rolls into the next
// calendar day must NOT be reported as drift when the frontmatter names the
// LOCAL commit day: the author edited it "that day" in their own timezone, and
// the corpus stamps modified as a bare date. Judging in UTC here would be a
// false positive — the class that trains users to ignore the check.
func TestDriftFindingUsesCommitLocalDay(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	// Frontmatter says 2026-06-28; commit at 2026-06-28T20:58-07:00 is
	// 2026-06-29 in UTC but 2026-06-28 in its own (recorded) timezone.
	writeDriftNode(t, root, "wine/x.md", "2026-06-28T00:00:00Z")
	loc := time.FixedZone("PDT", -7*3600)
	commitAt(t, root, "add x", time.Date(2026, 6, 28, 20, 58, 10, 0, loc))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("late-local-day commit falsely reported as drift: %+v", drift)
	}
}

func TestDriftFindingNoModifiedField(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	abs := filepath.Join(root, "n.md")
	if err := os.WriteFile(abs, []byte("---\ntype: Concept\n---\n\n# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "add n", time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("node without modified reported drift: %+v", drift)
	}
}
