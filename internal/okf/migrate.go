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
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Migration of a v0.1 or v0.2 bundle (or one that declares no version) to the
// target version, OKF v0.3. It applies the v0.1 -> v0.2 renames (v0.3 Appendix
// B: `timestamp` → `generated.at`; body `# Citations` → frontmatter `sources`),
// the v0.2 -> v0.3 edits (§13.1, see migrate_v03.go: bare paths that resolve
// only through the §12.1 bundle-root fallback become `/`-rooted, and §5
// timestamps are respelled where the instant is unambiguous), plus the §8.1/§12
// version markers. It does so in TWO phases so the tool never acquires a model
// dependency:
//
//   - MigratePlan is PURE READ. It computes every DETERMINISTIC edit and
//     enumerates every JUDGMENT item (a citation with no follow-able resource,
//     or a `timestamp` rename with no actor) with enough context to resolve it
//     later. It writes nothing.
//   - MigrateApply reads a plan back and applies its deterministic edits,
//     order-preserving and additive-only.
//
// A judgment item is NEVER "solved" by guessing: §5.1 requires a `resource` per
// source, so a prose finding cannot be forced into `sources` without fabricating
// one — the one thing this migration must not do (§11 favors ignoring over
// discarding, but never inventing).

// TargetOkfVersion is the version this migrator upgrades a bundle TO: the
// version this build targets.
const TargetOkfVersion = SpecVersion

// JudgmentKind classifies why an item could not be migrated deterministically.
type JudgmentKind string

const (
	// JudgmentProseCitation: a `# Citations` item with no follow-able resource
	// (§5.1 requires `resource`). The consumer must supply a resource, reclassify
	// it (e.g. to §5.2 `verified`), or drop it.
	JudgmentProseCitation JudgmentKind = "prose-citation"
	// JudgmentMissingActor: a `timestamp` rename with no actor supplied and none
	// inferable (§7). The consumer must supply `generated.by`.
	JudgmentMissingActor JudgmentKind = "missing-actor"
	// JudgmentSourceTitleConflict: a `# Citations` entry that names a
	// different work (citationName) than the `title` its source already has
	// (§5.1). The existing title is kept; the consumer decides. The resource
	// is already in frontmatter, so this needs no §12.1 fallback and does not
	// hold the version declaration.
	JudgmentSourceTitleConflict JudgmentKind = "source-title-conflict"
	// JudgmentSourceTitleUnnamed: a `# Citations` entry whose source has no
	// `title` and whose text marks no work name (citationName). §5.1 title is
	// a label for humans, the work's name; the full entry text is not one, so
	// the title stays unset until the consumer names it. The resource is in
	// frontmatter, so this does not hold the version declaration.
	JudgmentSourceTitleUnnamed JudgmentKind = "source-title-unnamed"
)

// JudgmentItem is one thing the migrator refuses to guess. Path is the node it
// belongs to; Context is verbatim material (the citation line, the timestamp
// value) the consumer needs to resolve it.
type JudgmentItem struct {
	Path    string       `json:"path"`
	Kind    JudgmentKind `json:"kind"`
	Context string       `json:"context"`
}

// GeneratedEdit is the planned §13.1 `timestamp` → `generated { by, at }`
// rename for one node. At is the legacy timestamp value carried over verbatim;
// By is the supplied/inferred actor (§7).
type GeneratedEdit struct {
	By string `json:"by"`
	At string `json:"at"`
}

// SourceEdit is one planned §5.1 `sources` entry derived from a body citation.
// Resource is REQUIRED and preserves the author's form (absolute URL, or a
// bundle-relative path kept relative per §6).
type SourceEdit struct {
	Resource string `json:"resource"`
	// Title is the optional §5.1 label ("a label for humans"): the cited
	// work's name (citationName), never the entry's full text.
	Title string `json:"title,omitempty"`
	// ID is the §5.1 id the body cites the source by (CitationEntryIDs).
	ID string `json:"id,omitempty"`
}

// SourceTitleEdit sets the optional §5.1 `title` of a `sources` entry that is
// already listed by Resource and has no title. Title is the name of the work
// (citationName) the matching `# Citations` entry cites.
type SourceTitleEdit struct {
	Resource string `json:"resource"`
	Title    string `json:"title"`
}

