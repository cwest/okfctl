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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// A converted `# Citations` entry keeps its attribution. v0.3 §5.1: a source's
// `id` "SHOULD be present when the body cites the source", and per-claim
// attribution is a Markdown footnote whose label is a `sources[].id` (§4.2).
// Consumers "do not parse the footnote text", so the footnote definition can
// carry the whole entry: its grade, dates and notes survive there, and
// `title` stays a label for humans. Three edits do this:
//
//   - each entry whose source is in `sources` gets an `id` (CitationEntryIDs);
//   - its full text becomes the footnote definition `[^id]: <text>`, written
//     just above the `# Citations` heading, so the v0.1 list parser never
//     reads it as an entry;
//   - each body `[k]` that names the entry becomes `[^id]`.
//
// GFM matches footnote labels case-insensitively, so every uniqueness test
// folds case (SourceIDSet). A node whose ids would collide is refused and
// reported, never renumbered.

// Judgment kinds for citation ids and body references.
const (
	// JudgmentSourceIDCollision: two of a node's ids (assigned, already in
	// `sources`, or already a footnote label in the body) are equal after
	// case folding. The node gets no ids, footnotes or reference rewrites.
	JudgmentSourceIDCollision JudgmentKind = "source-id-collision"
	// JudgmentCitationRef: a body `[k]` that names an entry of this node but
	// sits near "brief", "citation", "node", or a link, or is the label of
	// an undefined `[text][k]` reference. It may name another document's
	// list, so it is left as written.
	JudgmentCitationRef JudgmentKind = "citation-ref-unclear"
	// JudgmentCitationUnattached: a second entry for a resource another
	// entry already holds. A source carries one id, so this entry gets no
	// footnote and its body references stay as written.
	JudgmentCitationUnattached JudgmentKind = "citation-unattached"
)

// SourceIDEdit sets the `id` of a `sources` entry that is already listed by
// Resource and has none.
type SourceIDEdit struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
}

// FootnoteEdit is one footnote definition `[^ID]: Text` (§4.2) holding the
// full text of the citation entry whose source carries ID.
type FootnoteEdit struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// RefEdit is a body citation reference rewritten to footnotes: Old as
// written (`[3]`, `[3]–[5]`, `[1, 2]`), New its footnote form, Count the
// occurrences.
type RefEdit struct {
	Old   string `json:"old"`
	New   string `json:"new"`
	Count int    `json:"count"`
}

// SourceIDSet hands out source ids that are unique within one node after
// case folding, the way GFM matches footnote labels. Every path that assigns
// ids for a node (one id per entry, or `12`, `12-2` for the parts of a split
// entry) claims them from one set, so no label is handed out twice.
type SourceIDSet struct {
	seen map[string]string
}

// NewSourceIDSet returns an empty set.
func NewSourceIDSet() *SourceIDSet {
	return &SourceIDSet{seen: map[string]string{}}
}

// Claim records id. It fails when id is not a usable footnote label (empty,
// or holding whitespace, a bracket or a caret) or when an equal id, after
// case folding, was claimed before.
func (s *SourceIDSet) Claim(id string) error {
	if id == "" || strings.ContainsAny(id, " \t\r\n[]^") {
		return fmt.Errorf("%q cannot be a footnote label", id)
	}
	k := strings.ToLower(id)
	if prev, ok := s.seen[k]; ok {
		return fmt.Errorf("id %q collides with %q (footnote labels match case-insensitively)", id, prev)
	}
	s.seen[k] = id
	return nil
}

// Has reports whether an id equal to id after case folding was claimed.
func (s *SourceIDSet) Has(id string) bool {
	_, ok := s.seen[strings.ToLower(id)]
	return ok
}

// CitationEntryID is the id of one `# Citations` entry. RefKey is the token
// the body cites the entry by (`[RefKey]`): its key when the key became the
// id, its list number when the number did, and "" for a generated id.
type CitationEntryID struct {
	ID     string
	RefKey string
}

var (
	// entryKeyRe captures an entry's leading bracket key, after an optional
	// bullet, ordered marker, and bold: `- [12]`, `3. [S1]`, `**[ga4]**`.
	entryKeyRe = regexp.MustCompile(`^\s*(?:[-*]\s+)?(?:\d+\.\s+)?\*{0,2}\[([^\]\n]+)\]`)
	// entryOrdinalRe captures an ordered-list entry's number.
	entryOrdinalRe = regexp.MustCompile(`^\s*(?:[-*]\s+)?(\d+)\.\s+`)
	// slugKeyRe is a key that can serve as an id as written.
	slugKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
)

