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
)

// v0.3 §4.2: "Do not use a list of citations in the body." Appendix B: the
// body `# Citations` list is replaced by frontmatter `sources` (§5.1), with
// per-claim attribution as footnotes whose label is a `sources[].id`. migrate
// removes a node's list only when nothing in it would be lost, judged from
// the bytes the migration writes, not from the plan (citationListLoss):
//
//   - every entry's full text (list marker removed, wrapped lines joined, a
//     leading `[id]` dropped only when it became the id: footnoteText) is,
//     whitespace collapsed, the text of a footnote definition whose label is
//     a `sources[].id`, or the start of it followed by a space (holdsEntry:
//     a moved title annotation comes after the entry's text). A split
//     entry's first part holds it.
//   - every resource the entry names (citationResources: link targets and
//     URLs outside code) is some source's `resource`.
//   - the section holds nothing but entries: a status-key line or group
//     label lives nowhere else, so it keeps the list.
//   - no body `[k]` still cites an entry: it would point at nothing.
//
// Otherwise the list stays and a citations-kept judgment item names each
// entry and part that would be lost. The footnote text itself is never
// parsed for meaning (§5.1); it is compared as text, which is what makes the
// removal provably lossless.

// JudgmentCitationsKept: a node's `# Citations` list that migrate leaves in
// place because removing it would lose an entry's text or resource, a line
// that is not an entry, a link reference definition, or the target of a body
// reference. Context names each one. It does not hold the version
// declaration: an entry readable only through the §12.1 fallback is already
// a prose-citation item, and a kept list stays in the file as text.
const JudgmentCitationsKept JudgmentKind = "citations-kept"

// citationSectionSpan returns the byte span of body's `# Citations` section:
// from its heading to the next heading, or to the end of body.
func citationSectionSpan(body string) (start, end int, ok bool) {
	loc := citationsHeadRe.FindStringIndex(body)
	if loc == nil {
		return 0, 0, false
	}
	end = len(body)
	for off := loc[1]; off < len(body); {
		nl := strings.IndexByte(body[off:], '\n')
		if nl < 0 {
			break
		}
		off += nl + 1
		if strings.HasPrefix(strings.TrimLeft(body[off:], " 	"), "#") {
			end = off
			break
		}
	}
	return loc[0], end, true
}

// noteSource is one frontmatter source: its id and resource.
type noteSource struct{ id, resource string }

// frontmatterSources lists fm's `sources` entries in order.
func frontmatterSources(fm map[string]any) []noteSource {
	raw, _ := fm["sources"].([]any)
	var out []noteSource
	for _, item := range raw {
		m, ok := asStringMap(item)
		if !ok {
			continue
		}
		id := ""
		if v, ok := m["id"]; ok && v != nil {
			id = strings.TrimSpace(fmt.Sprint(v))
		}
		out = append(out, noteSource{id: id, resource: stringField(m, "resource")})
	}
	return out
}

// collapse joins s's whitespace runs into single spaces.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// holdsEntry reports whether the footnote text def holds the entry text: it
// is that text, or that text then a space and more (a hand-annotated title
// migrate moved into the footnote). Both are whitespace-collapsed. The entry
// text is then in the footnote byte for byte, so removing the list loses
// nothing; a footnote that only starts the same way does not count.
func holdsEntry(def, entry string) bool {
	return def == entry || strings.HasPrefix(def, entry+" ")
}

