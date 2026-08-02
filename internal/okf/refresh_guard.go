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

// Guardrail thresholds for RefreshGuard. A bulk mechanical commit shows up in a
// refresh plan as one commit accounting for an overwhelming share of the
// changes — the exact shape whose remediation collapses a corpus's real
// authoring history into the migration date. These two bounds separate that
// shape from ordinary incremental cleanup:
//
//   - refreshGuardMinPlan: below this the plan is too small to be a migration.
//     A handful of nodes fixed from one commit is normal cleanup; refusing there
//     would just cry wolf. 10 is comfortably above the "a few related edits"
//     range and well below any real day-one migration (hundreds to thousands).
//   - refreshGuardShare: the fraction of the plan one commit must account for to
//     look mechanical. At 0.5+ a single commit is rewriting the majority of the
//     plan — not an incremental edit. A diffuse plan (many commits, none
//     dominant) stays under this and is left alone.
//
// Both must be crossed to trigger, so the guard fires only on the large-AND-
// dominated shape and never on small or diffuse plans. The remedy the guard
// points at is `.okf-drift-ignore-revs`, which fixes the root cause (git cannot
// read commit intent) rather than the guessed-from-fan-out symptom.
const (
	refreshGuardMinPlan = 10
	refreshGuardShare   = 0.5
)

// RefreshGuardResult reports whether a refresh plan looks like the remediation
// of a bulk mechanical commit, and the evidence for it.
type RefreshGuardResult struct {
	Triggered bool   // the plan is large and dominated by one commit
	Commit    string // the dominant commit's SHA (empty when not triggered)
	Count     int    // how many of the plan's changes that commit accounts for
	Total     int    // total changes in the plan
}

// RefreshGuard inspects a refresh plan for the bulk-mechanical-commit signature:
// a large plan in which a single commit accounts for an implausible share of the
// changes. Such a plan is not an incremental cleanup — running it would flatten
// every distinct authoring date the commit touched into the migration date. The
// caller uses the result to refuse (or require explicit confirmation) and to
// point the user at `.okf-drift-ignore-revs`, which resolves the case without
// destroying data. RefreshGuard is pure: it reads the plan and writes nothing.
func RefreshGuard(plan []RefreshChange) RefreshGuardResult {
	total := len(plan)
	if total < refreshGuardMinPlan {
		return RefreshGuardResult{Total: total}
	}
	counts := make(map[string]int, total)
	for _, c := range plan {
		if c.Commit == "" {
			continue // no commit attribution: cannot be the dominant one
		}
		counts[c.Commit]++
	}
	var topSHA string
	var topN int
	for sha, n := range counts {
		if n > topN {
			topSHA, topN = sha, n
		}
	}
	triggered := topN >= refreshGuardMinPlan && float64(topN)/float64(total) >= refreshGuardShare
	res := RefreshGuardResult{Total: total}
	if triggered {
		res.Triggered = true
		res.Commit = topSHA
		res.Count = topN
	}
	return res
}
