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
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// linkRefDefRe matches a CommonMark link reference definition
// (`[label]: <url> "title"`), the target of a `[text][label]` link.
var linkRefDefRe = regexp.MustCompile(`^\s*\[[^\]]+\]:\s*<?\S+>?(\s+("[^"]*"|'[^']*'|\([^)]*\)))?\s*$`)

// A `# Citations` entry whose resource is already listed in frontmatter
// `sources` is not added again (idempotence). When that source has no §5.1
// `title`, the name of the cited work (citationName) becomes its title. §5.1
// calls title "a label for humans", so it carries the name only. An entry that
// marks no name leaves the title unset and becomes a judgment item, and a
// source that already has a different title keeps it and the entry is
// reported. The entry's full text is never written into a title.

// sourceTitle is the `title` of a source already in frontmatter. has is false
// when the entry has no title: the key is absent, null, or empty (§11.2 rule
// 1 reads a null or empty value as an absent key). resource is the source's
// resource as the frontmatter spells it, the key a title edit names.
type sourceTitle struct {
	resource string
	title    string
	has      bool
}

// lookupSourceTitle finds the existing source for res, matching a DOI URL
// however it is spelled (canonicalResource).
func lookupSourceTitle(existing map[string]sourceTitle, res string) (sourceTitle, bool) {
	if cur, ok := existing[res]; ok {
		return cur, true
	}
	cur, ok := existing[canonicalResource(res)]
	return cur, ok
}

// existingSourceTitles maps each resource already present in a node's
// frontmatter `sources` (§5.1) to its title. When a resource is listed more
// than once, the first entry wins; the renderer edits that same entry.
func existingSourceTitles(n *Node) map[string]sourceTitle {
	out := map[string]sourceTitle{}
	raw, _ := n.Frontmatter["sources"].([]any)
	for _, item := range raw {
		m, ok := asStringMap(item)
		if !ok {
			continue
		}
		res := stringField(m, "resource")
		if _, seen := out[res]; res == "" || seen {
			continue
		}
		v, present := m["title"]
		st := sourceTitle{resource: res, title: stringField(m, "title"), has: present && v != nil && v != ""}
		out[res] = st
		if k := canonicalResource(res); k != res {
			if _, seen := out[k]; !seen {
				out[k] = st
			}
		}
	}
	return out
}

// entryText is a citation entry without its list marker: the form an earlier
// migration wrote as a title.
func entryText(entry string) string {
	text := citationMarkerRe.ReplaceAllString(strings.TrimSpace(entry), "")
	return strings.TrimSpace(orderedMarkerRe.ReplaceAllString(text, ""))
}

// planSourceTitle handles a citation entry whose resource is already in
// frontmatter `sources`; name is the work's name (citationName), or "".
//
//   - A link reference definition, or an entry with nothing beyond its
//     resource, has no name to give: nothing is planned.
//   - An untitled source gets name as its title, or, with no name, a
//     JudgmentSourceTitleUnnamed item.
//   - A titled source keeps its title. It is reported as a
//     JudgmentSourceTitleConflict only when the entry names a different work.
//     A title equal to the name, or to this entry's full text or the head or
//     tail of it (a title an earlier migration wrote), is not a conflict;
//     rewriting such titles to the name is a separate change.
func planSourceTitle(plan *MigratePlan, nm *NodeMigration, existing map[string]sourceTitle, rel, entry string, cur sourceTitle, name string) {
	if linkRefDefRe.MatchString(entry) || !citationHasProse(entry) {
		return
	}
	switch {
	case !cur.has && name != "":
		nm.Titles = append(nm.Titles, SourceTitleEdit{Resource: cur.resource, Title: name})
		titled := sourceTitle{resource: cur.resource, title: name, has: true}
		for k, v := range existing {
			if v.resource == cur.resource {
				existing[k] = titled
			}
		}
	case !cur.has:
		plan.Judgment = append(plan.Judgment, JudgmentItem{
			Path: rel, Kind: JudgmentSourceTitleUnnamed, Context: entry,
		})
	case name != "" && cur.title != name && !isEntryText(cur.title, entry):
		plan.Judgment = append(plan.Judgment, JudgmentItem{
			Path: rel, Kind: JudgmentSourceTitleConflict, Context: entry,
		})
	}
}

