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
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// errFrontmatterNotMapping reports a frontmatter block that does not decode to
// a YAML mapping (empty, a scalar, a sequence).
var errFrontmatterNotMapping = errors.New("frontmatter is not a mapping")

// errNoFrontmatterBlock reports a file with no `---` fenced frontmatter block.
var errNoFrontmatterBlock = errors.New("no frontmatter block")

// editFrontmatter applies mutate to the top-level frontmatter mapping of raw (a
// whole node file) and returns the new file bytes.
//
// OKF v0.3 §4: "A producer that modifies an existing file SHOULD NOT
// re-serialize values that it did not change." So the result is a splice: every
// top-level entry whose value mutate left alone keeps its original bytes (its
// quoting, alignment padding, indentation, comments, and the blank lines around
// it), and only the entries mutate changed, removed, or added are touched. v0.3
// §4.1 ("MUST keep the keys that it does not know") holds by construction:
// unknown and `x-` keys are never re-rendered at all. The body is copied
// verbatim.
//
// mutate is the same yaml.Node mutation the writers have always used
// (setScalar, removeKey, setMapping, appendSources), so WHAT changes is decided
// exactly as before; this function only changes HOW the change reaches disk.
//
// The splice is verified before it is trusted: the spliced block must parse and
// decode to the same value, with the same top-level key order, as the mutated
// tree. Any YAML shape the splicer does not model (an indented top-level
// mapping, complex keys, duplicate keys, anchors resolving differently after the
// edit) fails that check, and the function falls back to re-encoding the whole
// block, byte-identical to the pre-splice writer. The fallback is a correctness
// backstop, never a silent corruption path.
func editFrontmatter(raw []byte, mutate func(*yaml.Node)) ([]byte, error) {
	head, block, tail, ok := splitFrontmatterVerbatim(raw)
	if !ok {
		return nil, errNoFrontmatterBlock
	}
	var origDoc, editDoc yaml.Node
	if err := yaml.Unmarshal(block, &origDoc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if err := yaml.Unmarshal(block, &editDoc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	orig := frontmatterMapping(&origDoc)
	edited := frontmatterMapping(&editDoc)
	if orig == nil || edited == nil {
		return nil, errFrontmatterNotMapping
	}
	mutate(edited)

	if spliced, ok := spliceFrontmatter(block, orig, edited); ok {
		var out bytes.Buffer
		out.Grow(len(head) + len(spliced) + len(tail))
		out.Write(head)
		out.Write(spliced)
		out.Write(tail)
		return out.Bytes(), nil
	}
	return reencodeFrontmatter(raw, edited)
}

// reencodeFrontmatter is the pre-splice writer, kept verbatim as the fallback:
// it re-serializes the whole mutated mapping and copies the body region through.
func reencodeFrontmatter(raw []byte, edited *yaml.Node) ([]byte, error) {
	_, rawAfter, ok := splitFrontmatterRaw(raw)
	if !ok {
		return nil, errNoFrontmatterBlock
	}
	var fmBuf bytes.Buffer
	enc := yaml.NewEncoder(&fmBuf)
	enc.SetIndent(2)
	if err := enc.Encode(edited); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	_ = enc.Close()

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(fmBuf.Bytes())
	out.WriteString("---\n")
	out.Write(rawAfter)
	return out.Bytes(), nil
}

// splitFrontmatterVerbatim splits a node file into the opening fence line, the
// raw frontmatter block, and everything from the closing fence on, with every
// byte (CRLF included) preserved: head+block+tail == src. The fence rules match
// splitFrontmatterRaw (§4: the file starts with a line that is exactly `---`,
// and the block ends at the next line that is exactly `---`).
func splitFrontmatterVerbatim(src []byte) (head, block, tail []byte, ok bool) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim+"\n")) && !bytes.HasPrefix(src, []byte(delim+"\r\n")) {
		return nil, nil, nil, false
	}
	start := bytes.IndexByte(src, '\n') + 1
	pos := start
	for pos < len(src) {
		end := len(src)
		if i := bytes.IndexByte(src[pos:], '\n'); i >= 0 {
			end = pos + i + 1
		}
		line := bytes.TrimSuffix(bytes.TrimSuffix(src[pos:end], []byte("\n")), []byte("\r"))
		if bytes.Equal(line, []byte(delim)) {
			return src[:start], src[start:pos], src[pos:], true
		}
		pos = end
	}
	return nil, nil, nil, false
}

// fmEntry is one top-level frontmatter entry located in the raw block.
type fmEntry struct {
	key       string
	keyNode   *yaml.Node
	valNode   *yaml.Node
	startLine int // 1-based line of the key
	endLine   int // 1-based last line of the value (inclusive)
}