// citationListLoss reports what removing the `# Citations` section of a node
// whose file bytes are raw would lose, one line per entry part, body
// reference, or non-entry line; it is empty when the list is recoverable.
// has is false when the node has no such section. rel and b resolve a bundle
// path resource (§6) when its spelling differs between entry and source.
func citationListLoss(b *Bundle, rel string, raw []byte) (lost []string, has bool) {
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		return []string{"frontmatter does not parse: " + err.Error()}, citationsHeadRe.MatchString(string(raw))
	}
	s, e, ok := citationSectionSpan(body)
	if !ok {
		return nil, false
	}
	entries, other := citationSection(body)
	for _, l := range other {
		lost = append(lost, fmt.Sprintf("line %q is not a citation entry", l))
	}

	srcs := frontmatterSources(fm)
	// matchSource finds the source whose resource is res: the same string,
	// the same DOI however spelled (canonicalResource), or the same bundle
	// file (§6) however the path is spelled. A source with an id wins.
	matchSource := func(res string) (noteSource, bool) {
		k := canonicalResource(res)
		var want Ref
		if b != nil {
			want = b.ResolveLink(rel, res)
		}
		var found noteSource
		ok := false
		for _, src := range srcs {
			same := src.resource == res || canonicalResource(src.resource) == k
			if !same && b != nil && want.Kind == RefBundle {
				got := b.ResolvePathField(rel, src.resource)
				same = got.Kind == RefBundle && got.Path == want.Path
			}
			if same && (!ok || found.id == "" && src.id != "") {
				found, ok = src, true
			}
		}
		return found, ok
	}

	// Footnote definitions outside the section, folded label -> text.
	outside := body[:s] + body[e:]
	defs := map[string]string{}
	for _, line := range strings.Split(outside, "\n") {
		loc := footnoteDefRe.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		l := strings.ToLower(line[loc[2]:loc[3]])
		if _, dup := defs[l]; !dup {
			defs[l] = collapse(line[loc[1]:])
		}
	}

	for i, entry := range entries {
		name := fmt.Sprintf("entry %d", i+1)
		if k := entryKey(entry); k != "" {
			name = "entry [" + k + "]"
		}
		if linkRefDefRe.MatchString(entry) {
			// A link reference definition is the target of a `[text][label]`
			// link, not a citation (planSourceIDs gives it no footnote).
			// Removing it would break that link.
			lost = append(lost, fmt.Sprintf("line %q is a link reference definition, not a citation", collapse(entry)))
			continue
		}
		// The entry's first source: its first resource, or the one built
		// from its identifier (identifierResource) when it names none.
		parts, rest, _ := citationResources(entryText(entry))
		first := ""
		if len(parts) > 0 {
			first = parts[0].resource
		} else if res, ok := identifierResource(rest); ok {
			first = res
		}
		src, found := matchSource(first)
		def, defined := defs[strings.ToLower(src.id)]
		switch {
		case first == "":
			lost = append(lost, name+": names no resource, so no source holds it")
		case !found:
			lost = append(lost, fmt.Sprintf("%s: %s is not a source's resource", name, first))
		case src.id == "":
			lost = append(lost, fmt.Sprintf("%s: source %s has no id, so no footnote holds the entry's text", name, first))
		case !defined:
			lost = append(lost, fmt.Sprintf("%s: source %s has no footnote definition [^%s]", name, src.id, src.id))
		case !holdsEntry(def, collapse(footnoteText(entry, src.id))):
			lost = append(lost, fmt.Sprintf("%s: footnote [^%s] does not hold the entry's text", name, src.id))
		}
		for _, p := range parts {
			if _, ok := matchSource(p.resource); !ok && p.resource != first {
				lost = append(lost, fmt.Sprintf("%s: %s is not a source's resource", name, p.resource))
			}
		}
	}

	// A body `[k]` naming an entry still cites the list: every reference
	// scanCitationRefs finds, rewritable or unclear, is one.
	nodeKeys, anyID := map[string]bool{}, map[string]string{}
	for i, id := range CitationEntryIDs(entries) {
		if id.RefKey != "" && !linkRefDefRe.MatchString(entries[i]) {
			nodeKeys[id.RefKey], anyID[id.RefKey] = true, "x"
		}
	}
	if len(nodeKeys) > 0 {
		rw, unclear := scanCitationRefs(body, nodeKeys, anyID)
		for _, r := range rw {
			lost = append(lost, fmt.Sprintf("body reference %s still cites the list", r.old))
		}
		for _, u := range unclear {
			ref, _, _ := strings.Cut(u, " in: ")
			lost = append(lost, fmt.Sprintf("body reference %s still cites the list", ref))
		}
	}
	return lost, true
}

// removeCitationSection returns raw without its body `# Citations` section
// when citationListLoss finds nothing it would lose; otherwise raw unchanged.
// The check runs on the current bytes, so a stale plan never removes a list
// that is no longer recoverable. Everything outside the section keeps its
// bytes; a section that ends the file leaves the text before it ending in
// one line break.
func removeCitationSection(b *Bundle, rel string, raw []byte) []byte {
	if lost, has := citationListLoss(b, rel, raw); !has || len(lost) > 0 {
		return raw
	}
	start := 0
	if _, rawAfter, ok := splitFrontmatterRaw(raw); ok {
		start = len(raw) - len(rawAfter)
	}
	body := string(raw[start:])
	s, e, _ := citationSectionSpan(body)
	before, after := body[:s], body[e:]
	if after == "" {
		eol := "\n"
		if strings.Contains(body, "\r\n") {
			eol = "\r\n"
		}
		before = strings.TrimRight(before, "\r\n")
		if before != "" {
			before += eol
		}
	}
	out := append([]byte(nil), raw[:start]...)
	return append(out, before+after...)
}

// planCitationRemoval decides, for one node whose deterministic edits are
// already planned in nm, whether its `# Citations` list goes: it renders the
// planned edits in memory and checks the result. A list that would lose
// something becomes one citations-kept item naming every loss.
func planCitationRemoval(plan *MigratePlan, b *Bundle, rel string, nm *NodeMigration) error {
	_, out, err := RenderMigration(b.Root, b, *nm)
	if err != nil {
		return err
	}
	lost, has := citationListLoss(b, rel, out)
	switch {
	case !has:
	case len(lost) == 0:
		nm.RemoveCitations = true
	default:
		plan.Judgment = append(plan.Judgment, JudgmentItem{
			Path: rel, Kind: JudgmentCitationsKept, Context: strings.Join(lost, "; "),
		})
	}
	return nil
}