// entryKey returns an entry's leading bracket key, or "". A bracket followed
// by `(` or `[` is link text, not a key.
func entryKey(entry string) string {
	loc := entryKeyRe.FindStringSubmatchIndex(entry)
	if loc == nil {
		return ""
	}
	if loc[1] < len(entry) && (entry[loc[1]] == '(' || entry[loc[1]] == '[') {
		return ""
	}
	return entry[loc[2]:loc[3]]
}

// CitationEntryIDs assigns an id to each entry of one node's `# Citations`
// list, in order:
//
//  1. the entry key, when it is a plain slug (`[A-Za-z0-9-]+`) and no other
//     entry in the list has the same key after case folding;
//  2. otherwise the ordered-list number, when the entry is an ordered item
//     and the number is neither a usable key nor an id already assigned;
//  3. otherwise `cite-<n>`, n the entry's 1-based position in the list.
//
// A generated id is never `s<n>`: that collides with an `[S1]`-style key
// once case is folded. The ids are not checked against each other or the
// node's existing labels here; callers claim them from a SourceIDSet and
// refuse the node on a collision.
func CitationEntryIDs(entries []string) []CitationEntryID {
	keys := make([]string, len(entries))
	count := map[string]int{}
	for i, e := range entries {
		keys[i] = entryKey(e)
		if keys[i] != "" {
			count[strings.ToLower(keys[i])]++
		}
	}
	usable := map[string]bool{}
	for _, k := range keys {
		if k != "" && slugKeyRe.MatchString(k) && count[strings.ToLower(k)] == 1 {
			usable[strings.ToLower(k)] = true
		}
	}
	out := make([]CitationEntryID, len(entries))
	assigned := map[string]bool{}
	for i, e := range entries {
		k := keys[i]
		switch m := entryOrdinalRe.FindStringSubmatch(e); {
		case k != "" && usable[strings.ToLower(k)]:
			out[i] = CitationEntryID{ID: k, RefKey: k}
		case m != nil && !usable[strings.ToLower(m[1])] && !assigned[strings.ToLower(m[1])]:
			out[i] = CitationEntryID{ID: m[1], RefKey: m[1]}
		default:
			out[i] = CitationEntryID{ID: "cite-" + strconv.Itoa(i+1)}
		}
		assigned[strings.ToLower(out[i].ID)] = true
	}
	return out
}

// footnoteText is the footnote definition text of an entry: the entry
// without its list marker, wrapped lines already joined by single spaces
// (citationEntries), and without a leading `[id]` when that key became the
// id exactly. Any other leading bracket (a grade such as
// `[PRIMARY-DOC, VERIFIED]`, a bold key) stays.
func footnoteText(entry, id string) string {
	text := entryText(entry)
	if rest, ok := strings.CutPrefix(text, "["+id+"]"); ok && strings.TrimSpace(rest) != "" {
		text = strings.TrimLeft(rest, " \t")
	}
	return text
}

// entryAttachment is what the planner knows about one entry: the resource of
// the source that holds it ("" when it has none), whether another entry
// already holds that resource, and the id that source already carries.
type entryAttachment struct {
	resource   string
	duplicate  bool
	existingID string
	// parts are the second and later sources of a split entry, in order.
	parts []splitPart
}

// existingSourceIDs maps each source resource in a node's frontmatter to its
// id, and lists the ids of every source in order. A number-looking id the
// author left unquoted still counts (§11: read leniently).
func existingSourceIDs(n *Node) (byRes map[string]string, all []string) {
	byRes = map[string]string{}
	raw, _ := n.Frontmatter["sources"].([]any)
	for _, item := range raw {
		m, ok := asStringMap(item)
		if !ok {
			continue
		}
		id := ""
		if v, ok := m["id"]; ok && v != nil {
			id = strings.TrimSpace(fmt.Sprint(v))
		}
		if id == "" {
			continue
		}
		all = append(all, id)
		if res := stringField(m, "resource"); res != "" {
			if _, seen := byRes[res]; !seen {
				byRes[res] = id
			}
		}
	}
	return byRes, all
}

