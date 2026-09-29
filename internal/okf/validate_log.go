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
	"strings"
	"time"
)

// isoDateLayout is the ISO 8601 calendar-date layout §9 mandates for log date
// headings. time.Parse rejects both non-dates ("last Tuesday") and impossible
// dates ("2025-13-45"), which a bare regex would let through.
const isoDateLayout = "2006-01-02"

// legacyBulletDatePrefixLen is the byte length of a legacy `- YYYY-MM-DD ` prefix
// (dash, space, 10-char date, space) that AppendLog wrote before this change.
const legacyBulletDatePrefixLen = len("- 2006-01-02 ")

// validateReservedLog enforces OKF v0.2 §9 on a reserved log.md at any depth:
//   - it must begin with a `# ` title and carry NO frontmatter block (§9);
//   - every `## ` heading must be a valid YYYY-MM-DD date (§9's one MUST);
//   - consecutive date headings must be non-increasing (newest first, §9);
//   - a log with entry content must have at least one date heading (§9);
//   - legacy `- YYYY-MM-DD — msg` bullets before the first date heading are not
//     date-grouped and must be regrouped (the shape AppendLog wrote pre-fix).
//
// An empty log, a title-only log, and the `bundle init` scaffold (title +
// placeholder line) are all well-formed and stay silent, so a fresh bundle never
// trips the floor. The load-bearing invariant: Validate never rejects a log that
// AppendLog produced.
func validateReservedLog(path string, n *Node) []Finding {
	var out []Finding
	if n.Frontmatter == nil {
		return append(out, Finding{Path: path, Message: "unparseable frontmatter"})
	}
	// §9: a log carries no frontmatter block; its body must open with the title.
	// A frontmatter-bearing log fails here because its content does not begin
	// with the required `# ` title.
	if n.HasFrontmatterBlock {
		return append(out, Finding{Path: path, Message: "log.md must begin with a `# ` title heading (§9); a frontmatter block is not permitted"})
	}

	body := n.Body
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return out // empty log — well-formed.
	}

	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return append(out, Finding{Path: path, Message: "log.md must begin with a `# ` title heading (§9); a frontmatter block is not permitted"})
	}

	var (
		dates        []string
		sawHeading   bool
		sawEntryOnly bool // non-heading, non-title content before the first heading
		sawLegacy    bool
	)
	for _, line := range lines[1:] {
		switch {
		case strings.HasPrefix(line, "## "):
			sawHeading = true
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if _, err := time.Parse(isoDateLayout, heading); err != nil {
				out = append(out, Finding{Path: path, Message: fmt.Sprintf("date heading %q is not a valid YYYY-MM-DD date (§9)", heading)})
			} else {
				dates = append(dates, heading)
			}
		default:
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !sawHeading {
				sawEntryOnly = true
				if isLegacyDatedBullet(line) {
					sawLegacy = true
				}
			}
		}
	}

	// §9: consecutive date headings must be non-increasing (equal allowed).
	for i := 1; i < len(dates); i++ {
		if dates[i] > dates[i-1] {
			out = append(out, Finding{Path: path, Message: fmt.Sprintf("date headings are not newest first (§9): %s before %s", dates[i-1], dates[i])})
		}
	}

	// §9: legacy dated bullets before the first heading are not date-grouped.
	if sawLegacy {
		out = append(out, Finding{Path: path, Message: "log has legacy dated bullets not grouped under date headings (§9); run `okfctl log append` to regroup"})
	}

	// §9: a log with entry content but no date heading is not date-grouped. The
	// scaffold placeholder is entry-shaped prose but must stay silent; treat it
	// as no content. A log with only a title and no entries also stays silent.
	if !sawHeading && sawEntryOnly && !isScaffoldPlaceholderOnly(lines[1:]) && !sawLegacy {
		out = append(out, Finding{Path: path, Message: "log has no ISO 8601 date headings (§9)"})
	}

	return out
}

// isLegacyDatedBullet reports whether line is a legacy `- YYYY-MM-DD — msg`
// bullet (the inline-dated form AppendLog wrote before the §9 rewrite).
func isLegacyDatedBullet(line string) bool {
	if !strings.HasPrefix(line, "- ") || len(line) < legacyBulletDatePrefixLen {
		return false
	}
	date := line[2:12]
	if _, err := time.Parse(isoDateLayout, date); err != nil {
		return false
	}
	// A space must follow the date (guards `- 2026-01-01foo`).
	return line[12] == ' '
}

// isScaffoldPlaceholderOnly reports whether the post-title lines are exactly the
// `bundle init` scaffold placeholder (blank lines plus the single placeholder
// hint), so a freshly scaffolded log stays silent under §9.
func isScaffoldPlaceholderOnly(afterTitle []string) bool {
	placeholder := strings.TrimSpace(logPlaceholder)
	for _, line := range afterTitle {
		t := strings.TrimSpace(line)
		if t == "" || t == placeholder {
			continue
		}
		return false
	}
	return true
}
