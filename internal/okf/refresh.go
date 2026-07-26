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
	"path/filepath"
	"sort"
	"time"
)

// RefreshChange is a single planned timestamp correction: the drift finding's
// remediation. AbsPath is the file to rewrite; OldModified is the current
// frontmatter date (bare-date form, for display); NewModified is the value the
// refresh will write — the git last-commit calendar day stamped in the corpus's
// RFC3339-at-midnight-UTC form, so it both resolves the drift and matches the
// bare-date convention the corpus already uses (minimising the git diff).
type RefreshChange struct {
	Path        string // bundle-relative, e.g. "wine/tannin.md"
	AbsPath     string // absolute on-disk path to rewrite
	OldModified string // current modified, "2006-01-02"
	NewModified string // target modified, RFC3339 (e.g. "2026-07-20T00:00:00Z")
}

// driftPair holds a drifting node together with the git commit day the refresh
// would write. It is the single scan both DriftFindings and RefreshPlan build
// on, so the read-only report and the remediation can never disagree about
// which nodes drift.
type driftPair struct {
	path        string
	oldModified time.Time
	gitDay      time.Time // git last-commit instant, in its OWN recorded location
}

// scanDrift walks the bundle once and returns every node whose frontmatter
// `modified` contradicts its git last-commit date, sorted by path. It degrades
// exactly like GitLastCommitDate: outside a git repo, without git, or for an
// untracked file there is no source of truth and the node is skipped. A node
// without a parseable `modified` cannot contradict anything and is skipped.
func scanDrift(b *Bundle) []driftPair {
	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []driftPair
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
		out = append(out, driftPair{path: p, oldModified: mod, gitDay: git})
	}
	return out
}

// refreshStamp renders the git last-commit day as the value to write into
// `modified`. It uses the commit's OWN recorded calendar day (matching
// sameCalendarDay), stamped at midnight UTC in RFC3339 — the corpus's bare-date
// convention — so the write resolves the drift and touches only the date.
func refreshStamp(gitDay time.Time) string {
	y, m, d := gitDay.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format(timestampLayout)
}

// RefreshPlan is the read-only remediation companion of DriftFindings: it
// reports the timestamp correction the refresh would make for every drifting
// node, in path order. It writes nothing. Outside a git repo the plan is empty
// (no source of truth, nothing to fix).
func RefreshPlan(b *Bundle) []RefreshChange {
	pairs := scanDrift(b)
	out := make([]RefreshChange, 0, len(pairs))
	for _, dp := range pairs {
		out = append(out, RefreshChange{
			Path:        dp.path,
			AbsPath:     filepath.Join(b.Root, filepath.FromSlash(dp.path)),
			OldModified: dp.oldModified.Format("2006-01-02"),
			NewModified: refreshStamp(dp.gitDay),
		})
	}
	return out
}

// RefreshPlanNode narrows RefreshPlan to a single bundle-relative path. An
// honest (non-drifting) node yields an empty plan — a clean no-op, not an error.
// A path that is not a node in the bundle is a real caller error.
func RefreshPlanNode(b *Bundle, relPath string) ([]RefreshChange, error) {
	if _, ok := b.Nodes[relPath]; !ok {
		return nil, fmt.Errorf("node not found: %s", relPath)
	}
	for _, c := range RefreshPlan(b) {
		if c.Path == relPath {
			return []RefreshChange{c}, nil
		}
	}
	return nil, nil
}

// RefreshApply writes each planned change to disk, rewriting only the frontmatter
// `modified` field via the order- and body-preserving writer. `created` is never
// touched. It is safe to call with an empty plan (no-op).
func RefreshApply(changes []RefreshChange) error {
	for _, c := range changes {
		at, err := time.Parse(timestampLayout, c.NewModified)
		if err != nil {
			return fmt.Errorf("plan for %s carries an unparseable target %q: %w", c.Path, c.NewModified, err)
		}
		if err := TouchModifiedFile(c.AbsPath, at); err != nil {
			return fmt.Errorf("refresh %s: %w", c.Path, err)
		}
	}
	return nil
}
