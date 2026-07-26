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

// RefreshPlan is the read-only companion of DriftFindings: for every node whose
// frontmatter `modified` disagrees with git last-commit, it reports the change
// the refresh would make (old modified date -> git last-commit day). It must
// agree with DriftFindings exactly: same nodes, same order.
func TestRefreshPlanMirrorsDrift(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/tannin.md", "2026-07-01T00:00:00Z")
	writeDriftNode(t, root, "wine/acid.md", "2026-07-20T00:00:00Z") // honest below
	commitAt(t, root, "add tannin", time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := RefreshPlan(b)
	if len(plan) != 1 {
		t.Fatalf("want 1 planned change, got %d: %+v", len(plan), plan)
	}
	if plan[0].Path != "wine/tannin.md" {
		t.Fatalf("plan path = %q", plan[0].Path)
	}
	// The target is the git last-commit calendar day, stamped in the corpus's
	// bare-date form.
	if plan[0].NewModified != "2026-07-20T00:00:00Z" {
		t.Fatalf("NewModified = %q, want 2026-07-20T00:00:00Z", plan[0].NewModified)
	}
	if plan[0].OldModified != "2026-07-01" {
		t.Fatalf("OldModified = %q, want 2026-07-01", plan[0].OldModified)
	}

	// Plan and DriftFindings must name the same set of nodes.
	drift := DriftFindings(b)
	if len(drift) != len(plan) {
		t.Fatalf("plan (%d) and drift (%d) disagree", len(plan), len(drift))
	}
}

// After RefreshApply, the drift is gone, created is untouched, and the body is
// preserved verbatim.
func TestRefreshApplyResolvesDriftPreservesCreatedAndBody(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/tannin.md", "2026-07-01T00:00:00Z")
	commitAt(t, root, "add tannin", time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC))

	abs := filepath.Join(root, "wine", "tannin.md")
	before, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := RefreshPlan(b)
	if err := RefreshApply(plan); err != nil {
		t.Fatalf("RefreshApply: %v", err)
	}

	after, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)

	// created is immutable.
	if !strings.Contains(got, "created: 2026-01-01T00:00:00Z") {
		t.Fatalf("created was altered:\n%s", got)
	}
	// modified was refreshed to the git last-commit day.
	if !strings.Contains(got, "modified: 2026-07-20T00:00:00Z") {
		t.Fatalf("modified not refreshed:\n%s", got)
	}
	// body preserved (the "# X" heading from writeDriftNode).
	if !strings.Contains(got, "# X") {
		t.Fatalf("body lost:\n%s", got)
	}
	// Exactly one line changed vs before: the modified line.
	if diffLines(string(before), got) != 1 {
		t.Fatalf("refresh changed more than the modified line:\nBEFORE\n%s\nAFTER\n%s", before, got)
	}

	// Re-loading, the node no longer drifts.
	b2, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if d := DriftFindings(b2); len(d) != 0 {
		t.Fatalf("drift remains after refresh: %+v", d)
	}
}

// Outside a git repo there is no source of truth, so the plan is empty and a
// refresh is a clean no-op.
func TestRefreshPlanNoGitIsEmpty(t *testing.T) {
	root := t.TempDir() // no git init
	writeDriftNode(t, root, "wine/x.md", "2020-01-01T00:00:00Z")
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if plan := RefreshPlan(b); len(plan) != 0 {
		t.Fatalf("no-git bundle produced a plan: %+v", plan)
	}
}

// RefreshPlanNode narrows the plan to a single node's path.
func TestRefreshPlanNodeSingle(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/tannin.md", "2026-07-01T00:00:00Z")
	writeDriftNode(t, root, "wine/body.md", "2026-07-02T00:00:00Z")
	commitAt(t, root, "add two", time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// Both drift; narrowing to one path yields exactly that one.
	if all := RefreshPlan(b); len(all) != 2 {
		t.Fatalf("want 2 drifting, got %d", len(all))
	}
	one, err := RefreshPlanNode(b, "wine/tannin.md")
	if err != nil {
		t.Fatalf("RefreshPlanNode: %v", err)
	}
	if len(one) != 1 || one[0].Path != "wine/tannin.md" {
		t.Fatalf("single-node plan = %+v", one)
	}
}

// RefreshPlanNode on a node that is NOT drifting returns an empty plan (nothing
// to do), not an error — asking to refresh an honest node is a clean no-op.
func TestRefreshPlanNodeHonestIsEmpty(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/acid.md", "2026-07-20T00:00:00Z")
	commitAt(t, root, "add acid", time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	one, err := RefreshPlanNode(b, "wine/acid.md")
	if err != nil {
		t.Fatalf("RefreshPlanNode honest node errored: %v", err)
	}
	if len(one) != 0 {
		t.Fatalf("honest node planned a change: %+v", one)
	}
}

// RefreshPlanNode on a path not in the bundle is a real failure the caller
// should surface.
func TestRefreshPlanNodeUnknownPathErrors(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/acid.md", "2026-07-20T00:00:00Z")
	commitAt(t, root, "add acid", time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshPlanNode(b, "wine/nope.md"); err == nil {
		t.Fatalf("unknown path should error")
	}
}

// A commit made late in the local day (UTC instant rolls to the next date) must
// refresh modified to the LOCAL commit day, not the UTC day — matching the
// drift check's own calendar-day semantics so the refresh actually resolves it.
func TestRefreshApplyUsesCommitLocalDay(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	writeDriftNode(t, root, "wine/x.md", "2026-06-01T00:00:00Z") // stale -> drifts
	loc := time.FixedZone("PDT", -7*3600)
	commitAt(t, root, "add x", time.Date(2026, 6, 28, 20, 58, 10, 0, loc))

	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := RefreshPlan(b)
	if len(plan) != 1 {
		t.Fatalf("want 1 change, got %d: %+v", len(plan), plan)
	}
	// Local commit day is 2026-06-28 even though UTC is 2026-06-29.
	if plan[0].NewModified != "2026-06-28T00:00:00Z" {
		t.Fatalf("NewModified = %q, want local day 2026-06-28T00:00:00Z", plan[0].NewModified)
	}
	if err := RefreshApply(plan); err != nil {
		t.Fatal(err)
	}
	b2, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if d := DriftFindings(b2); len(d) != 0 {
		t.Fatalf("drift remains after local-day refresh: %+v", d)
	}
}

// diffLines counts the number of lines that differ between a and b (by index),
// a coarse guard that a refresh touched only the modified line.
func diffLines(a, b string) int {
	al := strings.Split(a, "\n")
	bl := strings.Split(b, "\n")
	n := 0
	max := len(al)
	if len(bl) > max {
		max = len(bl)
	}
	for i := 0; i < max; i++ {
		var av, bv string
		if i < len(al) {
			av = al[i]
		}
		if i < len(bl) {
			bv = bl[i]
		}
		if av != bv {
			n++
		}
	}
	return n
}