// footnoteLabelRe matches a footnote label anywhere in a body: a reference
// `[^x]` or a definition `[^x]:`.
var footnoteLabelRe = regexp.MustCompile(`\[\^([^\]\s]+)\]`)

// footnoteDefRe matches a footnote definition line and captures its label.
var footnoteDefRe = regexp.MustCompile(`(?m)^ {0,3}\[\^([^\]\s]+)\]:`)

// planSourceIDs plans the ids, footnote definitions, and body reference
// rewrites for one node. att holds one attachment per entry. On a case-folded
// collision the node is reported and nothing is planned for it.
func planSourceIDs(plan *MigratePlan, nm *NodeMigration, n *Node, rel string, entries []string, att []entryAttachment) {
	// A link reference definition (`[label]: url`) is the target of a
	// `[text][label]` link, not a citation the body cites by key: it gets no
	// id and no footnote, and its label is never rewritten.
	att = append([]entryAttachment(nil), att...)
	for i, e := range entries {
		if linkRefDefRe.MatchString(e) {
			att[i] = entryAttachment{}
		}
	}
	ids := CitationEntryIDs(entries)
	for i, e := range entries {
		if linkRefDefRe.MatchString(e) {
			ids[i].RefKey = ""
		}
	}
	srcIDs, allIDs := existingSourceIDs(n)
	final := make([]string, len(entries))
	partIDs := make([][]string, len(entries))
	held := map[string]bool{} // existing ids a matched entry takes over
	for i, a := range att {
		final[i] = ids[i].ID
		if a.resource != "" && !a.duplicate {
			if id := srcIDs[a.resource]; id != "" {
				final[i] = id
				held[strings.ToLower(id)] = true
			}
		}
		for k, p := range a.parts {
			id := splitPartID(final[i], k+2)
			if p.existingID != "" {
				id = p.existingID
				held[strings.ToLower(id)] = true
			}
			partIDs[i] = append(partIDs[i], id)
		}
	}
	set := NewSourceIDSet()
	refuse := func(err error) {
		plan.Judgment = append(plan.Judgment, JudgmentItem{
			Path: rel, Kind: JudgmentSourceIDCollision, Context: err.Error(),
		})
		for i := range nm.Sources {
			nm.Sources[i].ID = ""
		}
	}
	for _, id := range allIDs {
		if held[strings.ToLower(id)] {
			continue
		}
		if err := set.Claim(id); err != nil {
			refuse(fmt.Errorf("existing sources id: %w", err))
			return
		}
	}
	for i := range entries {
		if err := set.Claim(final[i]); err != nil {
			refuse(fmt.Errorf("entry %d: %w", i+1, err))
			return
		}
		for _, id := range partIDs[i] {
			if err := set.Claim(id); err != nil {
				refuse(fmt.Errorf("entry %d: %w", i+1, err))
				return
			}
		}
	}
	defined := map[string]bool{}
	for _, m := range footnoteDefRe.FindAllStringSubmatch(n.Body, -1) {
		defined[strings.ToLower(m[1])] = true
	}
	for _, m := range footnoteLabelRe.FindAllStringSubmatch(n.Body, -1) {
		l := strings.ToLower(m[1])
		if set.Has(l) && !held[l] && !ownLabel(final, partIDs, att, srcIDs, l) {
			refuse(fmt.Errorf("footnote label [^%s] is already in the body", m[1]))
			return
		}
	}

	refKeys := map[string]string{} // body token -> id, attached entries only
	nodeKeys := map[string]bool{}  // body tokens of every entry
	newIdx := map[string]int{}
	for i, s := range nm.Sources {
		newIdx[s.Resource] = i
	}
	for i, a := range att {
		if ids[i].RefKey != "" {
			nodeKeys[ids[i].RefKey] = true
		}
		if a.resource == "" {
			continue
		}
		if a.duplicate {
			plan.Judgment = append(plan.Judgment, JudgmentItem{
				Path: rel, Kind: JudgmentCitationUnattached, Context: entries[i],
			})
			continue
		}
		id := final[i]
		if j, ok := newIdx[a.resource]; ok {
			nm.Sources[j].ID = id
		} else if a.existingID == "" {
			nm.IDs = append(nm.IDs, SourceIDEdit{Resource: a.resource, ID: id})
		}
		if !defined[strings.ToLower(id)] {
			nm.Footnotes = append(nm.Footnotes, FootnoteEdit{ID: id, Text: footnoteText(entries[i], id)})
		}
		cite := []string{id}
		for k, p := range a.parts {
			pid := partIDs[i][k]
			cite = append(cite, pid)
			if j, ok := newIdx[p.resource]; ok {
				nm.Sources[j].ID = pid
			} else if p.existingID == "" {
				nm.IDs = append(nm.IDs, SourceIDEdit{Resource: p.resource, ID: pid})
			}
			if !defined[strings.ToLower(pid)] {
				text := p.def
				if text == "" {
					text = p.resource
				}
				nm.Footnotes = append(nm.Footnotes, FootnoteEdit{ID: pid, Text: text})
			}
		}
		if ids[i].RefKey != "" {
			refKeys[ids[i].RefKey] = strings.Join(cite, " ")
		}
	}
	if len(refKeys) == 0 {
		return
	}
	nm.RefKeys = refKeys
	rewrites, unclear := scanCitationRefs(n.Body, nodeKeys, refKeys)
	nm.Refs = summarizeRefs(rewrites)
	for _, u := range unclear {
		plan.Judgment = append(plan.Judgment, JudgmentItem{Path: rel, Kind: JudgmentCitationRef, Context: u})
	}
}

