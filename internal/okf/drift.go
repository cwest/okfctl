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
	"sort"
	"time"
)

// frontmatterTime reads a frontmatter timestamp value. yaml.v3 parses an
// RFC3339 scalar into a time.Time, but a quoted or non-standard value stays a
// string; accept both. Returns ok=false when the value is absent or unparseable
// (a malformed timestamp is not this check's concern — the floor validator owns
// format failures; here it simply means "no reliable modified to compare").
func frontmatterTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), true
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if ts, err := time.Parse(layout, t); err == nil {
				return ts.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

// sameCalendarDay reports whether two instants name the same calendar day, each
// evaluated in its OWN recorded location. The corpus stamps modified as a bare
// date (T00:00:00Z), and a git commit records the author's local offset; judging
// both in a shared UTC frame would flag a late-in-the-local-day commit (whose
// UTC instant rolls to the next date) as drift, a false positive that trains
// users to ignore the check. So each side keeps the day it was written as.
func sameCalendarDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// DriftFindings reports concept nodes whose frontmatter `modified` contradicts
// the file's git last-commit date — the tool noticing when the hand-maintained
// field has gone stale (or been bumped ahead of reality). It is READ-ONLY: it
// never rewrites a node (following the `index check` precedent — report, and let
// a write command fix it).
//
// It degrades cleanly: outside a git repo, when git is unavailable, or for an
// untracked file, there is no source of truth to compare against and no finding
// is produced. A node without a `modified` field cannot contradict anything and
// is skipped. Findings are returned sorted by path for deterministic output.
func DriftFindings(b *Bundle) []Finding {
	var out []Finding
	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		n := b.Nodes[p]
		if n.Frontmatter == nil {
			continue
		}
		mod, ok := frontmatterTime(n.Frontmatter["modified"])
		if !ok {
			continue // no reliable modified to compare
		}
		git, ok, err := GitLastCommitDate(b.Root, p)
		if err != nil || !ok {
			continue // no git source of truth: degrade, do not report
		}
		if sameCalendarDay(mod, git) {
			continue
		}
		out = append(out, Finding{
			Path: p,
			Message: fmt.Sprintf(
				"modified %s disagrees with git last-commit %s; run `okfctl node edit` (or fix the field) to refresh it",
				mod.Format("2006-01-02"), git.Format("2006-01-02"),
			),
		})
	}
	return out
}
