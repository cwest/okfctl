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
	"unicode"
	"unicode/utf8"
)

// §5.1 `title` is "a label for humans": the name of the cited work. A
// citation entry carries much more than that (authors, venue, year,
// identifiers, URLs, verification tags), and none of it belongs in a title.
// citationName finds the name only where the entry's own markup marks it, and
// otherwise reports none. It never guesses: a wrong name is worse than no
// name, because the full entry text stays available elsewhere.
//
// A name is marked in one of three ways:
//
//  1. The entry leads with a markdown link: the link text is the name, unless
//     it is a bare path.
//  2. The entry leads with a URL and a dash: the name is a quoted span right
//     after the dash when a colon, parenthetical, or dash follows it (a quote
//     that runs into `;` or ends the entry is an excerpt), or else the text
//     up to the first colon.
//  3. Otherwise the FIRST emphasized (`*Name*`) or quoted (`"Name"`) span is
//     the name, but only in the position a citation puts it: at the start, or
//     after `, ` / `— ` / `: `, and followed by `,` `.` `:` ` (` ` —` or the
//     end. A marked word inside a sentence ("is the *authentication* status",
//     "search for \"cairn\" on") is emphasis or a quoted term, not a name. An
//     italic container followed by `, "Piece"` (a show and its episode, a
//     journal and its article) names the piece.
//
// Only the first marked span is considered; when it fails the position test
// the entry has no name, rather than falling through to a later span. A
// candidate with an ellipsis is a truncated quotation and never a name. When
// the entry opens with a bold span that is not a grade tag (its first word
// not all caps), the bold span names the work, so an italic span after it is
// the venue and gives no name; a quoted span still names the piece.

var (
	nameKeyRe     = regexp.MustCompile(`^\[[^\]\s][^\]]*\]\s+`)
	nameBoldTagRe = regexp.MustCompile(`^\*\*\[[^*]*\*\*\s*`)
	// nameBoldLeadRe matches a bold span that opens the entry.
	nameBoldLeadRe = regexp.MustCompile(`^\*\*([^*]+)\*\*`)
	nameLeadLink   = regexp.MustCompile(`^\[([^\]\n]+)\]\(\s*<?[^)\s>]+>?(?:\s+"[^"]*")?\s*\)`)
	nameLeadURL    = regexp.MustCompile(`^<?https?://\S+?>?\s+[—–-]\s+(.+)$`)
	// nameSpanRe finds an emphasized or quoted span: `*x*` (not `**`),
	// straight double quotes, or curly double quotes.
	nameSpanRe = regexp.MustCompile(`(?:^|[^*])\*([^*\s][^*]*?)\*(?:[^*]|$)|"([^"]+)"|“([^”]+)”`)
)

// citationName returns the name of the work a citation entry cites, and false
// when the entry does not mark one.
func citationName(entry string) (string, bool) {
	text := strings.TrimSpace(entry)
	text = citationMarkerRe.ReplaceAllString(text, "")
	text = orderedMarkerRe.ReplaceAllString(text, "")
	for {
		t := nameBoldTagRe.ReplaceAllString(text, "")
		t = nameKeyRe.ReplaceAllString(t, "")
		t = strings.TrimSpace(t)
		if t == text {
			break
		}
		text = t
	}

	boldLead := false
	if m := nameBoldLeadRe.FindStringSubmatch(text); m != nil {
		first, _, _ := strings.Cut(strings.TrimSpace(m[1]), " ")
		boldLead = strings.ToUpper(first) != first // not a grade tag
	}
	if m := nameLeadLink.FindStringSubmatch(text); m != nil {
		name := strings.TrimSpace(m[1])
		if strings.HasPrefix(name, "/") || strings.HasSuffix(name, ".md") {
			return "", false
		}
		return cleanName(name)
	}
	if m := nameLeadURL.FindStringSubmatch(text); m != nil {
		rest := strings.TrimSpace(m[1])
		if q, after := leadingQuote(rest); q != "" {
			// A quoted excerpt is evidence, not the page's name: a name is
			// followed by a colon, a parenthetical, or a dash.
			for _, p := range []string{":", " (", " —", " –"} {
				if strings.HasPrefix(after, p) && !quoteIsExcerpt("— ", q, after) {
					return cleanName(q)
				}
			}
			return "", false
		}
		head, _, found := strings.Cut(rest, ": ")
		if !found || strings.ContainsAny(head, "`\"*[") || !startsWithLetter(head) {
			return "", false
		}
		return cleanName(head)
	}

	loc := nameSpanRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return "", false
	}
	var start, end int
	switch {
	case loc[2] >= 0:
		start, end = loc[2]-1, loc[3]+1 // include the asterisks
	case loc[4] >= 0:
		start, end = loc[4]-1, loc[5]+1
	default:
		start, end = loc[6]-len("“"), loc[7]+len("”")
	}
	inner := text[start+1 : end-1]
	if strings.HasPrefix(text[start:], "“") {
		inner = text[start+len("“") : end-len("”")]
	}
	before, after := text[:start], text[end:]
	if !namePrefixOK(before) || !nameSuffixOK(after, inner) {
		return "", false
	}
	if text[start] != '*' && quoteIsExcerpt(before, inner, after) {
		return "", false
	}
	// After a bold lead that names the work, an italic span is its venue.
	if boldLead && text[start] == '*' {
		return "", false
	}
	// An italic container followed by a quoted piece (`*Podcast*, "Episode"`)
	// cites the piece.
	if text[start] == '*' && strings.HasPrefix(after, ", ") {
		if q, rest := leadingQuote(after[2:]); q != "" && nameSuffixOK(rest, q) && !quoteIsExcerpt(", ", q, rest) {
			return cleanName(q)
		}
	}
	return cleanName(inner)
}

