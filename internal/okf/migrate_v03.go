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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Migration from OKF v0.2 (or an undeclared version) to v0.3. v0.3 §13.1 lists
// two breaking changes. Only one changes what a producer should write:
//
//   - A link or path field without a leading `/` resolves from the directory of
//     its file (§6, §6.2). A legacy bundle keeps a bundle-root fallback
//     (§12.1), and a bundle that declares 0.3 does not. So before the version
//     marker moves to 0.3, every value that resolves ONLY through that fallback
//     is rewritten to the `/`-rooted form (§6 rule 2), which resolves to the
//     same target under every version.
//   - Only current verification events count toward the trust tier (§5.3).
//     That is a reading rule with no producer edit.
//
// §5 also says a timestamp is an RFC 3339 date-time with an offset, or a
// full-date, and "a date-time without an offset stays invalid" (§13.2). A value
// whose instant is fixed by its own text (it carries an offset) but is spelled
// another way is rewritten to the RFC 3339 form. A date-time without an offset
// names no instant, so it is never guessed; it becomes a judgment item.

// Version actions a plan can carry (MigratePlan.VersionAction).
const (
	// VersionBump: the bundle declares 0.2 or earlier, or no version (§12.1).
	// The plan's edits apply, and the version markers move to the target.
	VersionBump = "bump"
	// VersionHold: the bundle is legacy, but the plan leaves judgment items
	// that a consumer reads only through a §12.1 fallback (Appendix B
	// `# Citations` or `timestamp`). The edits apply; the declaration waits
	// until those items are resolved, or until ForceDeclare overrides it.
	VersionHold = "hold"
	// VersionKeep: the bundle declares the target version, a later one, or one
	// this build does not know. §12: such a bundle is consumed best-effort and
	// never refused, so migrate plans nothing and writes nothing.
	VersionKeep = "keep"
)

// Judgment kinds for §5 timestamps that cannot be rewritten without guessing.
const (
	// JudgmentTimestampNoOffset: a date-time with no UTC offset. §5/§13.2: it
	// is invalid and names no instant; only the author knows the offset.
	JudgmentTimestampNoOffset JudgmentKind = "timestamp-no-offset"
	// JudgmentTimestampInvalid: a value at a §5 timestamp key that is not a
	// date or date-time at all.
	JudgmentTimestampInvalid JudgmentKind = "timestamp-invalid"
)

// PathEdit is one planned §6 rewrite of a value that resolves only through the
// §12.1 bundle-root fallback. Field is "link" for a body link, otherwise the
// path field name (e.g. "computation", "sources[0].resource"). Old is the
// value as written (for a link, the URL without its title); New resolves to
// the same target with a leading `/`.
type PathEdit struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// TimestampEdit is one planned §5 rewrite of a timestamp to the RFC 3339 form.
// Key names the field (e.g. "generated.at", "verified[1].at").
type TimestampEdit struct {
	Key string `json:"key"`
	Old string `json:"old"`
	New string `json:"new"`
}

// planV03Node computes the v0.3 edits for one file: every body link and path
// field that resolves only through the §12.1 fallback, and every §5 timestamp
// that can be rewritten without guessing. Judgment items go to judg.
func planV03Node(b *Bundle, rel string, n *Node, raw []byte, judg *[]JudgmentItem) ([]PathEdit, []TimestampEdit) {
	var paths []PathEdit
	seen := map[string]bool{}
	for _, l := range scanRawLinks(n.Body) {
		r := b.ResolveLink(rel, l.url)
		if r.Kind != RefBundle || r.Form != FormRootFallback || seen[l.url] {
			continue
		}
		seen[l.url] = true
		paths = append(paths, PathEdit{Field: "link", Old: l.url, New: rootedURL(l.url, r.Path)})
	}
	if n.Frontmatter != nil {
		for _, f := range n.PathFields() {
			r := b.ResolvePathField(rel, f.Value)
			if r.Kind == RefBundle && r.Form == FormRootFallback {
				paths = append(paths, PathEdit{Field: f.Field, Old: f.Value, New: rootedURL(f.Value, r.Path)})
			}
		}
	}
	var stamps []TimestampEdit
	for _, ts := range rawSection5Timestamps(raw) {
		if _, ok := parseOKFTimestampLocal(ts.value); ok {
			continue
		}
		if norm, ok := normalizeTimestamp(ts.value); ok {
			stamps = append(stamps, TimestampEdit{Key: ts.key, Old: ts.value, New: norm})
			continue
		}
		*judg = append(*judg, timestampJudgment(rel, ts.key, ts.value))
	}
	return paths, stamps
}