// NodeMigration is the set of deterministic frontmatter edits planned for one
// node. A nil Generated means no timestamp rename is planned; an empty Sources
// means no citation became a source. Only nodes with at least one edit appear.
type NodeMigration struct {
	Path      string         `json:"path"`
	Generated *GeneratedEdit `json:"generated,omitempty"`
	Sources   []SourceEdit   `json:"sources,omitempty"`
	// Titles fill in the missing title of sources already listed in
	// frontmatter with the cited work's name.
	Titles []SourceTitleEdit `json:"titles,omitempty"`
	// Paths are the v0.3 §6 rewrites of values that resolve only through the
	// §12.1 bundle-root fallback.
	Paths []PathEdit `json:"paths,omitempty"`
	// Timestamps are the v0.3 §5 rewrites to the RFC 3339 form.
	Timestamps []TimestampEdit `json:"timestamps,omitempty"`
	// IDs set the §5.1 id of sources already listed in frontmatter.
	IDs []SourceIDEdit `json:"ids,omitempty"`
	// Footnotes are the §4.2 footnote definitions holding each converted
	// entry's full text.
	Footnotes []FootnoteEdit `json:"footnotes,omitempty"`
	// Retitles shorten a title that holds its full citation entry to the
	// work's name, once a footnote definition holds that text.
	Retitles []SourceRetitleEdit `json:"retitles,omitempty"`
	// Refs are the body citation references rewritten to footnotes.
	Refs []RefEdit `json:"refs,omitempty"`
	// RefKeys maps each body citation key to the id it is rewritten to; the
	// renderer finds the planned Refs with it.
	RefKeys map[string]string `json:"ref_keys,omitempty"`
	// RemoveCitations removes the node's v0.1 `# Citations` section
	// (§4.2, Appendix B) once every entry in it is recoverable from
	// `sources` and footnote definitions (citationListLoss).
	RemoveCitations bool `json:"remove_citations,omitempty"`
}

func (nm NodeMigration) hasEdits() bool {
	return nm.Generated != nil || len(nm.Sources) > 0 || len(nm.Titles) > 0 || len(nm.Paths) > 0 || len(nm.Timestamps) > 0 ||
		len(nm.IDs) > 0 || len(nm.Footnotes) > 0 || len(nm.Refs) > 0 || len(nm.Retitles) > 0 || nm.RemoveCitations
}

// EditCount is the number of deterministic edits planned for the node.
func (nm NodeMigration) EditCount() int {
	n := len(nm.Sources) + len(nm.Titles) + len(nm.Paths) + len(nm.Timestamps) + len(nm.IDs) + len(nm.Footnotes) + len(nm.Retitles)
	for _, r := range nm.Refs {
		n += r.Count
	}
	if nm.Generated != nil {
		n++
	}
	if nm.RemoveCitations {
		n++
	}
	return n
}

// MigratePlan is the full, JSON-serializable migration plan: the deterministic
// per-node edits, the enumerated judgment items, and the target version. It is
// the review surface — for Casey and for a solo user alike.
type MigratePlan struct {
	TargetVersion string `json:"target_version"`
	// DeclaredVersion is the okf_version the bundle declared when planned, or
	// "" when it declared none.
	DeclaredVersion string `json:"declared_version"`
	// VersionAction is VersionBump (the markers move to TargetVersion),
	// VersionHold (the edits apply but the declaration waits, because an item
	// still depends on a §12.1 fallback), or VersionKeep (the bundle is left at
	// the version it declares, §12). A plan file without the field (written
	// before it existed) is a bump.
	VersionAction string `json:"version_action,omitempty"`
	// ForceDeclare records that the plan bumps even though fallback items
	// remain (MigrateOptions.ForceDeclare).
	ForceDeclare bool            `json:"force_declare,omitempty"`
	Nodes        []NodeMigration `json:"nodes"`
	Judgment     []JudgmentItem  `json:"judgment"`
}

