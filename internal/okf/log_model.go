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
	"regexp"
	"sort"
	"strings"
)

// logModel is a parsed OKF v0.2 §9 log.md: a verbatim preamble (title + any
// prose before the first date heading) plus ordered date groups. AppendLog
// mutates it (fold legacy bullets, insert the new entry) and re-renders it.
//
// An entry is captured as its VERBATIM multi-line text (the `* ` line plus any
// indented continuation lines — tables, nested lists, sub-blocks — up to the next
// top-level bullet or heading). Preserving entries verbatim is load-bearing: the
// real corpus has rich `* **Update**:` entries with embedded markdown tables, and
// re-tokenizing them line-by-line would shred those blocks. This is the "preserve
// the non-edited regions of an in-place edit" discipline applied to the log.
type logModel struct {
	// preamble is the lines before the first `## ` heading, MINUS any legacy
	// `- YYYY-MM-DD — msg` bullets (those are lifted into groups). It is kept
	// verbatim so a non-`# Change Log` title and any hand-written intro survive.
	preamble []string
	// groups are the date-grouped sections in file order. AppendLog sorts them
	// newest-first before rendering.
	groups []*logGroup
	// legacy holds `- YYYY-MM-DD — msg` bullets found in the preamble, in file
	// order, to be folded into groups on the next write.
	legacy []legacyEntry
}

type logGroup struct {
	date    string   // YYYY-MM-DD
	entries []string // full verbatim entry text, each starting with `* ` (or `- `)
}

type legacyEntry struct {
	date string
	// text is the entry's VERBATIM multi-line body with the `- YYYY-MM-DD — `
	// leader already rewritten to `* ` — the bullet's message plus every indented
	// continuation line (tables, nested lists, prose) up to the next top-level
	// bullet or heading. Carrying the block with its bullet is load-bearing: the
	// real corpus has legacy sweep bullets with embedded ledger tables, and
	// lifting only the bullet line strands the table above the first heading,
	// detaching it from its entry and date (silent history corruption of an
	// append-only record).
	text string
}

// legacyDatedBulletRe matches a legacy `- YYYY-MM-DD — msg` bullet (an em dash
// separator, the inline-dated form AppendLog wrote before the §9 rewrite). It is
// anchored at column 0 so an indented `  - 2026-…` continuation line inside a
// rich entry is NOT mistaken for a top-level legacy bullet.
var legacyDatedBulletRe = regexp.MustCompile(`^- (\d{4}-\d{2}-\d{2}) — (.*)$`)

// isTopLevelBullet reports whether line begins a new entry: a `* ` or `- ` bullet
// at column 0 (no leading whitespace). Indented bullets are continuation content.
func isTopLevelBullet(line string) bool {
	return strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "- ")
}

// parseLog splits content into a verbatim preamble, ordered date groups (each
// with verbatim multi-line entries), and any legacy dated bullets before the
// first heading. It is tolerant: anything that is not a heading and not a legacy
// bullet in the preamble is preserved verbatim.
func parseLog(content string) *logModel {
	lg := &logModel{}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")

	var cur *logGroup
	var pending []string // the entry block currently being accumulated in cur

	// A legacy dated bullet in the preamble owns every following line — indented
	// tables, nested lists, prose, blank lines — up to the next top-level bullet
	// or heading. pendingLegacy accumulates that verbatim block so it travels
	// with its bullet into a date group rather than being stranded in the
	// preamble. legacyDate is the ISO date parsed from the bullet leader.
	var pendingLegacy []string
	var legacyDate string

	flushLegacy := func() {
		if len(pendingLegacy) == 0 {
			return
		}
		// Trim trailing blank lines from the legacy block.
		for len(pendingLegacy) > 0 && strings.TrimSpace(pendingLegacy[len(pendingLegacy)-1]) == "" {
			pendingLegacy = pendingLegacy[:len(pendingLegacy)-1]
		}
		lg.legacy = append(lg.legacy, legacyEntry{date: legacyDate, text: strings.Join(pendingLegacy, "\n")})
		pendingLegacy = nil
		legacyDate = ""
	}

	flush := func() {
		if cur == nil || len(pending) == 0 {
			pending = nil
			return
		}
		// Trim trailing blank lines from the entry block.
		for len(pending) > 0 && strings.TrimSpace(pending[len(pending)-1]) == "" {
			pending = pending[:len(pending)-1]
		}
		if len(pending) > 0 {
			cur.entries = append(cur.entries, strings.Join(pending, "\n"))
		}
		pending = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			flushLegacy()
			flush()
			date := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			cur = &logGroup{date: date}
			lg.groups = append(lg.groups, cur)
			continue
		}
		if cur == nil {
			// Preamble region. A legacy `- YYYY-MM-DD — msg` bullet starts a new
			// legacy entry; its continuation block accumulates until the next
			// top-level bullet or heading. Everything else is real preamble —
			// unless we are mid-legacy-block, in which case it belongs to that
			// entry (its indented table/body).
			if m := legacyDatedBulletRe.FindStringSubmatch(line); m != nil {
				flushLegacy()
				legacyDate = m[1]
				// Rewrite the `- YYYY-MM-DD — ` leader to `* <msg>` in place, so the
				// entry renders as a §9 bullet while its continuation stays verbatim.
				pendingLegacy = []string{"* " + m[2]}
				continue
			}
			if len(pendingLegacy) > 0 {
				// A top-level bullet (a `* `/`- ` at column 0 that is NOT a legacy
				// dated bullet) ends the current legacy block and is itself real
				// preamble. Anything else (indented content, blank line, prose)
				// continues the legacy entry.
				if isTopLevelBullet(line) {
					flushLegacy()
					lg.preamble = append(lg.preamble, line)
					continue
				}
				pendingLegacy = append(pendingLegacy, line)
				continue
			}
			lg.preamble = append(lg.preamble, line)
			continue
		}
		// Inside a group. A top-level bullet starts a new entry; everything else
		// (blank lines, indented tables/lists, prose) continues the current one.
		if isTopLevelBullet(line) {
			flush()
			pending = []string{line}
			continue
		}
		if len(pending) == 0 {
			// Content under a heading before any bullet (rare / malformed): keep
			// it as its own entry block so nothing is dropped.
			if strings.TrimSpace(line) == "" {
				continue
			}
			pending = []string{line}
			continue
		}
		pending = append(pending, line)
	}
	flushLegacy()
	flush()

	// Drop a single trailing blank line in the preamble; render adds exactly one
	// blank between the preamble and the first group.
	for len(lg.preamble) > 0 && strings.TrimSpace(lg.preamble[len(lg.preamble)-1]) == "" {
		lg.preamble = lg.preamble[:len(lg.preamble)-1]
	}
	// Drop the scaffold placeholder line so the first real entry replaces it.
	lg.dropPlaceholder()
	return lg
}