func timestampJudgment(rel, key, value string) JudgmentItem {
	kind := JudgmentTimestampInvalid
	if noOffsetDateTimeRe.MatchString(value) {
		kind = JudgmentTimestampNoOffset
	}
	return JudgmentItem{Path: rel, Kind: kind, Context: key + " = " + value}
}

// rootedURL rewrites a value to the §6 rule 2 form for the bundle-relative
// target, keeping any #fragment or ?query the author wrote.
func rootedURL(value, target string) string {
	suffix := ""
	if i := strings.IndexAny(value, "#?"); i >= 0 {
		suffix = value[i:]
	}
	return "/" + escapePath(target) + suffix
}

var (
	// offsetDateTimeRe is a date-time whose instant its own text fixes: a
	// date, "T" or a space, hh:mm with optional seconds and fraction, and an
	// offset (Z, ±hh:mm, ±hhmm, or ±hh), optionally after a space.
	offsetDateTimeRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})[Tt ](\d{2}):(\d{2})(?::(\d{2})(\.\d+)?)? ?([Zz]|[+-]\d{2}(?::?\d{2})?)$`)
	// noOffsetDateTimeRe is a date-time with no offset (§13.2: invalid).
	noOffsetDateTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?$`)
)

// normalizeTimestamp rewrites a timestamp that names one instant (it carries
// an offset) to the RFC 3339 date-time form §5 requires. The author's offset
// is kept, only respelled (+0000 becomes +00:00; Z stays Z), so no instant
// moves. ok is false when the value has no offset or is not a date-time.
func normalizeTimestamp(s string) (string, bool) {
	m := offsetDateTimeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	sec := m[4]
	if sec == "" {
		sec = "00"
	}
	off := strings.ToUpper(m[6])
	if off != "Z" {
		digits := strings.ReplaceAll(off[1:], ":", "")
		if len(digits) == 2 {
			digits += "00"
		}
		off = off[:1] + digits[:2] + ":" + digits[2:]
	}
	out := m[1] + "T" + m[2] + ":" + m[3] + ":" + sec + m[5] + off
	if _, ok := parseOKFTimestampLocal(out); !ok {
		return "", false // e.g. month 13: not a real instant
	}
	return out, true
}

// rawTimestamp is one §5 timestamp value as written in a file's frontmatter.
type rawTimestamp struct {
	key   string
	value string
}

// rawSection5Timestamps returns the §5 timestamp values of a file as their
// source text, in a fixed order: generated.at, verified[].at, stale_after,
// sources[].last_modified and sources[].usage_window, usage_window. Reading
// the yaml.Node keeps the text yaml.v3 would otherwise resolve (and lose the
// missing offset of). Null and empty values are absent (§11.2 rule 1).
func rawSection5Timestamps(raw []byte) []rawTimestamp {
	m := parsedFrontmatterMapping(raw)
	if m == nil {
		return nil
	}
	var out []rawTimestamp
	for _, s := range section5TimestampNodes(m) {
		v := s.node
		if v.Kind != yaml.ScalarNode || v.Value == "" || v.ShortTag() == "!!null" {
			continue
		}
		out = append(out, rawTimestamp{key: s.key, value: v.Value})
	}
	return out
}

type keyedNode struct {
	key  string
	node *yaml.Node
}