// DeclaresTarget reports whether applying the plan declares TargetVersion.
func (p MigratePlan) DeclaresTarget() bool {
	return p.VersionAction == VersionBump || p.VersionAction == ""
}

// FallbackItems counts the judgment items a consumer can still read only
// through a §12.1 fallback (Appendix B: a prose `# Citations` entry, or a
// legacy `timestamp` with no `generated`). Declaring 0.3 switches those
// fallbacks off, so while any remain the plan holds the declaration.
func (p MigratePlan) FallbackItems() int {
	n := 0
	for _, j := range p.Judgment {
		if j.Kind == JudgmentProseCitation || j.Kind == JudgmentMissingActor {
			n++
		}
	}
	return n
}

// MigrateOptions tunes PlanMigrationWith.
type MigrateOptions struct {
	// GeneratedBy is the actor (§7) recorded for every `timestamp` rename.
	GeneratedBy string
	// ForceDeclare declares the target version even while fallback items
	// remain. Those items then lose the §12.1 fallback that still carries
	// them; the plan records the choice.
	ForceDeclare bool
}

// PlanMigration computes the migration plan to TargetOkfVersion for a loaded
// bundle. It is PURE: it reads the bundle and writes nothing. generatedBy is
// the actor (§7) used for every `timestamp` rename; when empty and no actor is
// inferable, each such node becomes a JudgmentMissingActor item instead of a
// guessed rename.
//
// A bundle that declares a version later than the target, or one this build
// does not know, gets an empty VersionKeep plan: §12 says a consumer SHOULD
// consume it best-effort and SHOULD NOT refuse it, and rewriting it toward an
// older version would be a downgrade. A bundle that already declares the
// target gets content edits only; its markers stay.
func PlanMigration(b *Bundle, generatedBy string) (MigratePlan, error) {
	return PlanMigrationWith(b, MigrateOptions{GeneratedBy: generatedBy})
}

// PlanMigrationWith is PlanMigration with options. A legacy bundle whose plan
// leaves fallback items (MigratePlan.FallbackItems) gets VersionHold unless
// opts.ForceDeclare is set: §12.1 grants the Appendix B fallbacks only to a
// bundle that declares 0.2 or earlier, or nothing, so declaring 0.3 first
// would make a consumer drop those items.
func PlanMigrationWith(b *Bundle, opts MigrateOptions) (MigratePlan, error) {
	plan, err := planMigration(b, opts.GeneratedBy)
	if err != nil {
		return plan, err
	}
	if plan.VersionAction == VersionBump && plan.FallbackItems() > 0 {
		if opts.ForceDeclare {
			plan.ForceDeclare = true
		} else {
			plan.VersionAction = VersionHold
		}
	}
	return plan, nil
}