// excerptIntroducers are the words an entry uses to introduce a quotation
// of the page rather than the page's name ("Description: \"…\"",
// "verbatim: \"…\"", "the `insufficient_scope` error: \"…\"").
var excerptIntroducers = map[string]bool{
	"description": true, "verbatim": true, "states": true, "state": true,
	"says": true, "say": true, "reads": true, "read": true, "error": true,
	"message": true, "text": true, "tagline": true, "abstract": true,
	"statement": true, "quote": true, "quotes": true, "wording": true,
}

// titleSmallWords may be lowercase inside a title-case name.
var titleSmallWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"nor": true, "for": true, "of": true, "on": true, "in": true, "at": true,
	"to": true, "by": true, "with": true, "from": true, "as": true, "is": true,
	"vs": true, "via": true, "per": true, "into": true, "over": true, "are": true,
	"be": true, "it": true, "its": true, "not": true, "no": true,
}

var (
	// excerptSentenceRe finds a sentence break inside a quote: a lowercase
	// word of three or more letters, a period or `!`, a space, and another
	// word. `vs. ` and initials (`U.S. `, `A. `) are too short to match. A
	// `?` is not a break: titles ask questions ("Can There Ever Be Too Many
	// Options? A Meta-Analytic Review").
	excerptSentenceRe = regexp.MustCompile(`\b\p{Ll}{3,}[.!]\s+\pL`)
	nameWordRe        = regexp.MustCompile(`[\pL\pN][\pL\pN'’-]*`)
)

// quoteIsExcerpt reports whether a quoted span (inner) quotes the page
// rather than naming it (§5.1: `title` is the work's name). before is the
// entry text up to the opening quote and after the text past the closing
// one. A quote is an excerpt when:
//
//   - a word like Description, Verbatim, states, reads, error, or message
//     introduces it;
//   - a backticked ref or a URL introduces it (`spend_limit`: "…");
//   - it holds a sentence break, so it is a sentence, not a name;
//   - another quote follows it, so it is one of a list of excerpts;
//   - it opens lowercase after a colon or a URL (a clause fragment); or
//   - a colon introduces it and it is not title case (page text, where a
//     label like "Source video:" introduces a title-case name).
func quoteIsExcerpt(before, inner, after string) bool {
	sep := ""
	head := before
	for _, s := range []string{", ", "— ", "– ", ": "} {
		if strings.HasSuffix(before, s) {
			sep, head = s, strings.TrimRight(before[:len(before)-len(s)], " ")
			break
		}
	}
	fields := strings.Fields(head)
	last := ""
	if len(fields) > 0 {
		last = fields[len(fields)-1]
	}
	if words := nameWordRe.FindAllString(last, -1); len(words) > 0 &&
		excerptIntroducers[strings.ToLower(words[len(words)-1])] {
		return true
	}
	byRef := strings.HasSuffix(last, "`") || strings.Contains(last, "://")
	if byRef && sep != "" {
		return true
	}
	if excerptSentenceRe.MatchString(inner) {
		return true
	}
	rest := strings.TrimLeft(after, ",; ")
	if strings.HasPrefix(rest, `"`) || strings.HasPrefix(rest, "“") {
		return true
	}
	if sep == ": " || byRef {
		r, _ := utf8.DecodeRuneInString(inner)
		if unicode.IsLower(r) {
			first := nameWordRe.FindString(inner)
			if first == strings.ToLower(first) {
				return true
			}
		}
	}
	if sep == ": " && !titleCase(inner) {
		return true
	}
	return false
}

// titleCase reports whether every word of s past the first that is not a
// small word (of, the, for, …) opens with a capital letter or a digit.
func titleCase(s string) bool {
	for i, w := range nameWordRe.FindAllString(s, -1) {
		if i == 0 || titleSmallWords[strings.ToLower(w)] {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// leadingQuote returns the text of a quoted span that opens s and the text
// after its closing quote, or "" and s. Straight quotes are one byte and
// curly quotes three, so the caller never computes the offset itself.
func leadingQuote(s string) (quote, rest string) {
	for _, q := range [][2]string{{`"`, `"`}, {"“", "”"}} {
		if strings.HasPrefix(s, q[0]) {
			body := s[len(q[0]):]
			if i := strings.Index(body, q[1]); i > 0 {
				return body[:i], body[i+len(q[1]):]
			}
		}
	}
	return "", s
}

// namePrefixOK reports whether the text before a marked span puts the span
// where a citation puts a name: at the start, or after a list separator.
func namePrefixOK(before string) bool {
	if strings.TrimSpace(before) == "" {
		return true
	}
	for _, sep := range []string{", ", "— ", "– ", ": "} {
		if strings.HasSuffix(before, sep) {
			return true
		}
	}
	return false
}

// nameSuffixOK reports whether the text after a marked span ends a name. A
// quoted name may carry its own closing comma or period inside the quotes.
func nameSuffixOK(after, inner string) bool {
	if after == "" {
		return true
	}
	if strings.HasSuffix(inner, ",") || strings.HasSuffix(inner, ".") {
		return unicode.IsSpace([]rune(after)[0])
	}
	for _, p := range []string{",", ".", ":", ";", " (", " —", " –"} {
		if strings.HasPrefix(after, p) {
			return true
		}
	}
	return false
}

// cleanName trims a candidate name and rejects anything that is not one: an
// empty or one-character string, text carrying a URL or a link, or a
// truncated quotation (an ellipsis).
func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ",.;:"))
	if utf8.RuneCountInString(s) < 2 || strings.Contains(s, "://") || strings.Contains(s, "](") ||
		strings.Contains(s, "…") || strings.Contains(s, "...") {
		return "", false
	}
	return s, true
}

func startsWithLetter(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
