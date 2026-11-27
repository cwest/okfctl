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
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// An earlier migration wrote a whole `# Citations` entry into a source's
// `title`. v0.3 §5.1 calls title "a label for humans": the work's name, not
// the entry. Per-claim attribution is a footnote whose label is the source's
// id (§4.2), and consumers do not parse the footnote text, so the entry's
// full text belongs in the footnote definition. Once the footnote holds that
// text, the title is shortened to the name (citationName), or removed when
// the entry names no work (title is optional) and reported as
// JudgmentSourceTitleUnnamed.
//
// A title is never shortened unless its text is in the footnote: the planner
// checks the footnote this run writes or the one already in the body, and the
// renderer checks the bytes it is about to write. A title whose text is in no
// footnote is kept and reported as JudgmentSourceTitleUnpreserved. Every
// other key of the source (`note`, `x-` keys) keeps its bytes (§4.1).

// JudgmentSourceTitleUnpreserved: a source whose title holds its `# Citations`
// entry, but no footnote definition with the source's id holds that text (the
// node's ids were refused, or the author's own footnote says something else).
// The title is kept. The resource is in frontmatter, so this does not hold the
// version declaration.
const JudgmentSourceTitleUnpreserved JudgmentKind = "source-title-unpreserved"

// SourceRetitleEdit replaces a `title` that holds the full citation entry.
// Old is the title as written, Title the work's name ("" removes the title),
// and Footnote the id whose footnote definition must hold Old's text before
// the edit is applied.
type SourceRetitleEdit struct {
	Resource string `json:"resource"`
	Old      string `json:"old"`
	Title    string `json:"title,omitempty"`
	Footnote string `json:"footnote"`
}

// retitleCandidate is an entry whose existing source's title holds the
// entry's text; name is the work's name, or "".
type retitleCandidate struct {
	entry int
	cur   sourceTitle
	name  string
}

// collapseSpace joins s's whitespace runs into single spaces.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// titlePreserved reports whether a footnote definition text holds title's
// text: the title, with whitespace collapsed, is part of it. A leading `[id]`
// on the title is the key that became the footnote label, so it is not
// required in the text.
func titlePreserved(title, id, def string) bool {
	t, d := collapseSpace(title), collapseSpace(def)
	if t == "" || d == "" {
		return false
	}
	if strings.Contains(d, t) {
		return true
	}
	rest, ok := strings.CutPrefix(t, "["+id+"]")
	rest = strings.TrimSpace(rest)
	return ok && rest != "" && strings.Contains(d, rest)
}

// footnoteDefLineRe matches a footnote definition and captures its label and
// first line of text.
var footnoteDefLineRe = regexp.MustCompile(`^ {0,3}\[\^([^\]\s]+)\]:[ \t]?(.*)$`)

// footnoteDefinitions maps each footnote label in body, folded, to its text:
// the first line and any indented continuation lines, joined by spaces. The
// first definition of a label wins, as in GFM.
func footnoteDefinitions(body string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		m := footnoteDefLineRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		text := []string{m[2]}
		for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], "    ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			text = append(text, strings.TrimSpace(lines[i]))
		}
		if k := strings.ToLower(m[1]); out[k] == "" {
			out[k] = strings.Join(text, " ")
		}
	}
	return out
}

// planRetitles plans a SourceRetitleEdit for each candidate whose entry got a
// footnote id (ids maps entry index to it) and whose footnote, planned in
// nm.Footnotes or already in the body, holds the title's text. Any other
// candidate is reported, and its title is kept.
func planRetitles(plan *MigratePlan, nm *NodeMigration, n *Node, rel string, entries []string, ids map[int]string, cands []retitleCandidate) {
	if len(cands) == 0 {
		return
	}
	planned := map[string]string{}
	for _, f := range nm.Footnotes {
		planned[strings.ToLower(f.ID)] = f.Text
	}
	defs := footnoteDefinitions(n.Body)
	for _, c := range cands {
		entry := entries[c.entry]
		id := ids[c.entry]
		def, ok := planned[strings.ToLower(id)]
		if !ok {
			def = defs[strings.ToLower(id)]
		}
		if id == "" || !titlePreserved(c.cur.title, id, def) {
			plan.Judgment = append(plan.Judgment, JudgmentItem{
				Path: rel, Kind: JudgmentSourceTitleUnpreserved, Context: entry,
			})
			continue
		}
		nm.Retitles = append(nm.Retitles, SourceRetitleEdit{
			Resource: c.cur.resource, Old: c.cur.title, Title: c.name, Footnote: id,
		})
	}
}

// titleCall is one deferred planSourceTitle call: an entry that cites a source
// whose full-citation title may be shortened.
type titleCall struct {
	entry string
	cur   sourceTitle
	name  string
}