func planMigration(b *Bundle, generatedBy string) (MigratePlan, error) {
	plan := MigratePlan{
		TargetVersion:   TargetOkfVersion,
		DeclaredVersion: b.DeclaredOkfVersion,
		VersionAction:   planVersionAction(b),
	}
	if plan.VersionAction == VersionKeep && !sameVersion(b.DeclaredOkfVersion, TargetOkfVersion) {
		return plan, nil
	}

	byPath := map[string]*NodeMigration{}
	nodeFor := func(rel string) *NodeMigration {
		if nm, ok := byPath[rel]; ok {
			return nm
		}
		byPath[rel] = &NodeMigration{Path: rel}
		return byPath[rel]
	}

	for _, rel := range sortedNodePaths(b) {
		n := b.Nodes[rel]
		if n.Frontmatter == nil {
			continue // unparseable frontmatter is a different failure class
		}
		nm := nodeFor(rel)

		// §13.1: timestamp -> generated { by, at }. Only when generated is not
		// already present (idempotence) and a legacy timestamp exists.
		if _, has := n.Frontmatter["generated"]; !has {
			if ts, ok := n.Frontmatter["timestamp"]; ok {
				// v0.2 §5 "Provenance, trust, and lifecycle": every timestamp-valued
				// key is an ISO 8601 datetime with an explicit UTC offset, so the
				// rename must carry the legacy value across at FULL precision. The
				// lossless source is the raw frontmatter scalar: yaml.v3 resolves an
				// unquoted ISO datetime to time.Time (dropping precision if re-rendered
				// as a bare date) and a quoted one to a string, so re-rendering the
				// parsed time is asymmetric and lossy. Carrying the source scalar makes
				// quoted and unquoted identical by construction and never invents a
				// time-of-day (a bare `2026-08-01` stays `2026-08-01`, not
				// `2026-08-01T00:00:00Z`). migrateTimeString is the fallback for the
				// rare case the raw scalar is unreachable (e.g. an unreadable file).
				atStr, ok := rawTimestampScalar(b.Root, rel)
				if !ok {
					atStr = migrateTimeString(ts)
				}
				if atStr == "" {
					atStr = fmt.Sprintf("%v", ts)
				}
				if generatedBy != "" {
					nm.Generated = &GeneratedEdit{By: generatedBy, At: atStr}
				} else {
					// §7: never guess provenance — report & skip.
					plan.Judgment = append(plan.Judgment, JudgmentItem{
						Path: rel, Kind: JudgmentMissingActor, Context: atStr,
					})
				}
			}
		}

		// Appendix B: body # Citations -> frontmatter sources. An entry that
		// names exactly one follow-able resource becomes one source; one that
		// names several becomes one source per resource (§5.1 allows one
		// `resource` per source); a prose finding is a judgment item (§5.1
		// requires resource; never fabricate one). Idempotence:
		// a resource already present in the node's sources is not added
		// again; its missing title is filled with the work's name instead.
		existing := existingSourceTitles(n)
		added := map[string]bool{}
		entries := citationEntries(n.Body)
		names := make([]map[string]bool, len(entries))
		for i, item := range entries {
			names[i] = citationNames(item)
		}
		att := make([]entryAttachment, len(entries))
		srcIDs, _ := existingSourceIDs(n)
		attached := map[string]bool{}
		var retitle []retitleCandidate
		deferred := map[string][]titleCall{}
		attach := func(i int, res string) {
			k := canonicalResource(res)
			if attached[k] {
				att[i] = entryAttachment{resource: res, duplicate: true}
				return
			}
			attached[k] = true
			att[i] = entryAttachment{resource: res, existingID: srcIDs[res]}
		}
		// §6.2/§12.1: a bare resource that resolves only from the bundle
		// root is written `/`-rooted, the form its body link is rewritten
		// to, so it resolves in the v0.3 bundle and a re-plan finds it.
		rooted := func(res string) string {
			if r := b.ResolvePathField(rel, res); r.Kind == RefBundle && r.Form == FormRootFallback {
				return rootedURL(res, r.Path)
			}
			return res
		}
		splits := planCitationSplits(entries, rooted)
		for i, item := range entries {
			src, ok, fromID := citationSourceFrom(item)
			res := src.Resource
			// An identifier that another entry in the same list also names
			// is a cross-reference ("attribution to arXiv:..."), not this
			// entry's own document. Skipping it as already migrated would
			// drop the entry with no source of its own, so it stays for
			// judgment. Read from the body only, so a re-plan agrees.
			if ok && fromID && citedElsewhere(names, i, canonicalResource(res)) {
				ok = false
			}
			if !ok && splits[i] != nil {
				att[i] = planSplitEntry(&plan, nm, rel, item, splits[i], existing, srcIDs, added)
				for _, r := range append([]string{att[i].resource}, att[i].partResources()...) {
					attached[canonicalResource(r)] = true
				}
				continue
			}
			if !ok {
				plan.Judgment = append(plan.Judgment, JudgmentItem{
					Path: rel, Kind: JudgmentProseCitation, Context: item,
				})
				continue
			}
			res = rooted(res)
			if added[res] || added[canonicalResource(res)] {
				attach(i, res)
				continue // a second entry for a source this plan adds
			}
			if cur, ok := lookupSourceTitle(existing, res); ok {
				attach(i, cur.resource)
				if !att[i].duplicate && cur.has && cur.title != src.Title && isEntryText(cur.title, item) {
					retitle = append(retitle, retitleCandidate{entry: i, cur: cur, name: src.Title})
					deferred[cur.resource] = []titleCall{}
				}
				// A source whose title may be shortened is decided once its
				// final title is known (replayTitleCalls), so a re-plan of
				// the result agrees with this plan.
				if calls, ok := deferred[cur.resource]; ok {
					deferred[cur.resource] = append(calls, titleCall{entry: item, cur: cur, name: src.Title})
					continue
				}
				planSourceTitle(&plan, nm, existing, rel, item, cur, src.Title)
				continue
			}
			attach(i, res)
			if src.Title == "" && citationHasProse(item) {
				plan.Judgment = append(plan.Judgment, JudgmentItem{
					Path: rel, Kind: JudgmentSourceTitleUnnamed, Context: item,
				})
			}
			nm.Sources = append(nm.Sources, SourceEdit{Resource: res, Title: src.Title})
			added[res] = true
			added[canonicalResource(res)] = true
		}
		if len(entries) > 0 {
			ids := planSourceIDs(&plan, nm, n, rel, entries, att)
			planRetitles(&plan, nm, n, rel, entries, ids, retitle)
		}
		replayTitleCalls(&plan, nm, existing, rel, deferred)
	}

	// v0.3 §6/§12.1 and §5 edits. Links are rewritten in every Markdown file
	// that carries them (concepts, §3.2 rule 2 non-concepts, and the reserved
	// index/log files); path fields and timestamps only in concepts.
	files := append(sortedLinkSourcePaths(b), sortedReservedPaths(b)...)
	sort.Strings(files)
	for _, rel := range files {
		n, concept := b.Nodes[rel], true
		if n == nil {
			concept = false
			if n = b.NonConcepts[rel]; n == nil {
				n = b.Reserved[rel]
			}
		}
		if n == nil {
			continue
		}
		pn := *n
		var raw []byte
		if concept && n.Frontmatter != nil {
			var err error
			raw, err = os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(rel))) //nolint:gosec // G304: reading a node from the user's own bundle
			if err != nil {
				return MigratePlan{}, fmt.Errorf("read %s: %w", rel, err)
			}
		} else {
			pn.Frontmatter = nil // path fields and timestamps are concept families
		}
		paths, stamps := planV03Node(b, rel, &pn, raw, &plan.Judgment)
		if len(paths) == 0 && len(stamps) == 0 {
			continue
		}
		nm := nodeFor(rel)
		nm.Paths, nm.Timestamps = paths, stamps
	}

	// §4.2, Appendix B: a node's `# Citations` list goes once its rendered
	// bytes hold everything in it; otherwise it stays, reported. Decided last,
	// on every edit planned above.
	for _, rel := range sortedNodePaths(b) {
		if n := b.Nodes[rel]; n.Frontmatter == nil || !citationsHeadRe.MatchString(n.Body) {
			continue
		}
		if err := planCitationRemoval(&plan, b, rel, nodeFor(rel)); err != nil {
			return MigratePlan{}, err
		}
	}

	order := make([]string, 0, len(byPath))
	for rel := range byPath {
		order = append(order, rel)
	}
	sort.Strings(order)
	for _, rel := range order {
		if nm := byPath[rel]; nm.hasEdits() {
			plan.Nodes = append(plan.Nodes, *nm)
		}
	}
	sort.SliceStable(plan.Judgment, func(i, j int) bool { return plan.Judgment[i].Path < plan.Judgment[j].Path })
	return plan, nil
}