// byteEdit replaces block[start:end] with repl.
type byteEdit struct {
	start, end int
	repl       []byte
}

// spliceFrontmatter returns block with only the entries that differ between
// orig and edited rewritten. ok is false when the edit cannot be expressed as a
// verified splice; the caller then re-encodes.
func spliceFrontmatter(block []byte, orig, edited *yaml.Node) ([]byte, bool) {
	// spliceInlineScalar records a kept quoting style on the edited value
	// nodes. If the splice is abandoned, undo that so the re-encode fallback
	// renders exactly what the mutation produced.
	type nodeStyle struct {
		n     *yaml.Node
		style yaml.Style
		tag   string
	}
	var saved []nodeStyle
	for i := 1; i < len(edited.Content); i += 2 {
		n := edited.Content[i]
		saved = append(saved, nodeStyle{n, n.Style, n.Tag})
	}
	out, ok := trySplice(block, orig, edited)
	if !ok {
		for _, s := range saved {
			s.n.Style, s.n.Tag = s.style, s.tag
		}
	}
	return out, ok
}

// trySplice is spliceFrontmatter without the abandon-cleanup.
func trySplice(block []byte, orig, edited *yaml.Node) ([]byte, bool) {
	lines := lineOffsets(block)
	eol := "\n"
	if bytes.Contains(block, []byte("\r\n")) {
		eol = "\r\n"
	}

	origEntries, ok := locateEntries(block, lines, orig)
	if !ok {
		return nil, false
	}
	editedKeys, editedVals, ok := mappingPairs(edited)
	if !ok {
		return nil, false
	}

	// The writers only ever change values in place, remove keys, and append
	// keys at the end. Any other shape (a reorder) is left to the fallback.
	editedIdx := map[string]int{}
	for i, k := range editedKeys {
		editedIdx[k] = i
	}
	last := -1
	retained := 0
	for _, e := range origEntries {
		i, kept := editedIdx[e.key]
		if !kept {
			continue
		}
		if i < last {
			return nil, false
		}
		last = i
		retained++
	}
	if last+1 != retained {
		// An added key landed before a retained one.
		return nil, false
	}

	var edits []byteEdit
	for _, e := range origEntries {
		i, kept := editedIdx[e.key]
		spanStart, spanEnd := lines[e.startLine-1], lines[e.endLine]
		if !kept {
			edits = append(edits, byteEdit{spanStart, spanEnd, nil})
			continue
		}
		nv := editedVals[i]
		if decodedEqual(e.valNode, nv) {
			continue
		}
		if ed, ok := appendToBlockSequence(block, lines, e, nv, eol); ok {
			edits = append(edits, ed)
			continue
		}
		if ed, ok := spliceInlineScalar(block, lines, e, nv); ok {
			edits = append(edits, ed)
			continue
		}
		rendered, err := renderEntry(e.key, nv, eol)
		if err != nil {
			return nil, false
		}
		edits = append(edits, byteEdit{spanStart, spanEnd, rendered})
	}
	for i := retained; i < len(editedKeys); i++ {
		rendered, err := renderEntry(editedKeys[i], editedVals[i], eol)
		if err != nil {
			return nil, false
		}
		edits = append(edits, byteEdit{len(block), len(block), rendered})
	}

	out := applyEdits(block, edits)
	if !verifySplice(out, edited) {
		return nil, false
	}
	return out, true
}

// lineOffsets returns the byte offset where each line starts, plus a final
// entry at len(block): line n (1-based) spans [lines[n-1], lines[n]).
func lineOffsets(block []byte) []int {
	offs := []int{0}
	for i, c := range block {
		if c == '\n' && i+1 <= len(block) {
			offs = append(offs, i+1)
		}
	}
	if offs[len(offs)-1] != len(block) {
		offs = append(offs, len(block))
	}
	return offs
}

// lineAt returns line n (1-based) without its line ending.
func lineAt(block []byte, lines []int, n int) []byte {
	l := block[lines[n-1]:lines[n]]
	return bytes.TrimSuffix(bytes.TrimSuffix(l, []byte("\n")), []byte("\r"))
}