// ownLabel reports whether label (folded) is the id of an entry, or of a part
// of a split entry, whose source already carries it: a footnote this
// migration wrote on an earlier run.
func ownLabel(final []string, partIDs [][]string, att []entryAttachment, srcIDs map[string]string, label string) bool {
	for i, a := range att {
		if a.resource == "" || a.duplicate {
			continue
		}
		if strings.ToLower(final[i]) == label && srcIDs[a.resource] != "" {
			return true
		}
		for k, p := range a.parts {
			if strings.ToLower(partIDs[i][k]) == label && p.existingID != "" {
				return true
			}
		}
	}
	return false
}

// refRewrite is one body reference to rewrite: the byte span in the scanned
// text and its footnote form.
type refRewrite struct {
	start, end int
	old, new   string
}

func summarizeRefs(rw []refRewrite) []RefEdit {
	idx := map[[2]string]int{}
	var out []RefEdit
	for _, r := range rw {
		k := [2]string{r.old, r.new}
		if i, ok := idx[k]; ok {
			out[i].Count++
			continue
		}
		idx[k] = len(out)
		out = append(out, RefEdit{Old: r.old, New: r.new, Count: 1})
	}
	return out
}

var (
	// bracketTokenRe matches one single-line bracket group.
	bracketTokenRe = regexp.MustCompile(`\[([^\[\]\n]+)\]`)
	// refRangeRe matches a grouped range `[3]–[5]` (en dash or hyphen).
	refRangeRe = regexp.MustCompile(`^\[(\d{1,4})\] ?[–-] ?\[(\d{1,4})\]`)
	// fenceLineRe matches a code fence line.
	fenceLineRe = regexp.MustCompile("^ {0,3}(```|~~~)")
	// inlineCodeSpanRe matches a one-line inline code span.
	inlineCodeSpanRe = regexp.MustCompile("`+[^`\n]*`+")
	// refNearRe is the text near a reference that may mean it names another
	// document's list: "brief", "citation", "node", or a link.
	refNearRe = regexp.MustCompile(`(?i)brief|citation|node|\]\(|https?://`)
	// keyShapedRe is a bracket that reads as a citation: one or more keys
	// (`3`, `1, 2`, `S1`) or a footnote reference (`^3`).
	keyShapedRe = regexp.MustCompile(`^\^?[A-Za-z0-9-]+(?:, ?[A-Za-z0-9-]+)*$`)
)

// refWindow is how far from a reference refNearRe looks, in bytes.
const refWindow = 60

