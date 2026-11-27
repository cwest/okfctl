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

import "strconv"

// A `# Citations` entry that names several follow-able resources becomes one
// §5.1 source per resource: §5.1 makes `resource` REQUIRED and singular, so
// one source cannot hold two. The parts share the entry's key. The first
// part's `id` is the id the entry would get on its own (CitationEntryIDs);
// later parts are `<id>-2`, `<id>-3`, in the order the entry names them. The
// first part's footnote definition holds the whole entry text, as for any
// converted entry; consumers do not parse it (§5.1). A later part's definition
// is its name, or its resource when it has none, so nothing in it is new
// prose. A body `[k]` that cites the entry becomes one footnote per part
// (`[^12][^12-2]`, §4.2), so per-claim attribution reaches every part.
//
// An entry is split only when no part is named by another entry of the same
// list: a source holds one id, so a shared part would need two. Such an entry
// stays a prose-citation item. A part already in frontmatter `sources` (an
// earlier run, or the author) is not added again (§12); it keeps its id when
// it has one.

// splitPart is a later part (the second onward) of a split entry: its
// resource, the id its source already carries ("" when none), and the text of
// its footnote definition when it has a name.
type splitPart struct {
	resource   string
	existingID string
	def        string
}

// splitPartID is the id of the n-th part (n >= 2) of the entry whose id is id.
func splitPartID(id string, n int) string {
	return id + "-" + strconv.Itoa(n)
}

// partResources lists the resources of a split entry's later parts.
func (a entryAttachment) partResources() []string {
	out := make([]string, len(a.parts))
	for i, p := range a.parts {
		out[i] = p.resource
	}
	return out
}

// planSplitEntry plans the sources of one split entry and returns its
// attachment. Each part not yet in `sources` is added (§5.1); a part already
// there is not added again (§12). The first part's title is handled as for a
// single-resource entry: a missing one is filled with the name the entry
// marks, and an unnamed first part is a source-title-unnamed item. A later
// part already in `sources` gets its link text as title only when it has
// none. srcIDs are the ids the node's sources carry, so a re-plan keeps them.
func planSplitEntry(plan *MigratePlan, nm *NodeMigration, rel, item string, parts []SourceEdit,
	existing map[string]sourceTitle, srcIDs map[string]string, added map[string]bool) entryAttachment {
	for k, p := range parts {
		if cur, found := lookupSourceTitle(existing, p.Resource); found {
			parts[k].Resource = cur.resource
			switch {
			case k == 0:
				planSourceTitle(plan, nm, existing, rel, item, cur, p.Title)
			case !cur.has && p.Title != "":
				nm.Titles = append(nm.Titles, SourceTitleEdit{Resource: cur.resource, Title: p.Title})
			}
			continue
		}
		if added[canonicalResource(p.Resource)] {
			continue
		}
		if k == 0 && p.Title == "" {
			plan.Judgment = append(plan.Judgment, JudgmentItem{
				Path: rel, Kind: JudgmentSourceTitleUnnamed, Context: item,
			})
		}
		nm.Sources = append(nm.Sources, SourceEdit{Resource: p.Resource, Title: p.Title})
		added[p.Resource] = true
		added[canonicalResource(p.Resource)] = true
	}
	att := entryAttachment{resource: parts[0].Resource, existingID: srcIDs[parts[0].Resource]}
	for _, p := range parts[1:] {
		att.parts = append(att.parts, splitPart{resource: p.Resource, existingID: srcIDs[p.Resource], def: p.Title})
	}
	return att
}

// planCitationSplits returns, for each entry, the sources it splits into, or
// nil when it is not split: it names fewer than two resources outside code,
// or a part is also named by another entry in the list. rooted maps a bare
// bundle path to the `/`-rooted form it is written in (§6.2, §12.1).
func planCitationSplits(entries []string, rooted func(string) string) [][]SourceEdit {
	out := make([][]SourceEdit, len(entries))
	owners := map[string]int{} // canonical resource -> entries naming it
	named := make([][]string, len(entries))
	for i, e := range entries {
		parts, _, _ := citationResources(entryText(e))
		for _, p := range parts {
			k := canonicalResource(rooted(p.resource))
			named[i] = append(named[i], k)
			owners[k]++
		}
	}
	for i, e := range entries {
		if linkRefDefRe.MatchString(e) {
			continue
		}
		srcs, ok := citationSourcesFrom(e)
		if !ok {
			continue
		}
		shared := false
		for _, k := range named[i] {
			if owners[k] > 1 {
				shared = true
			}
		}
		if shared {
			continue
		}
		for k := range srcs {
			srcs[k].Resource = rooted(srcs[k].Resource)
		}
		out[i] = srcs
	}
	return out
}
