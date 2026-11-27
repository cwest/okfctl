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
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// rawLink is one Markdown link found in a text, before resolution. Offsets are
// relative to the scanned text, so a writer that scans the full file content
// can splice a replacement in the same coordinate space.
type rawLink struct {
	rawTarget string // full target incl. optional title, e.g. `foo.md "T"`
	url       string // first whitespace field, e.g. `foo.md`
	titleTail string // remainder after url incl. leading space, e.g. ` "T"`
	capStart  int    // byte offset where rawTarget starts
	capEnd    int    // byte offset where rawTarget ends
}

// refDefRe matches a CommonMark link reference definition line:
// up to three spaces of indent, [label]:, whitespace, then the destination and
// an optional title. A label starting with "^" is a footnote, not a link.
var refDefRe = regexp.MustCompile(`(?m)^ {0,3}\[([^\]\n]+)\]:[ \t]+(\S[^\n]*?)[ \t]*$`)

// scanRawLinks returns every Markdown link in body, in document order (§6.1):
//
//   - inline links `[text](target)`; image syntax `![alt](src)` is not a link;
//   - reference-style links: a reference definition `[label]: target` whose
//     label is used by a `[text][label]`, `[label][]`, or `[label]` reference.
//     Footnote definitions (`[^label]: …`) are not links.
//
// `[[wikilinks]]` are not links (§6.1) and never match: the inline form needs
// `](`, and a wikilink label is not a defined reference.
func scanRawLinks(body string) []rawLink {
	var out []rawLink
	for _, loc := range mdLinkRe.FindAllStringSubmatchIndex(body, -1) {
		if loc[0] > 0 && body[loc[0]-1] == '!' {
			continue // image, not a link
		}
		if l, ok := newRawLink(body, loc[2], loc[3]); ok {
			out = append(out, l)
		}
	}
	for _, loc := range refDefRe.FindAllStringSubmatchIndex(body, -1) {
		label := body[loc[2]:loc[3]]
		if strings.HasPrefix(label, "^") || !refLabelUsed(body, label, loc[0], loc[1]) {
			continue
		}
		if l, ok := newRawLink(body, loc[4], loc[5]); ok {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].capStart < out[j].capStart })
	return out
}

func newRawLink(body string, start, end int) (rawLink, bool) {
	raw := body[start:end]
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return rawLink{}, false
	}
	u := fields[0]
	off := strings.Index(raw, u)
	u = strings.TrimSuffix(strings.TrimPrefix(u, "<"), ">")
	return rawLink{
		rawTarget: raw,
		url:       u,
		titleTail: raw[off+len(fields[0]):],
		capStart:  start,
		capEnd:    end,
	}, true
}

// refLabelUsed reports whether a reference label is used anywhere in body
// outside its own definition (CommonMark labels match case-insensitively).
func refLabelUsed(body, label string, defStart, defEnd int) bool {
	needle := "[" + strings.ToLower(label) + "]"
	lower := strings.ToLower(body)
	for i := 0; ; {
		j := strings.Index(lower[i:], needle)
		if j < 0 {
			return false
		}
		at := i + j
		i = at + len(needle)
		if at >= defStart && at < defEnd {
			continue // the definition itself
		}
		if i < len(lower) && lower[i] == ':' {
			continue // another definition of the same label
		}
		return true
	}
}

// retarget renders a replacement for this link's raw target: newURL plus the
// link's original #fragment (or ?query) and CommonMark title, and the original
// <angle-bracket> wrapping when the author used it.
func (l rawLink) retarget(newURL string) string {
	if i := strings.IndexAny(l.url, "#?"); i >= 0 {
		newURL += l.url[i:]
	}
	if strings.HasPrefix(strings.TrimSpace(l.rawTarget), "<") {
		newURL = "<" + newURL + ">"
	}
	return newURL + l.titleTail
}

// scannedLink is one link in a body that resolves (§6) to a concept node.
type scannedLink struct {
	rawLink
	resolved string  // bundle-relative concept node the link resolves to
	form     RefForm // which §6 rule placed it (file-relative or "/"-rooted)
}

// scanNodeLinks returns every link in body that resolves to a concept node,
// using the shared §6 resolver from the file fromFile (a bundle-relative
// path). Edge building, inbound counts, move, and eval all consume this single
// scanner so they can never disagree about what "links to X" means.
func scanNodeLinks(b *Bundle, fromFile, body string) []scannedLink {
	var out []scannedLink
	for _, l := range scanRawLinks(body) {
		r := b.ResolveLink(fromFile, l.url)
		if r.Kind != RefBundle || !r.IsConcept {
			continue
		}
		out = append(out, scannedLink{rawLink: l, resolved: r.Path, form: r.Form})
	}
	return out
}

// linkURLFor renders a bundle-relative target path as a link URL in the given
// §6 form, as seen from the file fromFile: "/"-rooted for FormRootAbs, a bare
// bundle-root path for FormRootFallback (the legacy §12.1 spelling is kept, not
// silently upgraded), otherwise relative to fromFile's directory. Path segments
// are percent-encoded where RFC 3986 requires it (§6 Identifiers).
func linkURLFor(fromFile, target string, form RefForm) string {
	switch form {
	case FormRootAbs:
		return "/" + escapePath(target)
	case FormRootFallback:
		return escapePath(target)
	default:
		return escapePath(relFrom(fromFile, target))
	}
}

// relFrom returns the slash path from fromFile's directory to target, both
// bundle-relative.
func relFrom(fromFile, target string) string {
	var from []string
	if d := dirOf(fromFile); d != "" {
		from = strings.Split(d, "/")
	}
	to := strings.Split(target, "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	parts := make([]string, 0, len(from)-i+len(to)-i)
	for range from[i:] {
		parts = append(parts, "..")
	}
	parts = append(parts, to[i:]...)
	return strings.Join(parts, "/")
}

func dirOf(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func escapePath(p string) string {
	e := (&url.URL{Path: p}).EscapedPath()
	if hasURIScheme(e) {
		// A first segment like "a:b.md" would read as a URI scheme (§6 rule 1).
		e = "./" + e
	}
	return e
}
