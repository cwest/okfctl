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
	"strconv"
	"strings"
)

// The v0.1 body `# Citations` list (v0.3 Appendix B) becomes frontmatter
// `sources` (§5.1). §5.1 requires one `resource` per entry and leaves `title`
// optional ("a label for humans"). An entry is converted only when it names
// exactly ONE follow-able resource: an absolute URL or a bundle path (§6.2).
// That resource is the one thing §5.1 requires, and it comes from the entry's
// own text, so nothing is invented. The `title` is the cited work's name
// (citationName) when the entry marks one, and is left unset otherwise; the
// full entry text stays in the body.
// An entry with no URL or path but exactly one DOI, arXiv id, PubMed id or
// valid ISBN gets its resource built from that identifier (identifierResource).
// An entry with no resource is a judgment item. An entry with two or more is
// split into one source per resource (citationSourcesFrom,
// planCitationSplits), unless one of its resources is also named by another
// entry, which keeps it for judgment.

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
	// citationRefDefRe matches a whole-line markdown link reference definition
	// (`[label]: dest "title"`).
	citationRefDefRe = regexp.MustCompile(`^ {0,3}\[[^\]\n]+\]:[ \t]*<?[^\s<>]+>?(?:[ \t]+(?:"[^"]*"|'[^']*'|\([^)]*\)))?[ \t]*$`)
)