// isEntryText reports whether title is this entry's own text, or the head or
// tail of it: an earlier migration wrote the entry text as the title, and one
// that split the entry at a wrapped line wrote only a piece of it. Whitespace
// is compared collapsed, because the entry is joined from wrapped lines.
func isEntryText(title, entry string) bool {
	t, e := strings.Join(strings.Fields(title), " "), strings.Join(strings.Fields(entryText(entry)), " ")
	if t == "" {
		return false
	}
	return t == e || strings.HasPrefix(e, t) || strings.HasSuffix(e, t)
}

// firstSourceItem returns the first `sources` item (a mapping) whose
// `resource` scalar is res, or nil. A single mapping counts as a one-item
// list (§11.2 rule 2).
func firstSourceItem(m *yaml.Node, res string) *yaml.Node {
	for _, s := range itemsOf(mappingValue(m, "sources")) {
		if r := mappingValue(s, "resource"); r != nil && r.Kind == yaml.ScalarNode && r.Value == res {
			return s
		}
	}
	return nil
}

// untitled reports whether a sources item has no title (absent, null, or
// empty; §11.2 rule 1).
func untitled(item *yaml.Node) bool {
	t := mappingValue(item, "title")
	return t == nil || (t.Kind == yaml.ScalarNode && (t.Tag == "!!null" || t.Value == ""))
}

// applySourceTitles sets each planned title on the first matching source that
// is still untitled in raw, and returns the new file bytes. Every edit
// re-checks the current frontmatter, so a stale plan never overwrites a title
// the author has since written, and a re-applied plan changes nothing.
//
// OKF v0.3 §4 (SHOULD NOT re-serialize values that it did not change): the
// edit inserts one `title:` line under the source's `resource:` line and
// keeps every other byte. The insert is verified to decode to the intended
// tree; a shape it does not model (a flow-style item, a multi-line resource,
// a null title key to replace) goes through editFrontmatter instead.
func applySourceTitles(raw []byte, titles []SourceTitleEdit) ([]byte, error) {
	m := parsedFrontmatterMapping(raw)
	if m == nil {
		return raw, nil
	}
	var todo []SourceTitleEdit
	for _, t := range titles {
		if item := firstSourceItem(m, t.Resource); item != nil && untitled(item) {
			todo = append(todo, t)
			setSourceTitle(item, t.Title) // later edits for the same resource see it titled
		}
	}
	if len(todo) == 0 {
		return raw, nil
	}
	mutate := func(root *yaml.Node) {
		for _, t := range todo {
			setSourceTitle(firstSourceItem(root, t.Resource), t.Title)
		}
	}
	lines := make([]sourceKeyLine, len(todo))
	for i, t := range todo {
		lines[i] = sourceKeyLine{resource: t.Resource, value: scalarValue(t.Title)}
	}
	if out, ok := insertSourceKeyLines(raw, "title", lines, mutate); ok {
		return out, nil
	}
	return editFrontmatter(raw, mutate)
}

// setSourceTitle sets (or replaces) the title of a sources item.
func setSourceTitle(item *yaml.Node, title string) {
	for i := 0; i+1 < len(item.Content); i += 2 {
		if item.Content[i].Value == "title" {
			item.Content[i+1] = scalarValue(title)
			return
		}
	}
	item.Content = append(item.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "title"}, scalarValue(title))
}

// sourceKeyLine is one `key: value` line to insert under a source's resource.
type sourceKeyLine struct {
	resource string
	value    *yaml.Node
}