// locateEntries maps each top-level key of orig to its line span in block. A
// key's span runs from its own line to the line before the next key, minus any
// trailing blank lines and column-1 comments (those separate entries or head
// the next key, and are left untouched). Shapes the splicer does not model are
// rejected: non-scalar or duplicate keys, and keys not at column 1.
func locateEntries(block []byte, lines []int, orig *yaml.Node) ([]fmEntry, bool) {
	total := len(lines) - 1
	seen := map[string]bool{}
	var entries []fmEntry
	for i := 0; i+1 < len(orig.Content); i += 2 {
		k, v := orig.Content[i], orig.Content[i+1]
		if k.Kind != yaml.ScalarNode || k.Column != 1 || k.Line < 1 || k.Line > total || seen[k.Value] {
			return nil, false
		}
		seen[k.Value] = true
		entries = append(entries, fmEntry{key: k.Value, keyNode: k, valNode: v, startLine: k.Line})
	}
	for i := range entries {
		end := total
		if i+1 < len(entries) {
			end = entries[i+1].startLine - 1
		}
		for end > entries[i].startLine {
			l := lineAt(block, lines, end)
			t := bytes.TrimSpace(l)
			if len(t) == 0 || (len(l) > 0 && l[0] == '#') {
				end--
				continue
			}
			break
		}
		if end < entries[i].startLine {
			return nil, false
		}
		entries[i].endLine = end
	}
	return entries, true
}

// mappingPairs returns the keys and values of a mapping, rejecting non-scalar
// and duplicate keys.
func mappingPairs(m *yaml.Node) ([]string, []*yaml.Node, bool) {
	var keys []string
	var vals []*yaml.Node
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind != yaml.ScalarNode || seen[k.Value] {
			return nil, nil, false
		}
		seen[k.Value] = true
		keys = append(keys, k.Value)
		vals = append(vals, m.Content[i+1])
	}
	return keys, vals, true
}

// decodedEqual reports whether two value nodes decode to the same value, so a
// mutation that rewrote a node to an equal value (or only cleared its style)
// leaves the original bytes alone.
func decodedEqual(a, b *yaml.Node) bool {
	var av, bv any
	if err := a.Decode(&av); err != nil {
		return false
	}
	if err := b.Decode(&bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

// spliceInlineScalar replaces just the value bytes of a single-line scalar
// entry, so the key, the alignment padding before the value, and any trailing
// line comment survive. The author's quoting style is kept for the new value
// (a quoted `modified: "…"` stays quoted) unless the mutation chose an explicit
// style itself.
func spliceInlineScalar(block []byte, lines []int, e fmEntry, nv *yaml.Node) (byteEdit, bool) {
	ov := e.valNode
	if ov.Kind != yaml.ScalarNode || nv.Kind != yaml.ScalarNode {
		return byteEdit{}, false
	}
	if e.endLine != e.startLine || ov.Line != e.startLine || ov.Anchor != "" || nv.Anchor != "" {
		return byteEdit{}, false
	}
	switch ov.Style {
	case 0, yaml.SingleQuotedStyle, yaml.DoubleQuotedStyle:
	default:
		return byteEdit{}, false
	}
	line := lineAt(block, lines, e.startLine)
	start, ok := runeColumnToByte(line, ov.Column)
	if !ok || start >= len(line) {
		return byteEdit{}, false
	}
	if line[start] == '!' || line[start] == '&' || line[start] == '*' {
		return byteEdit{}, false // tagged, anchored, or aliased value
	}
	end, ok := scalarEnd(line, start, ov.Style)
	if !ok {
		return byteEdit{}, false
	}

	style := nv.Style
	if style == 0 && (ov.Style == yaml.SingleQuotedStyle || ov.Style == yaml.DoubleQuotedStyle) {
		style = ov.Style
	}
	text, ok := renderInlineScalar(nv, style)
	if !ok {
		return byteEdit{}, false
	}
	if style != nv.Style {
		// Record the kept quoting on the edited tree, so verification (and the
		// re-encode fallback) see the scalar as written: a quoted value is a
		// string, as it was before the edit.
		nv.Style, nv.Tag = style, ""
	}
	base := lines[e.startLine-1]
	return byteEdit{base + start, base + end, text}, true
}

// runeColumnToByte converts a yaml.v3 1-based (rune-counted) column to a byte
// offset within line.
func runeColumnToByte(line []byte, col int) (int, bool) {
	off := 0
	for c := 1; c < col; c++ {
		if off >= len(line) {
			return 0, false
		}
		_, size := utf8.DecodeRune(line[off:])
		off += size
	}
	return off, true
}

// scalarEnd returns the byte offset just past a scalar starting at start. A
// quoted scalar ends at its closing quote; a plain one ends before a ` #`
// comment or the end of the line, with trailing blanks trimmed.
func scalarEnd(line []byte, start int, style yaml.Style) (int, bool) {
	switch style {
	case yaml.DoubleQuotedStyle:
		for i := start + 1; i < len(line); i++ {
			switch line[i] {
			case '\\':
				i++
			case '"':
				return i + 1, true
			}
		}
		return 0, false
	case yaml.SingleQuotedStyle:
		for i := start + 1; i < len(line); i++ {
			if line[i] == '\'' {
				if i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				return i + 1, true
			}
		}
		return 0, false
	}
	end := len(line)
	for i := start; i < len(line); i++ {
		if line[i] == '#' && i > start && (line[i-1] == ' ' || line[i-1] == '\t') {
			end = i
			break
		}
	}
	for end > start && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}
	return end, end > start
}

// renderInlineScalar renders a scalar's YAML text for in-line use, or reports
// false when the encoder needs more than one line for it.
func renderInlineScalar(n *yaml.Node, style yaml.Style) ([]byte, bool) {
	s := &yaml.Node{Kind: yaml.ScalarNode, Tag: n.Tag, Value: n.Value, Style: style}
	if style == yaml.SingleQuotedStyle || style == yaml.DoubleQuotedStyle {
		s.Tag = "" // a quoted scalar is a string; a stale resolved tag would force `!!timestamp "…"`
	}
	b, err := yaml.Marshal(s)
	if err != nil {
		return nil, false
	}
	b = bytes.TrimSuffix(b, []byte("\n"))
	if len(b) == 0 || bytes.ContainsAny(b, "\n\r") {
		return nil, false
	}
	return b, true
}

// appendToBlockSequence handles the one structural edit the writers make inside
// a value: appending items to an existing block sequence (the migrator's
// `sources` fold). The existing items keep their bytes; the new items are
// rendered at the indentation the existing items already use and inserted after
// the last one.
func appendToBlockSequence(block []byte, lines []int, e fmEntry, nv *yaml.Node, eol string) (byteEdit, bool) {
	ov := e.valNode
	if ov.Kind != yaml.SequenceNode || nv.Kind != yaml.SequenceNode || ov.Style&yaml.FlowStyle != 0 {
		return byteEdit{}, false
	}
	if len(ov.Content) == 0 || len(nv.Content) <= len(ov.Content) || ov.Line <= e.startLine {
		return byteEdit{}, false
	}
	for i := range ov.Content {
		if !decodedEqual(ov.Content[i], nv.Content[i]) {
			return byteEdit{}, false
		}
	}
	first := lineAt(block, lines, ov.Content[0].Line)
	indent := 0
	for indent < len(first) && first[indent] == ' ' {
		indent++
	}
	if indent >= len(first) || first[indent] != '-' {
		return byteEdit{}, false
	}

	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: nv.Content[len(ov.Content):]}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(seq); err != nil {
		return byteEdit{}, false
	}
	_ = enc.Close()

	pad := bytes.Repeat([]byte(" "), indent)
	var out bytes.Buffer
	for _, l := range bytes.SplitAfter(buf.Bytes(), []byte("\n")) {
		if len(l) == 0 {
			continue
		}
		body := bytes.TrimSuffix(l, []byte("\n"))
		if len(body) > 0 {
			out.Write(pad)
		}
		out.Write(body)
		out.WriteString(eol)
	}
	at := lines[e.endLine]
	if at > 0 && block[at-1] != '\n' {
		// The block's final line has no newline (cannot happen behind a closing
		// fence, but never splice onto the end of a line).
		return byteEdit{}, false
	}
	return byteEdit{at, at, out.Bytes()}, true
}