// maskBody returns body with every region a citation reference cannot be in
// replaced by spaces of the same length, so offsets still line up: code
// fences and spans, link reference and footnote definition lines, and the
// `# Citations` section itself.
func maskBody(body string) string {
	b := []byte(body)
	blank := func(s, e int) {
		for i := s; i < e; i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	pos := 0
	inFence := false
	for _, line := range strings.SplitAfter(body, "\n") {
		end := pos + len(line)
		if fenceLineRe.MatchString(line) {
			blank(pos, end)
			inFence = !inFence
		} else if inFence || refDefRe.MatchString(strings.TrimRight(line, "\r\n")) || footnoteDefRe.MatchString(line) {
			blank(pos, end)
		}
		pos = end
	}
	for _, loc := range inlineCodeSpanRe.FindAllIndex(b, -1) {
		blank(loc[0], loc[1])
	}
	if loc := citationsHeadRe.FindIndex(b); loc != nil {
		end := len(b)
		for off := loc[1]; off < len(b); {
			nl := strings.IndexByte(body[off:], '\n')
			if nl < 0 {
				break
			}
			off += nl + 1
			if off < len(b) && strings.HasPrefix(strings.TrimLeft(body[off:], " "), "#") {
				end = off
				break
			}
		}
		blank(loc[0], end)
	}
	return string(b)
}

// definedRefLabels returns the link reference definition labels in body,
// folded (CommonMark matches them case-insensitively). A `[k]` that is such a
// label is a link, not a citation.
func definedRefLabels(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range refDefRe.FindAllStringSubmatch(body, -1) {
		if !strings.HasPrefix(m[1], "^") {
			out[strings.ToLower(m[1])] = true
		}
	}
	return out
}

// refKeysOf splits a bracket group into citation keys: `3` or `1, 2`. ok is
// false when any part is not one of nodeKeys (`[primary, legal]`).
func refKeysOf(inner string, nodeKeys map[string]bool) ([]string, bool) {
	parts := strings.Split(inner, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !nodeKeys[p] {
			return nil, false
		}
		keys = append(keys, p)
	}
	return keys, true
}

// scanCitationRefs finds every body reference to one of nodeKeys outside code,
// links, and the `# Citations` section. A reference whose keys all have an id
// in refKeys is planned as a rewrite to footnotes; one that may name another
// document's list (refNearRe within refWindow, or the label of an undefined
// `[text][k]` reference) is returned as a judgment context instead. A
// reference to an entry that has no source is left alone.
func scanCitationRefs(body string, nodeKeys map[string]bool, refKeys map[string]string) ([]refRewrite, []string) {
	masked := maskBody(body)
	labels := definedRefLabels(body)
	var rewrites []refRewrite
	var unclear []string
	locs := bracketTokenRe.FindAllStringSubmatchIndex(masked, -1)
	skipUntil := 0
	prevEnd, prevKeyShaped := -1, false
	for _, loc := range locs {
		s, e := loc[0], loc[1]
		if s < skipUntil {
			continue
		}
		inner := masked[loc[2]:loc[3]]
		// A bracket right after a key-shaped bracket (`[3][2]`, `[^4][5]`)
		// is the next item of a run of citations, not the label of a
		// `[text][label]` link. The test reads only the token's shape, so
		// the planner and the renderer (which knows fewer keys) agree.
		inChain := prevEnd == s && prevKeyShaped
		prevEnd, prevKeyShaped = e, keyShapedRe.MatchString(inner)
		if strings.HasPrefix(inner, "^") || (s > 0 && (masked[s-1] == '\\' || masked[s-1] == '!')) {
			continue
		}
		if e < len(masked) && masked[e] == '(' {
			continue // inline link text
		}
		if labels[strings.ToLower(inner)] {
			continue // a shortcut or full reference link label
		}
		if e < len(masked) && masked[e] == '[' {
			if nl := bracketTokenRe.FindStringSubmatch(masked[e:]); nl != nil && strings.HasPrefix(masked[e:], nl[0]) && labels[strings.ToLower(nl[1])] {
				continue // link text of `[text][label]`
			}
		}
		var keys []string
		var old string
		if m := refRangeRe.FindStringSubmatch(masked[s:]); m != nil && nodeKeys[m[1]] && nodeKeys[m[2]] {
			lo, _ := strconv.Atoi(m[1])
			hi, _ := strconv.Atoi(m[2])
			if lo >= hi || hi-lo > 50 {
				continue
			}
			for k := lo; k <= hi; k++ {
				keys = append(keys, strconv.Itoa(k))
			}
			old = m[0]
			e = s + len(m[0])
			skipUntil = e
			prevEnd, prevKeyShaped = e, true
		} else {
			ks, ok := refKeysOf(inner, nodeKeys)
			if !ok {
				continue
			}
			keys, old = ks, masked[s:e]
		}
		ctx := refContext(body, s, e)
		if s > 0 && masked[s-1] == ']' && !inChain {
			unclear = append(unclear, ctx) // label of an undefined `[text][k]`
			continue
		}
		if refNearRe.MatchString(masked[max(0, s-refWindow):s]) || refNearRe.MatchString(masked[e:min(len(masked), e+refWindow)]) {
			unclear = append(unclear, ctx)
			continue
		}
		var nb strings.Builder
		for _, k := range keys {
			id, ok := refKeys[k]
			if !ok {
				nb.Reset()
				break
			}
			// A split entry maps to every part's id, space-separated (an
			// id never holds whitespace; SourceIDSet.Claim refuses it).
			for _, part := range strings.Fields(id) {
				nb.WriteString("[^" + part + "]")
			}
		}
		if nb.Len() == 0 {
			continue // names an entry that has no source
		}
		rewrites = append(rewrites, refRewrite{start: s, end: e, old: old, new: nb.String()})
	}
	return rewrites, unclear
}

// refContext is the judgment context for a reference: the reference and the
// text around it on one line.
func refContext(body string, s, e int) string {
	a, z := max(0, s-refWindow), min(len(body), e+refWindow)
	for a > 0 && !utf8.RuneStart(body[a]) {
		a--
	}
	for z < len(body) && !utf8.RuneStart(body[z]) {
		z++
	}
	return body[s:e] + " in: " + strings.Join(strings.Fields(body[a:z]), " ")
}

// renderSourceIDs applies a node's planned footnote definitions and reference
// rewrites to the body of raw. Each edit re-checks the current bytes: a
// definition whose label is already defined is not written again, and a
// reference already in footnote form no longer matches, so re-applying a plan
// changes nothing. Only planned (old, new) pairs are rewritten.
func renderSourceIDs(raw []byte, nm NodeMigration) []byte {
	if len(nm.Footnotes) == 0 && len(nm.Refs) == 0 {
		return raw
	}
	start := 0
	if _, rawAfter, ok := splitFrontmatterRaw(raw); ok {
		start = len(raw) - len(rawAfter)
	}
	body := string(raw[start:])
	var edits []byteEdit
	if len(nm.Refs) > 0 && len(nm.RefKeys) > 0 {
		planned := map[[2]string]bool{}
		for _, r := range nm.Refs {
			planned[[2]string{r.Old, r.New}] = true
		}
		nodeKeys := map[string]bool{}
		for k := range nm.RefKeys {
			nodeKeys[k] = true
		}
		rw, _ := scanCitationRefs(body, nodeKeys, nm.RefKeys)
		for _, r := range rw {
			if planned[[2]string{r.old, r.new}] {
				edits = append(edits, byteEdit{r.start, r.end, []byte(r.new)})
			}
		}
	}
	if loc := citationsHeadRe.FindStringIndex(body); loc != nil && len(nm.Footnotes) > 0 {
		defined := map[string]bool{}
		for _, m := range footnoteDefRe.FindAllStringSubmatch(body, -1) {
			defined[strings.ToLower(m[1])] = true
		}
		eol := "\n"
		if strings.Contains(body, "\r\n") {
			eol = "\r\n"
		}
		var block strings.Builder
		for _, f := range nm.Footnotes {
			if defined[strings.ToLower(f.ID)] {
				continue
			}
			defined[strings.ToLower(f.ID)] = true
			block.WriteString("[^" + f.ID + "]: " + f.Text + eol)
		}
		if block.Len() > 0 {
			before := body[:loc[0]]
			prefix := ""
			if before != "" && !strings.HasSuffix(before, eol+eol) {
				prefix = eol
				if !strings.HasSuffix(before, eol) {
					prefix = eol + eol
				}
			}
			edits = append(edits, byteEdit{loc[0], loc[0], []byte(prefix + block.String() + eol)})
		}
	}
	if len(edits) == 0 {
		return raw
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	out := append([]byte(nil), raw[:start]...)
	return append(out, applyEdits([]byte(body), edits)...)
}

// idScalar builds the scalar for an id, double-quoted when a plain scalar
// would read back as a number, boolean or null (§4: "Producers SHOULD quote
// values that look like numbers or booleans").
func idScalar(id string) *yaml.Node {
	n := scalarValue(id)
	if n.Style == 0 {
		var v any
		if yaml.Unmarshal([]byte(id), &v) != nil {
			n.Style = yaml.DoubleQuotedStyle
		} else if _, isStr := v.(string); !isStr {
			n.Style = yaml.DoubleQuotedStyle
		}
	}
	return n
}