// section5TimestampNodes lists the §5 timestamp value nodes of a frontmatter
// mapping with their field names, the same key set lintTimestampForms and
// demoteInvalidNativeTimestamps use.
func section5TimestampNodes(m *yaml.Node) []keyedNode {
	var out []keyedNode
	add := func(key string, n *yaml.Node) {
		if n != nil {
			out = append(out, keyedNode{key, n})
		}
	}
	window := func(prefix string, n *yaml.Node) {
		add(prefix+".from", mappingValue(n, "from"))
		add(prefix+".to", mappingValue(n, "to"))
	}
	add("generated.at", mappingValue(mappingValue(m, "generated"), "at"))
	for i, ev := range itemsOf(mappingValue(m, "verified")) {
		add(fmt.Sprintf("verified[%d].at", i), mappingValue(ev, "at"))
	}
	add("stale_after", mappingValue(m, "stale_after"))
	for i, src := range itemsOf(mappingValue(m, "sources")) {
		add(fmt.Sprintf("sources[%d].last_modified", i), mappingValue(src, "last_modified"))
		window(fmt.Sprintf("sources[%d].usage_window", i), mappingValue(src, "usage_window"))
	}
	window("usage_window", mappingValue(m, "usage_window"))
	return out
}

func parsedFrontmatterMapping(raw []byte) *yaml.Node {
	block, _, ok := splitFrontmatterRaw(raw)
	if !ok {
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return nil
	}
	return frontmatterMapping(&doc)
}

// fieldNode returns the value node a field name such as "computation",
// "sources[1].resource", or "generated.at" names, or nil. A list index on a
// single mapping addresses it as a one-item list (§11.2 rule 2).
func fieldNode(m *yaml.Node, field string) *yaml.Node {
	cur := m
	for _, seg := range strings.Split(field, ".") {
		key, idx := seg, -1
		if i := strings.IndexByte(seg, '['); i >= 0 && strings.HasSuffix(seg, "]") {
			n, err := strconv.Atoi(seg[i+1 : len(seg)-1])
			if err != nil {
				return nil
			}
			key, idx = seg[:i], n
		}
		cur = mappingValue(cur, key)
		if cur == nil {
			return nil
		}
		if idx >= 0 {
			items := itemsOf(cur)
			if idx >= len(items) {
				return nil
			}
			cur = items[idx]
		}
	}
	return cur
}