// sortedReservedPaths returns the bundle's reserved file paths, sorted.
func sortedReservedPaths(b *Bundle) []string {
	out := make([]string, 0, len(b.Reserved))
	for p := range b.Reserved {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// sameVersion reports whether two okf_version values name the same version
// (§12), tolerating a leading "v".
func sameVersion(a, b string) bool {
	norm := func(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "v") }
	return norm(a) != "" && norm(a) == norm(b)
}

// rawTimestampScalar returns the VERBATIM source scalar of a node's legacy
// `timestamp` frontmatter key — the lossless value the v0.2 §5 rename must carry
// across. It re-reads the node file (PlanMigration is pure read, so this keeps
// that contract) and reads the raw yaml.Node scalar, which preserves the author's
// exact text regardless of yaml.v3 tag resolution: an unquoted datetime keeps its
// time-of-day and offset, a quoted value round-trips byte-identical, and a bare
// date stays a bare date (no invented T00:00:00Z). ok is false when the file is
// unreadable, has no frontmatter mapping, or the timestamp value is not a scalar —
// callers fall back to migrateTimeString.
func rawTimestampScalar(root, rel string) (string, bool) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading a node from the user's own bundle
	if err != nil {
		return "", false
	}
	yamlBlock, _, ok := splitFrontmatterRaw(raw)
	if !ok {
		return "", false
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return "", false
	}
	m := frontmatterMapping(&doc)
	if m == nil {
		return "", false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "timestamp" {
			v := m.Content[i+1]
			if v.Kind != yaml.ScalarNode {
				return "", false
			}
			return v.Value, true
		}
	}
	return "", false
}

