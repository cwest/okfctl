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
// A resource that two entries name is still one source with one id (§5.1).
// Its owner is the first entry that names it alone (a single-resource entry
// that converts), or, when none does, the first splittable entry that names
// it. A split entry adds a source only for the parts it owns, in the order
// it names them; its first owned part is its primary source and holds its
// full text. A part another entry owns is borrowed: the entry's body `[k]`
// also cites the owner's id (`[^5][^2-2]`), so the claim stays attributed to
// every resource the entry names. The name an entry marks titles its primary
// only when the primary is the entry's first resource: a borrowed first
// resource takes the name with it, and the owned part is titled by its own
// link text or not at all. An entry that owns no part names nothing of its
// own (every resource it names is another entry's work) and stays a
// prose-citation item, read through the §12.1 fallback. A part already in
// frontmatter `sources` (an earlier run, or the author) is not added again
// (§12); it keeps its id when it has one. Ownership is read from the body
// list alone, so a re-plan agrees.

// splitPart is a later part (the second onward) of a split entry: its
// resource, the id its source already carries ("" when none), and the text of
// its footnote definition when it has a name.
type splitPart struct {
	resource   string
	existingID string
	def        string
}

// partRef names the source a borrowed part cites: the owner entry's index and
// the owned part's position in it (0 is the owner's primary source).
type partRef struct {
	entry, part int
}

// citationSplit is the split of one entry: the sources it owns, in the order
// it names them, and the sources of other entries it borrows.
type citationSplit struct {
	parts    []SourceEdit
	borrowed []partRef
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

// partLink is the `[text](target)` span that names the j-th resource of a
// citation entry, or "" when that resource is a bare URL.
func partLink(entry string, j int) string {
	parts, _, _ := citationResources(entryText(entry))
	if j < len(parts) {
		return parts[j].link
	}
	return ""
}

// planSplitEntry plans the sources of one split entry and returns its
// attachment. Each owned part not yet in `sources` is added (§5.1); a part
// already there is not added again (§12). The first owned part's title is
// handled as for a single-resource entry: a missing one is filled with its
// name, and an unnamed first part is a source-title-unnamed item. A later
// part already in `sources` gets its link text as title only when it has
// none. srcIDs are the ids the node's sources carry, so a re-plan keeps them.
func planSplitEntry(plan *MigratePlan, nm *NodeMigration, rel, item string, split *citationSplit,
	existing map[string]sourceTitle, srcIDs map[string]string, added map[string]bool) entryAttachment {
	parts := split.parts
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
	att := entryAttachment{resource: parts[0].Resource, existingID: srcIDs[parts[0].Resource], borrowed: split.borrowed}
	for _, p := range parts[1:] {
		att.parts = append(att.parts, splitPart{resource: p.Resource, existingID: srcIDs[p.Resource], def: p.Title})
	}
	return att
}

// planCitationSplits returns, for each entry, the split it becomes, or nil
// when it is not split: it names fewer than two resources outside code, or it
// owns none of them. A link reference definition (`[label]: url`) is the
// target of a `[text][label]` link, not a citation: it gets no id
// (planSourceIDs) and owns no resource, so an entry that names its resource
// still owns it and splits. rooted maps a bare bundle path to the
// `/`-rooted form it is written in (§6.2, §12.1).
func planCitationSplits(entries []string, rooted func(string) string) []*citationSplit {
	out := make([]*citationSplit, len(entries))
	names := make([]map[string]bool, len(entries))
	for i, e := range entries {
		names[i] = citationNames(e)
	}
	// owners maps a resource to the entry and part that own it.
	owners := map[string]partRef{}
	// A single-resource entry that converts owns its resource. An
	// identifier another entry also names is a cross-reference, not the
	// entry's own document (planMigration), so it owns nothing; nor does a
	// link reference definition.
	for i, e := range entries {
		if linkRefDefRe.MatchString(e) {
			continue
		}
		src, ok, fromID := citationSourceFrom(e)
		if !ok || fromID && citedElsewhere(names, i, canonicalResource(src.Resource)) {
			continue
		}
		k := canonicalResource(rooted(src.Resource))
		if _, taken := owners[k]; !taken {
			owners[k] = partRef{entry: i}
		}
	}
	all := make([][]SourceEdit, len(entries))
	for i, e := range entries {
		if linkRefDefRe.MatchString(e) {
			continue
		}
		if parts, ok := citationSourcesFrom(e); ok {
			all[i] = parts
		}
	}
	// A splittable entry owns, in list order, each part no earlier entry
	// owns.
	own := make([][]SourceEdit, len(entries))
	for i, parts := range all {
		for j, p := range parts {
			p.Resource = rooted(p.Resource)
			k := canonicalResource(p.Resource)
			if _, taken := owners[k]; taken {
				continue
			}
			if j > 0 && len(own[i]) == 0 {
				// The first resource is borrowed and carries the name the
				// entry marks; the first owned part keeps only its own
				// link text (citationName of its link), or no title.
				p.Title = ""
				if l := partLink(entries[i], j); l != "" {
					p.Title, _ = citationName(l)
				}
			}
			owners[k] = partRef{entry: i, part: len(own[i])}
			own[i] = append(own[i], p)
		}
	}
	for i, parts := range all {
		if len(own[i]) == 0 {
			continue // owns nothing: kept for judgment
		}
		split := &citationSplit{parts: own[i]}
		for _, p := range parts {
			o := owners[canonicalResource(rooted(p.Resource))]
			if o.entry == i {
				continue
			}
			split.borrowed = append(split.borrowed, o)
		}
		out[i] = split
	}
	return out
}
