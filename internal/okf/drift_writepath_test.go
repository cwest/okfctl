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
	"testing"
	"time"
)

// The write path and the drift check must agree on the calendar day BY
// CONSTRUCTION. A node authored through okfctl in the evening US-Eastern and
// committed with that author-local offset must produce ZERO drift findings.
//
// Before the fix the authoring clock stamped the UTC calendar day: an edit at
// 2026-10-09T22:30-04:00 recorded modified=2026-10-10 (next UTC day), while the
// commit kept the author's local 2026-10-09 — so sameCalendarDay (§5: each side
// judged in its OWN recorded location, SPEC v0.2 "an explicit UTC offset") saw a
// disagreement and okfctl's own writer produced a node its own validator rejects
// under --strict. The fix stamps the author's LOCAL day, matching git's frame.
func TestDriftWritePath_EveningLocalCommitHasNoDrift(t *testing.T) {
	loc := time.FixedZone("EDT", -4*3600)
	// Evening US-Eastern: 2026-10-09 locally, which is 2026-10-10 in UTC.
	at := time.Date(2026, 10, 9, 22, 30, 0, 0, loc)
	withClock(t, at)

	root := t.TempDir()
	initRepo(t, root)
	if _, err := NewNode(root, "research/x.md", "Concept", "X"); err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	// Commit with the author's local offset, exactly as a human authoring in the
	// evening would: git records 2026-10-09 -0400.
	commitAt(t, root, "add x", at)

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("evening-local authored node falsely reported as drift: %+v", drift)
	}
}

// Positive control: a genuinely stale modified (authored days before the commit)
// still flags. The fix must not silence real drift.
func TestDriftWritePath_StaleStillFlags(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	// Author at an old clock, then commit days later: modified is really stale.
	loc := time.FixedZone("EDT", -4*3600)
	withClock(t, time.Date(2026, 10, 1, 9, 0, 0, 0, loc))
	if _, err := NewNode(root, "research/stale.md", "Concept", "Stale"); err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	commitAt(t, root, "add stale", time.Date(2026, 10, 9, 22, 30, 0, 0, loc))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	drift := DriftFindings(b)
	if len(drift) != 1 {
		t.Fatalf("want 1 drift finding for a genuinely stale node, got %d: %+v", len(drift), drift)
	}
}

// Negative control: the SAME evening-local instant stamped and committed, but in
// a POSITIVE UTC offset (e.g. +10:00, Sydney morning) where local day == UTC day
// minus one in the other direction. The write path must still agree with git —
// the fix is a general "stamp the author's own day", not an EDT special case.
func TestDriftWritePath_PositiveOffsetMorningHasNoDrift(t *testing.T) {
	loc := time.FixedZone("AEST", 10*3600)
	// 2026-10-10 08:00 +10:00 is 2026-10-09 22:00 UTC — local day is AHEAD of UTC.
	at := time.Date(2026, 10, 10, 8, 0, 0, 0, loc)
	withClock(t, at)

	root := t.TempDir()
	initRepo(t, root)
	if _, err := NewNode(root, "research/syd.md", "Concept", "Syd"); err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	commitAt(t, root, "add syd", at)

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("positive-offset authored node falsely reported as drift: %+v", drift)
	}
}