// migrateTimeString renders a legacy `timestamp` value for the v0.2 §5
// `generated.at` rename when the raw source scalar is unreachable. Unlike the
// display helper frontmatterTimeString (which truncates a time.Time to a bare
// date for analyze's freshness basis), this preserves full precision: a string
// passes through verbatim and a time.Time renders with the package
// timestampLayout (RFC3339, the layout §5 requires). It is the fallback path;
// the primary path carries the source scalar (rawTimestampScalar).
func migrateTimeString(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(timestampLayout) // time.RFC3339 — v0.2 §5
	case string:
		return t
	}
	return ""
}

// MigrateApply applies a plan's deterministic edits to disk, order-preserving
// and additive-only. root is the bundle root; b is the loaded bundle the plan
// was computed against. It writes each edited node in place, then bumps the
// version markers (bundle-root index.md, §8.1; and the legacy .okf sidecar
// when present) unless the plan is VersionKeep. b's declared version decides
// which values still resolve through the §12.1 fallback. It never touches a
// judgment item — those stay for the plan's consumer. Every edit re-checks the
// current bytes, so applying the same plan to a tree it already migrated
// changes nothing. A VersionHold plan applies its edits and declares nothing.
func MigrateApply(root string, b *Bundle, plan MigratePlan) error {
	for _, nm := range plan.Nodes {
		raw, out, err := RenderMigration(root, b, nm)
		if err != nil {
			return err
		}
		if bytes.Equal(out, raw) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(nm.Path))
		if err := os.WriteFile(abs, out, 0o644); err != nil { //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
			return fmt.Errorf("write %s: %w", nm.Path, err)
		}
	}
	if !plan.DeclaresTarget() {
		return nil
	}
	return bumpVersionMarkers(root, plan.TargetVersion)
}

// RenderMigration returns a file's current bytes and its bytes with the
// planned edits applied. It is PURE (it writes nothing) and it is the single
// renderer MigrateApply and `migrate --apply --dry-run` share, so a dry run
// shows exactly what an apply writes.
func RenderMigration(root string, b *Bundle, nm NodeMigration) ([]byte, []byte, error) {
	abs := filepath.Join(root, filepath.FromSlash(nm.Path))
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading a node from the user's own bundle
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", nm.Path, err)
	}
	out, err := renderMigratedNode(raw, nm)
	if err != nil {
		return nil, nil, fmt.Errorf("migrate %s: %w", nm.Path, err)
	}
	if out, err = applySourceIDs(out, nm.IDs); err != nil {
		return nil, nil, fmt.Errorf("migrate %s: %w", nm.Path, err)
	}
	out = renderSourceIDs(out, nm)
	if out, err = applyTitleRewrites(out, nm.Retitles); err != nil {
		return nil, nil, fmt.Errorf("migrate %s: %w", nm.Path, err)
	}
	if len(nm.Paths) > 0 || len(nm.Timestamps) > 0 {
		if out, err = renderV03Node(b, nm.Path, out, nm); err != nil {
			return nil, nil, fmt.Errorf("migrate %s: %w", nm.Path, err)
		}
	}
	if nm.RemoveCitations {
		out = removeCitationSection(b, nm.Path, out)
	}
	return raw, out, nil
}