// renderEntry renders one `key: value` entry with the writers' encoder settings.
// Only head/foot comments are dropped from the copy: they sit on lines outside
// the replaced span, which the splice keeps verbatim.
func renderEntry(key string, v *yaml.Node, eol string) ([]byte, error) {
	vc := *v
	vc.HeadComment, vc.FootComment = "", ""
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key},
		&vc,
	}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	_ = enc.Close()
	b := buf.Bytes()
	if eol != "\n" {
		b = bytes.ReplaceAll(b, []byte("\n"), []byte(eol))
	}
	return b, nil
}

// applyEdits applies non-overlapping edits to block. Inserts at the same offset
// keep their submission order.
func applyEdits(block []byte, edits []byteEdit) []byte {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	pos := 0
	for _, ed := range edits {
		out.Write(block[pos:ed.start])
		out.Write(ed.repl)
		pos = ed.end
	}
	out.Write(block[pos:])
	return out.Bytes()
}

// verifySplice confirms the spliced block parses as a mapping that decodes to
// the same value, with the same top-level key order, as the mutated tree.
func verifySplice(spliced []byte, edited *yaml.Node) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(spliced, &doc); err != nil {
		return false
	}
	got := frontmatterMapping(&doc)
	if got == nil {
		return false
	}
	gk, _, ok := mappingPairs(got)
	if !ok {
		return false
	}
	ek, _, ok := mappingPairs(edited)
	if !ok || !reflect.DeepEqual(gk, ek) {
		return false
	}
	return decodedEqual(got, edited)
}