// citationEntries returns the entries under a `# Citations` heading in body
// order, each as one line: an entry line plus the wrapped lines that follow it
// are joined with single spaces. An entry starts at a bracket-key or bullet
// line; it ends at a blank line, a legend line, the next entry, or the next
// heading. An unbulleted line that opens with a markdown link is wrapped text,
// never an entry start. Lines outside any entry (a status-key paragraph) are
// skipped.
//
// An unbulleted bracket-key line always starts an entry at a paragraph start:
// the first line of the section, or the first after a blank or legend line.
// Inside an open entry, a hard-wrapped line that merely begins with a bracket
// token (`[9]: C4 provides ...`, `[4]).`, `**[2026-11-25 corrected]**`) is
// continuation text; splitting there would write a fragment as a §5.1 title.
// Two shapes still start an entry with no blank line before them: the next
// key of a tight list (`[2]` after `[1]`, `[1b]` after `[1a]`; see
// bracketKeyContinues), and a link reference definition (`[label]: url`)
// right after another, each its own entry as CommonMark reads it.
//
// A bullet line starts an entry only at the open entry's indent or
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
	curRefDef := false  // the open entry is a link reference definition
	curIndent := 0      // indent of the open ordered entry's marker
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, " "))
			cur = nil
		}
		curOrdered = false
		curRefDef = false
	}
	start := func(line string, ordered bool) {
		flush()
		cur = []string{strings.TrimRight(line, " \t\r")}
		curOrdered = ordered
		curRefDef = citationRefDefRe.MatchString(line)
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
		case citationBulletRe.MatchString(line) && markerStarts(line, cur != nil, true, curOrdered, curIndent):
			start(line, false)
		case citationEntryRe.MatchString(line) && !citationBulletRe.MatchString(line) &&
			(cur == nil || curRefDef && citationRefDefRe.MatchString(line) ||
				bracketKeyContinues(cur[0], line, curOrdered, curIndent)):
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

// bracketKeyContinues reports whether an unbulleted bracket-key line starts
// the next entry of a tight list (no blank line between entries) while an
// entry is open. A bracket token on a wrapped line is usually a back-reference
// (`[9]: C4 provides ...`, `[4]).`, `[21].`) or a tag (`**[2026-11-25
// corrected]**`), so the line starts an entry only when its key is a citation
// key (digits, optional lowercase letter) that comes next in sequence after
// the open entry's key: `[2]` after `[1]`, `[1a]` after `[1]`, `[1b]` after
// `[1a]`, `[2]` after `[1c]`. When the open entry carries no citation key (an
// unkeyed bullet, a `[V]` legend-style key), only `[1]`, the start of a keyed
// list, qualifies. A line indented deeper than the open entry's marker, or
// any bracket line inside an ordered `n.` entry, is wrapped text.
func bracketKeyContinues(first, line string, curOrdered bool, curIndent int) bool {
	if curOrdered || len(line)-len(strings.TrimLeft(line, " \t")) > curIndent {
		return false
	}
	n1, s1, ok := citationKey(line)
	if !ok {
		return false
	}
	n0, s0, ok := citationKey(first)
	if !ok {
		return n1 == 1 && s1 == ""
	}
	switch {
	case n1 == n0+1:
		return s1 == ""
	case n1 == n0 && s0 == "":
		return s1 == "a"
	case n1 == n0:
		return s1 != "" && s1[0] == s0[0]+1
	}
	return false
}

// citationKeyRe captures a leading citation key: `[12]`, `**[1b]**`, or the
// same after a list bullet.
var citationKeyRe = regexp.MustCompile(`^\s*(?:[-*]\s+)?(?:\*\*)?\[(\d{1,4})([a-z]?)\]`)

// citationKey parses a line's leading citation key into its number and
// optional letter suffix; ok is false when the line opens with no such key.
func citationKey(line string) (n int, suffix string, ok bool) {
	m := citationKeyRe.FindStringSubmatch(line)
	if m == nil {
		return 0, "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, "", false
	}
	return n, m[2], true
}

// markerStarts reports whether an entry-marker line begins a new entry. With no
// entry open, any marker does. With an entry open, a marker indented deeper
// than the open entry's marker is wrapped text inside that entry. At the same
// or a shallower indent, a bullet marker (unordered) starts the next entry,
// and an ordered marker does only when the open entry is itself an ordered
// item (its next sibling); after a bracket-key or bullet entry, a line that
// merely begins with digits and a period is wrapped text. Unbulleted
// bracket-key lines are decided in citationEntries and bracketKeyContinues.
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
// exactly one follow-able resource. The title is the work's name only
// (citationName); a sole link keeps its link text as the name. An entry
// that marks no name gets no title: §5.1 title is "a label for humans", and
// the full entry text (authors, venue, year, URLs, grades) is not one. ok is
// false for an entry with no resource or with more than one distinct
// resource. fromIdentifier reports that the resource was built from a DOI,
// arXiv id, PubMed id or ISBN rather than read as a URL.
func citationSourceFrom(entry string) (src SourceEdit, ok, fromIdentifier bool) {
	text := strings.TrimSpace(entry)
	text = citationMarkerRe.ReplaceAllString(text, "")
	text = orderedMarkerRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)

	parts, rest, _ := citationResources(text)
	resources := make([]string, len(parts))
	linkText := ""
	for i, p := range parts {
		resources[i] = p.resource
		if p.link != "" {
			linkText = p.linkText
		}
	}
	if len(resources) == 0 {
		// No URL or bundle path: a DOI, arXiv id, PubMed id or valid ISBN in
		// the entry still names one document, and its resolver URL is the
		// resource. Its title is the work's name (citationName), as for any
		// other entry, never the full citation.
		if res, ok := identifierResource(rest); ok {
			title, _ := citationName(text)
			return SourceEdit{Resource: res, Title: title}, true, true
		}
	}
	if len(resources) != 1 {
		return SourceEdit{}, false, false
	}
	res := resources[0]
	title := ""
	switch {
	case citationLinkSpanRe.FindString(text) == text:
		title = strings.TrimSpace(linkText)
	case strings.Trim(text, "<>") == res:
	default:
		title, _ = citationName(text)
	}
	return SourceEdit{Resource: res, Title: title}, true, false
}

// citationPart is one follow-able resource in a citation entry: the resource,
// and, when the entry names it with a markdown link, that link span and its
// text.
type citationPart struct {
	resource string
	link     string // the whole `[text](target)` span, or "" for a bare URL
	linkText string
}

