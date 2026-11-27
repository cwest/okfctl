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
// An entry with no URL or path but exactly one DOI, arXiv id, PubMed id or
// valid ISBN gets its resource built from that identifier (identifierResource).
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
// are joined with single spaces. An entry starts at a bracket-key or bullet
// line; it ends at a blank line, a legend line, the next entry, or the next
// heading. An unbulleted line that opens with a markdown link is wrapped text,
// never an entry start. Lines outside any entry (a status-key paragraph) are
// skipped.
//
// An entry-marker line starts an entry only at the open entry's indent or
// shallower; a marker indented deeper is wrapped text inside that entry. An
// ordered `n.` line starts an entry only where a list item can start: when no
// entry is open (it begins a list block), or as the next sibling of an open
// ordered entry. Inside a bracket-key or bullet entry, a hard-wrapped line
// that merely begins with digits and a period (`2025. https://...`) is
// continuation text. CommonMark reads it the same way: an ordered list may
// interrupt a paragraph only when it starts at 1. Splitting there would turn
// one citation into an untitled source and a judgment item.
func citationEntries(body string) []string {
	loc := citationsHeadRe.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	var out []string
	var cur []string
	curOrdered := false // the open entry is an ordered-list item
	curIndent := 0      // indent of the open ordered entry's marker
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, " "))
			cur = nil
		}
		curOrdered = false
	}
	start := func(line string, ordered bool) {
		flush()
		cur = []string{strings.TrimRight(line, " \t\r")}
		curOrdered = ordered
		curIndent = len(line) - len(strings.TrimLeft(line, " \t"))
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
		case (citationEntryRe.MatchString(line) || citationBulletRe.MatchString(line)) && markerStarts(line, cur != nil, true, curOrdered, curIndent):
			start(line, false)
		case citationOrderedRe.MatchString(line) && markerStarts(line, cur != nil, false, curOrdered, curIndent):
			start(line, true)
		case cur != nil:
			cur = append(cur, trimmed)
		}
	}
	flush()
	return out
}

// markerStarts reports whether an entry-marker line begins a new entry. With no
// entry open, any marker does. With an entry open, a marker indented deeper
// than the open entry's marker is wrapped text inside that entry. At the same
// or a shallower indent, a bracket-key or bullet marker (unordered) starts the
// next entry, and an ordered marker does only when the open entry is itself an
// ordered item (its next sibling); after a bracket-key or bullet entry, a line
// that merely begins with digits and a period is wrapped text.
func markerStarts(line string, open, unordered, curOrdered bool, curIndent int) bool {
	if !open {
		return true
	}
	if len(line)-len(strings.TrimLeft(line, " \t")) > curIndent {
		return false
	}
	return unordered || curOrdered
}

// citationSource converts one citation entry to a §5.1 source; see
// citationSourceFrom.
func citationSource(entry string) (SourceEdit, bool) {
	s, ok, _ := citationSourceFrom(entry)
	return s, ok
}

// citationSourceFrom converts one citation entry to a §5.1 source when it names
// exactly one follow-able resource. A sole link keeps its link text as the
// title and a sole URL gets none; otherwise the title is the entry text with
// its list marker removed. ok is false for an entry with no resource or with
// more than one distinct resource. fromIdentifier reports that the resource
// was built from a DOI, arXiv id, PubMed id or ISBN rather than read as a URL.
func citationSourceFrom(entry string) (src SourceEdit, ok, fromIdentifier bool) {
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
	if len(resources) == 0 {
		// No URL or bundle path: a DOI, arXiv id, PubMed id or valid ISBN in
		// the entry still names one document, and its resolver URL is the
		// resource. The entry text is not copied as the title: a title is
		// the work's name, never the full citation.
		if res, ok := identifierResource(rest); ok {
			return SourceEdit{Resource: res}, true, true
		}
	}
	if len(resources) != 1 {
		return SourceEdit{}, false, false
	}
	res := resources[0]
	title := text
	switch {
	case citationLinkSpanRe.FindString(text) == text:
		title = strings.TrimSpace(linkText)
	case strings.Trim(text, "<>") == res:
		title = ""
	}
	return SourceEdit{Resource: res, Title: title}, true, false
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