// insertSourceKeyLines inserts a `<key>: <value>` line directly under the
// `resource:` line of each target item, at the resource key's indent. ok is
// false when any target is not a block mapping with a single-line resource
// and no such key, or when the result does not decode to the tree mutate
// builds.
func insertSourceKeyLines(raw []byte, key string, todo []sourceKeyLine, mutate func(*yaml.Node)) ([]byte, bool) {
	head, block, tail, ok := splitFrontmatterVerbatim(raw)
	if !ok {
		return nil, false
	}
	var doc, editDoc yaml.Node
	if yaml.Unmarshal(block, &doc) != nil || yaml.Unmarshal(block, &editDoc) != nil {
		return nil, false
	}
	m, edited := frontmatterMapping(&doc), frontmatterMapping(&editDoc)
	if m == nil || edited == nil {
		return nil, false
	}
	mutate(edited)
	lines := lineOffsets(block)
	eol := "\n"
	if bytes.Contains(block, []byte("\r\n")) {
		eol = "\r\n"
	}
	var edits []byteEdit
	for _, t := range todo {
		item := firstSourceItem(m, t.resource)
		if item == nil || item.Style&yaml.FlowStyle != 0 || mappingValue(item, key) != nil {
			return nil, false
		}
		var rkey, val *yaml.Node
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value == "resource" {
				rkey, val = item.Content[i], item.Content[i+1]
			}
		}
		switch val.Style {
		case 0, yaml.SingleQuotedStyle, yaml.DoubleQuotedStyle:
		default:
			return nil, false
		}
		if val.Line != rkey.Line || rkey.Line < 1 || rkey.Line >= len(lines) {
			return nil, false
		}
		text, ok := renderInlineScalar(t.value, t.value.Style)
		if !ok {
			return nil, false
		}
		at := lines[rkey.Line]
		if at == 0 || block[at-1] != '\n' {
			return nil, false
		}
		line := append(bytes.Repeat([]byte(" "), rkey.Column-1), key+": "...)
		line = append(append(line, text...), eol...)
		edits = append(edits, byteEdit{at, at, line})
	}
	out := applyEdits(block, edits)
	if !verifySplice(out, edited) {
		return nil, false
	}
	var buf bytes.Buffer
	buf.Write(head)
	buf.Write(out)
	buf.Write(tail)
	return buf.Bytes(), true
}

// applySourceIDs sets each planned id on the first source with that resource
// that still has no id, and returns the new file bytes. Like
// applySourceTitles it re-checks the current frontmatter, so a re-applied
// plan changes nothing, and it inserts one `id:` line under the resource
// line (§4: unchanged values are not re-serialized). §4: a number-looking id
// is quoted (idScalar).
func applySourceIDs(raw []byte, ids []SourceIDEdit) ([]byte, error) {
	m := parsedFrontmatterMapping(raw)
	if m == nil || len(ids) == 0 {
		return raw, nil
	}
	var todo []sourceKeyLine
	for _, e := range ids {
		if item := firstSourceItem(m, e.Resource); item != nil && mappingValue(item, "id") == nil {
			todo = append(todo, sourceKeyLine{resource: e.Resource, value: idScalar(e.ID)})
			setSourceKey(item, "id", idScalar(e.ID))
		}
	}
	if len(todo) == 0 {
		return raw, nil
	}
	mutate := func(root *yaml.Node) {
		for _, t := range todo {
			setSourceKey(firstSourceItem(root, t.resource), "id", t.value)
		}
	}
	if out, ok := insertSourceKeyLines(raw, "id", todo, mutate); ok {
		return out, nil
	}
	return editFrontmatter(raw, mutate)
}

// setSourceKey sets (or replaces) one key of a sources item.
func setSourceKey(item *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(item.Content); i += 2 {
		if item.Content[i].Value == key {
			item.Content[i+1] = v
			return
		}
	}
	item.Content = append(item.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, v)
}