// RenderVersionMarkers returns the current and migrated bytes of each version
// marker file (index.md, .okf) that a plan changes, keyed by bundle-relative
// path. It writes nothing.
func RenderVersionMarkers(root string, plan MigratePlan) (map[string][2][]byte, error) {
	out := map[string][2][]byte{}
	if !plan.DeclaresTarget() {
		return out, nil
	}
	before, after, err := renderOkfSidecar(root, plan.TargetVersion)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(before, after) {
		out[".okf"] = [2][]byte{before, after}
	}
	before, after, err = declareRootIndexVersion(root, plan.TargetVersion)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(before, after) {
		out["index.md"] = [2][]byte{before, after}
	}
	return out, nil
}

// renderMigratedNode returns the node file bytes with the planned deterministic
// edits applied to its frontmatter, order-preserving and additive: the only
// keys that change are `timestamp` (removed) and `generated` / `sources`
// (added, appended after the existing keys). Each edit re-checks the current
// frontmatter first: the rename is skipped once `generated` exists or
// `timestamp` is gone, a source whose resource is already listed is not
// added again, and a title is set only on a source that is still untitled
// (applySourceTitles), so a stale plan re-applied to a migrated file is a
// no-op.
// The edit is a splice (OKF v0.3
// §4): every untouched entry keeps its bytes, and the body is preserved
// verbatim. It
// is PURE — the single writer both --apply and its --dry-run render through, so
// a dry-run's rendered bytes are byte-identical to the applied bytes.
func renderMigratedNode(raw []byte, nm NodeMigration) ([]byte, error) {
	if _, _, ok := splitFrontmatterRaw(raw); !ok {
		// A node with no frontmatter block carries neither a timestamp nor a
		// body-citation edit that touches frontmatter; nothing to do.
		return raw, nil
	}
	m := parsedFrontmatterMapping(raw)
	if m == nil {
		return raw, nil
	}
	rename := nm.Generated != nil && mappingValue(m, "generated") == nil && mappingValue(m, "timestamp") != nil
	have := map[string]bool{}
	for _, s := range itemsOf(mappingValue(m, "sources")) {
		if r := mappingValue(s, "resource"); r != nil && r.Kind == yaml.ScalarNode {
			have[r.Value] = true
		}
	}
	var add []SourceEdit
	for _, s := range nm.Sources {
		if !have[s.Resource] {
			have[s.Resource] = true
			add = append(add, s)
		}
	}
	if !rename && len(add) == 0 {
		return applySourceTitles(raw, nm.Titles)
	}
	out, err := editFrontmatter(raw, func(root *yaml.Node) {
		if rename {
			removeKey(root, "timestamp")
			setMapping(root, "generated", []kv{{"by", nm.Generated.By}, {"at", nm.Generated.At}})
		}
		if len(add) > 0 {
			appendSources(root, add)
		}
	})
	if err != nil {
		return nil, err
	}
	return applySourceTitles(out, nm.Titles)
}

// kv is an ordered key/value pair for building a small mapping node.
type kv struct{ key, val string }

// setMapping sets key to a flow-style mapping of the given ordered pairs,
// preserving key order (replace in place, else append). Matches the §5.2
// `generated: { by, at }` flow form the spec's worked example uses.
func setMapping(mapping *yaml.Node, key string, pairs []kv) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	for _, p := range pairs {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: p.key},
			scalarValue(p.val),
		)
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = m
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, m)
}

// appendSources appends (or extends) a block-style `sources` sequence of
// `{ resource }` mappings, preserving order. An existing sources sequence is
// extended in place; otherwise a new key is appended after the existing keys.
func appendSources(mapping *yaml.Node, sources []SourceEdit) {
	seq := sourcesSequence(mapping)
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "sources"}, seq)
	}
	for _, s := range sources {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		if s.ID != "" {
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "id"},
				idScalar(s.ID),
			)
		}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "resource"},
			scalarValue(s.Resource),
		)
		if s.Title != "" {
			entry.Content = append(entry.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "title"},
				scalarValue(s.Title),
			)
		}
		seq.Content = append(seq.Content, entry)
	}
}

