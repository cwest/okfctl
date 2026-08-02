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

import "testing"

// RefreshGuard flags a plan dominated by a single commit — the signature of a
// bulk mechanical commit whose remediation would collapse authoring history. It
// returns the dominant commit, how many of the plan's changes it accounts for,
// and whether the share crosses the "implausible" bar.
func TestRefreshGuard_FlagsSingleCommitDominatedPlan(t *testing.T) {
	// 12 changes, 11 from one commit: an implausible share.
	var plan []RefreshChange
	for i := 0; i < 11; i++ {
		plan = append(plan, RefreshChange{Path: "n", Commit: "bulkbulkbulkbulkbulkbulkbulkbulkbulkbulk"})
	}
	plan = append(plan, RefreshChange{Path: "other", Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	g := RefreshGuard(plan)
	if !g.Triggered {
		t.Fatalf("a plan with 11/12 changes from one commit must trigger the guard")
	}
	if g.Commit != "bulkbulkbulkbulkbulkbulkbulkbulkbulkbulk" {
		t.Fatalf("dominant commit = %q", g.Commit)
	}
	if g.Count != 11 {
		t.Fatalf("dominant count = %d, want 11", g.Count)
	}
	if g.Total != 12 {
		t.Fatalf("total = %d, want 12", g.Total)
	}
}

// A small plan never triggers the guard even if one commit dominates it: a
// handful of changes from one commit is ordinary incremental cleanup, not a
// migration, and refusing there would be the check crying wolf.
func TestRefreshGuard_SmallPlanNeverTriggers(t *testing.T) {
	plan := []RefreshChange{
		{Path: "a", Commit: "same"},
		{Path: "b", Commit: "same"},
		{Path: "c", Commit: "same"},
	}
	if g := RefreshGuard(plan); g.Triggered {
		t.Fatalf("a 3-change plan must not trigger the guard: %+v", g)
	}
}

// A large plan spread across many commits is normal remediation, not a bulk
// mechanical commit — the guard must stay silent (negative control: a case that
// looks big but is legitimately diffuse).
func TestRefreshGuard_DiffusePlanDoesNotTrigger(t *testing.T) {
	var plan []RefreshChange
	for i := 0; i < 40; i++ {
		// Each change from a distinct commit.
		plan = append(plan, RefreshChange{Path: "n", Commit: string(rune('a'+i%26)) + string(rune('0'+i/26))})
	}
	if g := RefreshGuard(plan); g.Triggered {
		t.Fatalf("a diffuse 40-change plan must not trigger the guard: %+v", g)
	}
}

// An empty plan never triggers.
func TestRefreshGuard_EmptyPlan(t *testing.T) {
	if g := RefreshGuard(nil); g.Triggered {
		t.Fatalf("empty plan triggered the guard: %+v", g)
	}
}