// replayTitleCalls decides the title of each source with a deferred call. A
// source with a planned retitle gets, in this one pass, the title a re-plan
// of the result would settle on: the first name any of its entries gives
// (an entry with no name leaves the title for a later one), or no title when
// none does. Each call is then replayed against that final title, so the
// conflict and unnamed items match what a re-plan reports (§12: re-running
// changes nothing). A source whose retitle was refused keeps its title and
// its calls are replayed unchanged.
func replayTitleCalls(plan *MigratePlan, nm *NodeMigration, existing map[string]sourceTitle, rel string, deferred map[string][]titleCall) {
	if len(deferred) == 0 {
		return
	}
	retitled := map[string]int{}
	for i, r := range nm.Retitles {
		retitled[r.Resource] = i
	}
	resources := make([]string, 0, len(deferred))
	for res := range deferred {
		resources = append(resources, res)
	}
	sort.Strings(resources)
	for _, res := range resources {
		calls := deferred[res]
		i, ok := retitled[res]
		if !ok {
			for _, c := range calls {
				planSourceTitle(plan, nm, existing, rel, c.entry, c.cur, c.name)
			}
			continue
		}
		final := ""
		for _, c := range calls {
			if c.name != "" && !linkRefDefRe.MatchString(c.entry) && citationHasProse(c.entry) {
				final = c.name
				break
			}
		}
		nm.Retitles[i].Title = final
		cur := sourceTitle{resource: res, title: final, has: final != ""}
		for _, c := range calls {
			planSourceTitle(plan, nm, existing, rel, c.entry, cur, c.name)
		}
	}
}

// applyTitleRewrites applies each retitle whose source still has title Old
// and whose footnote definition, in raw as it stands, holds Old's text. Any
// other edit is skipped, so a stale plan never overwrites a title the author
// changed and never drops text that no footnote holds; a re-applied plan
// changes nothing. Removals delete the one `title:` line; replacements splice
// the value (§4: unchanged values are not re-serialized). Shapes the line
// edits do not model go through editFrontmatter.
func applyTitleRewrites(raw []byte, edits []SourceRetitleEdit) ([]byte, error) {
	if len(edits) == 0 {
		return raw, nil
	}
	m := parsedFrontmatterMapping(raw)
	if m == nil {
		return raw, nil
	}
	body := ""
	if _, after, ok := splitFrontmatterRaw(raw); ok {
		body = string(after)
	}
	defs := footnoteDefinitions(body)
	var removes, replaces []SourceRetitleEdit
	for _, e := range edits {
		item := firstSourceItem(m, e.Resource)
		t := mappingValue(item, "title")
		if item == nil || t == nil || t.Kind != yaml.ScalarNode || t.Value != e.Old {
			continue
		}
		if !titlePreserved(e.Old, e.Footnote, defs[strings.ToLower(e.Footnote)]) {
			continue
		}
		if e.Title == "" {
			removes = append(removes, e)
		} else {
			replaces = append(replaces, e)
		}
	}
	out := raw
	if len(removes) > 0 {
		var err error
		if out, err = removeSourceTitles(out, removes); err != nil {
			return nil, err
		}
	}
	if len(replaces) == 0 {
		return out, nil
	}
	byRes := map[string]string{}
	for _, e := range replaces {
		byRes[e.Resource] = e.Title
	}
	return rewriteScalars(out, func(root *yaml.Node) []scalarChange {
		var cs []scalarChange
		for _, e := range replaces {
			if item := firstSourceItem(root, e.Resource); item != nil {
				if t := mappingValue(item, "title"); t != nil && t.Value == e.Old {
					cs = append(cs, scalarChange{node: t, value: byRes[e.Resource]})
				}
			}
		}
		return cs
	})
}

// removeSourceTitles deletes the `title` key of each edit's source. It
// deletes the key's single line when that line holds only the key and its
// one-line value, and verifies the result decodes to the edited tree;
// otherwise it falls back to editFrontmatter.
func removeSourceTitles(raw []byte, edits []SourceRetitleEdit) ([]byte, error) {
	mutate := func(root *yaml.Node) {
		for _, e := range edits {
			if item := firstSourceItem(root, e.Resource); item != nil {
				removeKey(item, "title")
			}
		}
	}
	if out, ok := deleteSourceTitleLines(raw, edits, mutate); ok {
		return out, nil
	}
	return editFrontmatter(raw, mutate)
}

func deleteSourceTitleLines(raw []byte, edits []SourceRetitleEdit, mutate func(*yaml.Node)) ([]byte, bool) {
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
	var del []byteEdit
	for _, e := range edits {
		item := firstSourceItem(m, e.Resource)
		if item == nil || item.Style&yaml.FlowStyle != 0 {
			return nil, false
		}
		var key, val *yaml.Node
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value == "title" {
				key, val = item.Content[i], item.Content[i+1]
			}
		}
		if key == nil || key.Line < 1 || key.Line >= len(lines) || val.Line != key.Line {
			return nil, false
		}
		line := lineAt(block, lines, key.Line)
		if !bytes.HasPrefix(bytes.TrimLeft(line, " "), []byte("title:")) {
			return nil, false // the key shares its line with `- ` or another key
		}
		del = append(del, byteEdit{lines[key.Line-1], lines[key.Line], nil})
	}
	out := applyEdits(block, del)
	if !verifySplice(out, edited) {
		return nil, false
	}
	var buf bytes.Buffer
	buf.Write(head)
	buf.Write(out)
	buf.Write(tail)
	return buf.Bytes(), true
}