// scalarValue builds a scalar node, quoting when yaml would otherwise reparse
// the value as a non-string (so an actor like `human:casey` or a resource URL
// round-trips as the literal string the author meant).
func scalarValue(v string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	if needsQuote(v) {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n
}

// needsQuote reports whether a scalar must be quoted to round-trip as a string.
func needsQuote(v string) bool {
	return strings.ContainsAny(v, ":#{}[],&*!|>'\"%@`") || strings.TrimSpace(v) != v
}

// removeKey deletes key (and its value) from a mapping, preserving the order of
// the remaining keys. A no-op when the key is absent.
func removeKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// sourcesSequence returns the `sources` value as a sequence node, coercing an
// existing non-sequence value in place per OKF v0.3 §11.2: a null or empty
// value is an absent key (rule 1) and becomes an empty sequence, and any other
// single item becomes a one-item sequence (rule 2). Without this, extending a
// null or single-mapping `sources` would append a duplicate key. Returns nil
// only when the key is absent.
func sourcesSequence(mapping *yaml.Node) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "sources" {
			continue
		}
		v := mapping.Content[i+1]
		if v.Kind == yaml.SequenceNode {
			return v
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		nullish := v.Kind == yaml.ScalarNode && (v.Tag == "!!null" || v.Value == "")
		if !nullish {
			seq.Content = []*yaml.Node{v}
		}
		mapping.Content[i+1] = seq
		return seq
	}
	return nil
}

// citedElsewhere reports whether an entry other than skip names key.
func citedElsewhere(names []map[string]bool, skip int, key string) bool {
	for j, n := range names {
		if j != skip && n[key] {
			return true
		}
	}
	return false
}

// bumpVersionMarkers declares target in the bundle-root index.md (§8.1, §12)
// and in the legacy .okf sidecar when one exists. Each is a no-op when it
// already declares target (idempotence).
func bumpVersionMarkers(root, target string) error {
	if err := bumpOkfSidecar(root, target); err != nil {
		return err
	}
	return bumpRootIndexMarker(root, target)
}

// bumpOkfSidecar rewrites the okf_version line of the .okf sidecar to the
// quoted target (§4: quote number-like values), preserving every other line.
// A sidecar that already declares target, quoted or not, is left alone. An
// absent sidecar is a no-op: it is a legacy fallback and is never created.
func bumpOkfSidecar(root, target string) error {
	before, after, err := renderOkfSidecar(root, target)
	if err != nil || bytes.Equal(before, after) {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".okf"), after, 0o644) //nolint:gosec // G306: the .okf sidecar is a shareable bundle artifact; 0o644 is intended
}

// renderOkfSidecar returns the sidecar's current bytes and its bytes with
// okf_version set to target. Both are nil when the sidecar is absent.
func renderOkfSidecar(root, target string) ([]byte, []byte, error) {
	abs := filepath.Join(root, ".okf")
	if escapesBundleRoot(root, abs) { // §3.2 rule 5, §11.2
		return nil, nil, fmt.Errorf("%s is a symlink that resolves outside the bundle root; not read (OKF §3.2, §11.2)", ".okf")
	}
	data, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle .okf sidecar
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read .okf: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		key, val, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "okf_version" {
			continue
		}
		if strings.Trim(strings.TrimSpace(val), `"'`) == target {
			continue
		}
		lines[i] = "okf_version: " + strconv.Quote(target)
	}
	return data, []byte(strings.Join(lines, "\n")), nil
}

// bumpRootIndexMarker declares okf_version target on the bundle-root index.md
// (§8.1: "Producers SHOULD put a bundle-root index.md with okf_version in each
// bundle"). Only the marker changes (see declareRootIndexVersion), so §8.1's
// MUST keep the existing root frontmatter and §4's SHOULD NOT re-serialize
// unchanged values both hold.
func bumpRootIndexMarker(root, target string) error {
	before, after, err := declareRootIndexVersion(root, target)
	if err != nil || bytes.Equal(before, after) {
		return err
	}
	return os.WriteFile(filepath.Join(root, "index.md"), after, 0o644) //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
}