// renderV03Node returns raw with a node's planned v0.3 edits applied: body
// links retargeted (fragment, query, title, and <angle> wrapping kept), and
// path fields and timestamps rewritten in the frontmatter. Each change is a
// byte splice of just that value (§4: values that did not change are not
// re-serialized). A planned edit whose old value is no longer present is
// skipped, so re-applying a plan is a no-op.
func renderV03Node(b *Bundle, rel string, raw []byte, nm NodeMigration) ([]byte, error) {
	out := raw
	var fieldEdits []scalarChange
	links := map[string]string{}
	for _, p := range nm.Paths {
		if p.Field == "link" {
			links[p.Old] = p.New
		}
	}
	if len(links) > 0 {
		out = retargetBodyLinks(b, rel, out, links)
	}
	pick := func(m *yaml.Node) []scalarChange {
		fieldEdits = fieldEdits[:0]
		for _, p := range nm.Paths {
			if p.Field == "link" {
				continue
			}
			if n := fieldNode(m, p.Field); n != nil && n.Kind == yaml.ScalarNode && n.Value == p.Old {
				fieldEdits = append(fieldEdits, scalarChange{node: n, value: p.New})
			}
		}
		for _, ts := range nm.Timestamps {
			if n := fieldNode(m, ts.Key); n != nil && n.Kind == yaml.ScalarNode && n.Value == ts.Old {
				// The spec's examples quote timestamps (§5); an author's
				// single quotes are kept.
				style := yaml.DoubleQuotedStyle
				if n.Style == yaml.SingleQuotedStyle {
					style = yaml.SingleQuotedStyle
				}
				fieldEdits = append(fieldEdits, scalarChange{node: n, value: ts.New, style: style})
			}
		}
		return fieldEdits
	}
	if len(nm.Paths) > len(links) || len(nm.Timestamps) > 0 {
		var err error
		out, err = rewriteScalars(out, pick)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// retargetBodyLinks rewrites every link in the body of raw whose URL is a key
// of links and still resolves through the §12.1 fallback. Offsets are scanned
// on the body only, so a `[x](y)` inside a frontmatter string is never edited.
func retargetBodyLinks(b *Bundle, rel string, raw []byte, links map[string]string) []byte {
	start := 0
	if _, rawAfter, ok := splitFrontmatterRaw(raw); ok {
		start = len(raw) - len(rawAfter)
	}
	body := string(raw[start:])
	var buf strings.Builder
	pos := 0
	for _, l := range scanRawLinks(body) {
		nu, ok := links[l.url]
		if !ok || b.ResolveLink(rel, l.url).Form != FormRootFallback {
			continue
		}
		// nu already carries the fragment/query; retarget re-adds the one in
		// l.url, so pass the bare path.
		bare := nu
		if i := strings.IndexAny(bare, "#?"); i >= 0 {
			bare = bare[:i]
		}
		buf.WriteString(body[pos:l.capStart])
		buf.WriteString(l.retarget(bare))
		pos = l.capEnd
	}
	if pos == 0 {
		return raw
	}
	buf.WriteString(body[pos:])
	out := append([]byte(nil), raw[:start]...)
	return append(out, buf.String()...)
}

// scalarChange sets one scalar node to value. style 0 keeps the author's
// quoting style (plain stays plain where the value allows it).
type scalarChange struct {
	node  *yaml.Node
	value string
	style yaml.Style
}

// rewriteScalars applies the scalar changes pick returns. It first splices just
// the bytes of each value (any depth, block or flow), and verifies that the
// result decodes to the edited tree. When a value has a shape the splice does
// not model (multi-line, anchored, tagged), it falls back to editFrontmatter,
// which keeps every untouched top-level entry. pick is called once per parse,
// so it must be deterministic.
func rewriteScalars(raw []byte, pick func(*yaml.Node) []scalarChange) ([]byte, error) {
	head, block, tail, ok := splitFrontmatterVerbatim(raw)
	if !ok {
		return raw, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	m := frontmatterMapping(&doc)
	if m == nil {
		return nil, errFrontmatterNotMapping
	}
	changes := pick(m)
	if len(changes) == 0 {
		return raw, nil
	}
	if spliced, ok := spliceScalarValues(block, m, changes); ok {
		out := append([]byte(nil), head...)
		out = append(out, spliced...)
		return append(out, tail...), nil
	}
	return editFrontmatter(raw, func(em *yaml.Node) {
		for _, c := range pick(em) {
			setChange(c)
		}
	})
}

func setChange(c scalarChange) {
	c.node.Kind = yaml.ScalarNode
	c.node.Tag = ""
	c.node.Value = c.value
	if c.style != 0 {
		c.node.Style = c.style
	}
}

// spliceScalarValues computes the byte span of each changed value from its
// yaml.Node position, replaces it, and verifies the result. Spans come from
// the unmodified tree; the tree is edited only afterwards, for verification.
func spliceScalarValues(block []byte, m *yaml.Node, changes []scalarChange) ([]byte, bool) {
	lines := lineOffsets(block)
	flow := flowScalars(m, false, map[*yaml.Node]bool{})
	var edits []byteEdit
	for _, c := range changes {
		n := c.node
		if n.Anchor != "" || n.Line < 1 || n.Line >= len(lines) {
			return nil, false
		}
		switch n.Style {
		case 0, yaml.SingleQuotedStyle, yaml.DoubleQuotedStyle:
		default:
			return nil, false
		}
		line := lineAt(block, lines, n.Line)
		start, ok := runeColumnToByte(line, n.Column)
		if !ok || start >= len(line) || strings.ContainsRune("!&*", rune(line[start])) {
			return nil, false
		}
		end, ok := scalarEnd(line, start, n.Style)
		if !ok {
			return nil, false
		}
		if n.Style == 0 && flow[n] {
			if i := bytes.IndexAny(line[start:end], ",]}"); i >= 0 {
				end = start + i
			}
			for end > start && (line[end-1] == ' ' || line[end-1] == '\t') {
				end--
			}
		}
		if !sourceMatches(line[start:end], n) {
			return nil, false // multi-line or otherwise unmodeled scalar
		}
		style := c.style
		if style == 0 {
			style = n.Style
		}
		text, ok := renderInlineScalar(&yaml.Node{Kind: yaml.ScalarNode, Value: c.value}, style)
		if !ok {
			return nil, false
		}
		base := lines[n.Line-1]
		edits = append(edits, byteEdit{base + start, base + end, text})
	}
	for _, c := range changes {
		setChange(c)
	}
	out := applyEdits(block, edits)
	if !verifySplice(out, m) {
		return nil, false
	}
	return out, true
}

// sourceMatches reports whether src, parsed on its own as a YAML scalar, is
// exactly the node's value, which proves the span covers the whole scalar.
func sourceMatches(src []byte, n *yaml.Node) bool {
	if n.Style == 0 {
		return string(src) == n.Value
	}
	var v yaml.Node
	if err := yaml.Unmarshal(src, &v); err != nil || len(v.Content) != 1 {
		return false
	}
	return v.Content[0].Kind == yaml.ScalarNode && v.Content[0].Value == n.Value
}

// flowScalars marks the scalars that sit inside a flow collection, where a
// plain scalar also ends at `,`, `]`, or `}`.
func flowScalars(n *yaml.Node, inFlow bool, out map[*yaml.Node]bool) map[*yaml.Node]bool {
	if n.Kind == yaml.ScalarNode {
		if inFlow {
			out[n] = true
		}
		return out
	}
	f := inFlow || n.Style&yaml.FlowStyle != 0
	for _, c := range n.Content {
		flowScalars(c, f, out)
	}
	return out
}

// planVersionAction decides whether a bundle is migrated (§12.1 legacy:
// declares 0.2 or earlier, or nothing) or kept as is (§12: the target or a
// version this build does not know).
func planVersionAction(b *Bundle) string {
	if legacyVersion(b.DeclaredOkfVersion) {
		return VersionBump
	}
	return VersionKeep
}

// declareRootIndexVersion writes okf_version target on the bundle-root
// index.md (§8.1: "Producers SHOULD put a bundle-root index.md with
// okf_version in each bundle"). An existing marker is respelled to the quoted
// target; a frontmatter block without one gets the key appended; an index
// with no frontmatter gets an okf_version-only block. Every other byte is
// kept, including CRLF line endings. A missing index.md is left missing.
func declareRootIndexVersion(root, target string) ([]byte, []byte, error) {
	abs := filepath.Join(root, "index.md")
	if escapesBundleRoot(root, abs) { // §3.2 rule 5, §11.2
		return nil, nil, fmt.Errorf("%s is a symlink that resolves outside the bundle root; not read (OKF §3.2, §11.2)", "index.md")
	}
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle index.md
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read index.md: %w", err)
	}
	if _, _, ok := splitFrontmatterRaw(raw); !ok {
		eol := "\n"
		if bytes.Contains(raw, []byte("\r\n")) {
			eol = "\r\n"
		}
		blk := "---" + eol + "okf_version: " + strconv.Quote(target) + eol + "---" + eol + eol
		return raw, append([]byte(blk), raw...), nil
	}
	var doc yaml.Node
	block, _, _ := splitFrontmatterRaw(raw)
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse index.md frontmatter: %w", err)
	}
	m := frontmatterMapping(&doc)
	if m == nil {
		return raw, raw, nil // not a mapping: validate reports it; leave it
	}
	if v := mappingValue(m, "okf_version"); v != nil && v.Kind == yaml.ScalarNode && v.Value == target {
		return raw, raw, nil // already declared (idempotent)
	}
	out, err := editFrontmatter(raw, func(m *yaml.Node) {
		setScalar(m, "okf_version", target)
		mappingValue(m, "okf_version").Style = yaml.DoubleQuotedStyle
	})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite index.md okf_version: %w", err)
	}
	return raw, out, nil
}

// sortedLinkSourcePaths returns the bundle's link sources (concepts and §3.2
// rule 2 non-concepts) in path order.
func sortedLinkSourcePaths(b *Bundle) []string {
	src := b.linkSources()
	out := make([]string, 0, len(src))
	for p := range src {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
