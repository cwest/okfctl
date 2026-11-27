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
	"strings"
)

// The v0.1 body `# Citations` list (v0.3 Appendix B) becomes frontmatter
// `sources` (§5.1). §5.1 requires one `resource` per entry and leaves `title`
// optional ("a label for humans"). An entry is converted only when it names
// exactly ONE follow-able resource: an absolute URL or a bundle path (§6.2).
// That resource is the one thing §5.1 requires, and it comes from the entry's
// own text, so nothing is invented. The entry's text becomes the `title`, so
// the claim the author wrote next to the URL is carried across, not dropped.
// An entry with no resource, or with two different ones, is a judgment item:
// choosing which URL is "the" source, or splitting the entry, is the author's
// call.

var (
	// citationBulletRe matches a plain bullet entry (`- ...` / `* ...`); the
	// spec's own v0.1 worked example lists citations as bare bullets.
	citationBulletRe = regexp.MustCompile(`^\s*[-*]\s+\S`)
	// citationMarkerRe strips a leading list bullet from an entry.
	citationMarkerRe = regexp.MustCompile(`^\s*[-*]\s+`)
	// orderedMarkerRe strips a leading ordered-list marker from an entry.
	orderedMarkerRe = regexp.MustCompile(`^\s*\d+\.\s+`)
	// citationLinkSpanRe matches an inline markdown link: link text and target.
	citationLinkSpanRe = regexp.MustCompile(`!?\[([^\]\n]*)\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)
	// citationURLRe matches a bare or <angle-wrapped> absolute http(s) URL.
	citationURLRe = regexp.MustCompile("https?://[^\\s<>\"`\\]]+")
)

// citationEntries returns the entries under a `# Citations` heading in body
// order, each as one line: an entry line plus the wrapped lines that follow it
// are joined with single spaces. An entry starts at a bracket-key, ordered, or
// bullet line; it ends at a blank line, a legend line, the next entry, or the
// next heading. An unbulleted line that opens with a markdown link is wrapped
// text, never an entry start. Lines outside any entry (a status-key paragraph)
// are skipped.
func citationEntries(body string) []string {
	loc := citationsHeadRe.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, line := range strings.Split(body[loc[1]:], "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			flush()
			return out // the next heading ends the Citations section
		case trimmed == "" || citationLegendRe.MatchString(line):
			flush()
		case citationLinkRe.MatchString(line) && !citationBulletRe.MatchString(line) && !citationOrderedRe.MatchString(line):
			// An unbulleted line that opens with a markdown link is wrapped
			// prose, not a bracket-key entry (analyze.go's citationLinkRe
			// draws the same line): it continues the current entry, if any.
			if cur != nil {
				cur = append(cur, trimmed)
			}
		case citationEntryRe.MatchString(line) || citationOrderedRe.MatchString(line) || citationBulletRe.MatchString(line):
			flush()
			cur = []string{strings.TrimRight(line, " \t\r")}
		case cur != nil:
			cur = append(cur, trimmed)
		}
	}
	flush()
	return out
}

// citationSource converts one citation entry to a §5.1 source when it names
// exactly one follow-able resource. A sole link keeps its link text as the
// title and a sole URL gets none; otherwise the title is the entry text with
// its list marker removed. ok is false for an entry with no resource or with
// more than one distinct resource.
func citationSource(entry string) (SourceEdit, bool) {
	text := strings.TrimSpace(entry)
	text = citationMarkerRe.ReplaceAllString(text, "")
	text = orderedMarkerRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)

	var resources []string
	seen := map[string]bool{}
	add := func(r string) {
		if r != "" && !seen[r] {
			seen[r] = true
			resources = append(resources, r)
		}
	}
	linkText := ""
	rest := citationLinkSpanRe.ReplaceAllStringFunc(text, func(span string) string {
		m := citationLinkSpanRe.FindStringSubmatch(span)
		if strings.HasPrefix(span, "!") || strings.HasPrefix(m[2], "#") {
			return " " // an image or an in-page anchor names no source
		}
		add(m[2])
		linkText = m[1]
		return " "
	})
	for _, u := range citationURLRe.FindAllString(rest, -1) {
		add(trimURL(u))
	}
	if len(resources) != 1 {
		return SourceEdit{}, false
	}
	res := resources[0]
	title := text
	switch {
	case citationLinkSpanRe.FindString(text) == text:
		title = strings.TrimSpace(linkText)
	case strings.Trim(text, "<>") == res:
		title = ""
	}
	return SourceEdit{Resource: res, Title: title}, true
}

// trimURL drops the sentence punctuation and emphasis that commonly follow a
// URL in prose, and a closing parenthesis the URL did not open.
func trimURL(u string) string {
	for u != "" {
		last := u[len(u)-1]
		switch {
		case strings.IndexByte(".,;:!?'*_", last) >= 0:
			u = u[:len(u)-1]
		case last == ')' && strings.Count(u, ")") > strings.Count(u, "("):
			u = u[:len(u)-1]
		default:
			return u
		}
	}
	return u
}