// citationResources returns the distinct follow-able resources of an entry
// (list marker already removed) in the order they appear, and the entry text
// with every link span blanked. A resource is the target of an inline link
// (not an image or an in-page anchor) or a bare or <angle-wrapped> absolute
// URL. Two spellings of one DOI count once (canonicalResource).
//
// A URL inside a code span is an example, a command, or a template
// (`curl https://...`, `https://host/v0/servers?search=<term>`), not a source,
// so it is never counted next to a resource outside code. Only when the
// entry has no resource outside code does a code span count, and then only a
// lone one: `VERIFIED — Docs, `https://docs.example/`: ...` keeps the single
// source it always had, and two code-span URLs stay for judgment.
func citationResources(text string) (parts []citationPart, rest string, fromCode bool) {
	masked := []byte(text)
	for _, loc := range inlineCodeSpanRe.FindAllStringIndex(text, -1) {
		for i := loc[0]; i < loc[1]; i++ {
			masked[i] = ' '
		}
	}
	type found struct {
		at int
		citationPart
	}
	var all []found
	restB := []byte(text)
	for _, m := range citationLinkSpanRe.FindAllSubmatchIndex(masked, -1) {
		span := text[m[0]:m[1]]
		target := text[m[4]:m[5]]
		for i := m[0]; i < m[1]; i++ {
			masked[i], restB[i] = ' ', ' '
		}
		if strings.HasPrefix(span, "!") || strings.HasPrefix(target, "#") {
			continue // an image or an in-page anchor names no source
		}
		// Link text holds no `[`: a match that opened at an enclosing
		// bracket (`[GRADE — see the [node](/x.md)]`) starts at the last one.
		lt := text[m[2]:m[3]]
		if j := strings.LastIndexByte(lt, '['); j >= 0 {
			lt = lt[j+1:]
			span = "[" + lt + "](" + target + ")"
		}
		all = append(all, found{m[0], citationPart{resource: target, link: span, linkText: lt}})
	}
	for _, loc := range citationURLRe.FindAllIndex(masked, -1) {
		all = append(all, found{loc[0], citationPart{resource: trimURL(text[loc[0]:loc[1]])}})
	}
	if len(all) == 0 {
		var code []string
		seen := map[string]bool{}
		for _, span := range inlineCodeSpanRe.FindAllString(text, -1) {
			for _, u := range citationURLRe.FindAllString(span, -1) {
				if k := canonicalResource(trimURL(u)); !seen[k] {
					seen[k] = true
					code = append(code, trimURL(u))
				}
			}
		}
		if len(code) == 1 {
			return []citationPart{{resource: code[0]}}, string(restB), true
		}
		if len(code) > 1 {
			// Several code-span URLs: two resources the author must choose
			// between, never zero (which would read as "no URL at all").
			return []citationPart{{resource: code[0]}, {resource: code[1]}}, string(restB), true
		}
		return nil, string(restB), false
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].at < all[j].at })
	var out []citationPart
	seen := map[string]bool{}
	for _, f := range all {
		if k := canonicalResource(f.resource); f.resource != "" && !seen[k] {
			seen[k] = true
			out = append(out, f.citationPart)
		}
	}
	return out, string(restB), false
}

// citationSourcesFrom splits an entry that names two or more follow-able
// resources into one §5.1 source per resource, in the order they appear;
// §5.1 allows one `resource` per source. ok is false for an entry with fewer
// than two. Each part's `title` follows the name-only rule (citationName): the
// first part takes the name the entry marks, or else its own link text, and a
// later part takes its link text when the entry names it with a link. A bare
// URL part gets no title. Ids are assigned by the planner.
func citationSourcesFrom(entry string) ([]SourceEdit, bool) {
	text := strings.TrimSpace(entry)
	text = citationMarkerRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(orderedMarkerRe.ReplaceAllString(text, ""))
	parts, _, fromCode := citationResources(text)
	if len(parts) < 2 || fromCode {
		return nil, false
	}
	out := make([]SourceEdit, len(parts))
	for i, p := range parts {
		out[i].Resource = p.resource
		if p.link != "" {
			out[i].Title, _ = citationName(p.link)
		}
	}
	if name, ok := citationName(text); ok {
		out[0].Title = name
	}
	return out, true
}

// citationHasProse reports whether a single-resource entry says anything
// beyond its resource: a bare URL or a sole link does not, so there is no
// name to look for and nothing to report when none is found.
func citationHasProse(entry string) bool {
	text := strings.TrimSpace(entry)
	text = citationMarkerRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(orderedMarkerRe.ReplaceAllString(text, ""))
	bare := strings.Trim(text, "<>")
	return citationLinkSpanRe.FindString(text) != text && citationURLRe.FindString(bare) != bare
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