// dropPlaceholder removes the scaffold `_No entries yet…_` hint from the preamble
// so the first real entry replaces it rather than sitting below it.
func (lg *logModel) dropPlaceholder() {
	placeholder := strings.TrimSpace(logPlaceholder)
	kept := lg.preamble[:0]
	for _, line := range lg.preamble {
		if strings.TrimSpace(line) == placeholder {
			continue
		}
		kept = append(kept, line)
	}
	lg.preamble = kept
	for len(lg.preamble) > 0 && strings.TrimSpace(lg.preamble[len(lg.preamble)-1]) == "" {
		lg.preamble = lg.preamble[:len(lg.preamble)-1]
	}
}

// groupFor returns the group with the given date, creating it if absent.
func (lg *logModel) groupFor(date string) *logGroup {
	for _, g := range lg.groups {
		if g.date == date {
			return g
		}
	}
	g := &logGroup{date: date}
	lg.groups = append(lg.groups, g)
	return g
}

// addEntry folds any legacy bullets into their date groups (merging into an
// existing group when present, preserving intra-group order), then inserts the
// new message as the FIRST entry of today's group, and re-sorts groups
// newest-first.
func (lg *logModel) addEntry(today, message string) {
	// Fold legacy bullets in file order. Appending after existing entries keeps
	// both the pre-existing group content and the legacy order intact.
	for _, le := range lg.legacy {
		g := lg.groupFor(le.date)
		g.entries = append(g.entries, le.text)
	}
	lg.legacy = nil

	// Insert the new entry at the top of today's group (newest-first within day).
	tg := lg.groupFor(today)
	tg.entries = append([]string{"* " + message}, tg.entries...)

	lg.sortGroupsNewestFirst()
}

// sortGroupsNewestFirst orders groups by date descending. ISO-8601 dates sort
// lexicographically, so a string comparison suffices. The sort is stable so
// equal-date groups keep their relative order (the corpus has duplicate-date
// headings that must not be reordered).
func (lg *logModel) sortGroupsNewestFirst() {
	sort.SliceStable(lg.groups, func(i, j int) bool {
		return lg.groups[i].date > lg.groups[j].date
	})
}

// render serializes the model back to §9 markdown: the verbatim preamble, then
// each date group as `## <date>` followed by its verbatim entries, newest-first.
// Entries are emitted back-to-back (each already carries its own internal
// structure); a multi-line entry keeps its embedded blank lines. A blank line
// separates the preamble from the first group and each group from the next.
func (lg *logModel) render() string {
	var b strings.Builder
	preamble := strings.TrimRight(strings.Join(lg.preamble, "\n"), "\n")
	b.WriteString(preamble)
	if preamble != "" {
		b.WriteString("\n")
	}
	for i, g := range lg.groups {
		if i > 0 || preamble != "" {
			b.WriteString("\n")
		}
		b.WriteString("## " + g.date + "\n")
		for _, e := range g.entries {
			b.WriteString(strings.TrimRight(e, "\n"))
			b.WriteString("\n")
		}
	}
	return b.String()
}
