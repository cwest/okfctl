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
	"strings"

	"gopkg.in/yaml.v3"
)

// Migration from OKF v0.1 to v0.2. It applies the §13.1 breaking renames
// (`timestamp` → `generated.at`; body `# Citations` → frontmatter `sources`)
// plus the §8/§12 version markers, and does so in TWO phases so the tool never
// acquires a model dependency:
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

// TargetOkfVersion is the version this migrator upgrades a bundle TO. It is a
// migrate-local constant, deliberately independent of SpecVersion (the build's
// pinned version): closing the SpecVersion gap is separate, tracked work.
const TargetOkfVersion = "0.2"

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
}

// NodeMigration is the set of deterministic frontmatter edits planned for one
// node. A nil Generated means no timestamp rename is planned; an empty Sources
// means no citation became a source. Only nodes with at least one edit appear.
type NodeMigration struct {
	Path      string         `json:"path"`
	Generated *GeneratedEdit `json:"generated,omitempty"`
	Sources   []SourceEdit   `json:"sources,omitempty"`
}

// MigratePlan is the full, JSON-serializable migration plan: the deterministic
// per-node edits, the enumerated judgment items, and the target version. It is
// the review surface — for Casey and for a solo user alike.
type MigratePlan struct {
	TargetVersion string          `json:"target_version"`
	Nodes         []NodeMigration `json:"nodes"`
	Judgment      []JudgmentItem  `json:"judgment"`
}

// citationItemRe matches a single `# Citations` entry line and captures its text
// after any bullet / bracket-key / ordered-list marker. It mirrors the entry
// shapes analyze.go's citationEntryRe / citationOrderedRe recognize.
var citationItemRe = regexp.MustCompile(`^\s*(?:[-*]\s*)?(?:(?:\*\*)?\[[A-Za-z0-9]+\](?:\*\*)?|\d+\.)?\s*(.*\S)\s*$`)

// bareURLRe matches a leading bare URL in a citation item.
var bareURLRe = regexp.MustCompile(`^<?(https?://[^\s>]+)>?`)

// mdLinkOnlyRe matches a markdown link and captures its target.
var mdLinkTargetRe = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)`)

// PlanMigration computes the v0.1 → v0.2 migration plan for a loaded bundle. It
// is PURE: it reads the bundle and writes nothing. generatedBy is the actor (§7)
// used for every `timestamp` rename; when empty and no actor is inferable, each
// such node becomes a JudgmentMissingActor item instead of a guessed rename.
func PlanMigration(b *Bundle, generatedBy string) (MigratePlan, error) {
	plan := MigratePlan{TargetVersion: TargetOkfVersion}

	for _, rel := range sortedNodePaths(b) {
		n := b.Nodes[rel]
		if n.Frontmatter == nil {
			continue // unparseable frontmatter is a different failure class
		}
		nm := NodeMigration{Path: rel}

		// §13.1: timestamp -> generated { by, at }. Only when generated is not
		// already present (idempotence) and a legacy timestamp exists.
		if _, has := n.Frontmatter["generated"]; !has {
			if ts, ok := n.Frontmatter["timestamp"]; ok {
				atStr := frontmatterTimeString(ts)
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

		// §13.1: body # Citations -> frontmatter sources. Only items carrying a
		// follow-able resource are deterministic; a prose finding is a judgment
		// item (§5.1 requires resource; never fabricate one). Idempotence: skip
		// a resource already present in the node's sources.
		existing := existingSourceResources(n)
		for _, item := range parseCitationItems(n.Body) {
			res, ok := citationResource(item)
			if !ok {
				plan.Judgment = append(plan.Judgment, JudgmentItem{
					Path: rel, Kind: JudgmentProseCitation, Context: item,
				})
				continue
			}
			if existing[res] {
				continue // already migrated (idempotent)
			}
			nm.Sources = append(nm.Sources, SourceEdit{Resource: res})
			existing[res] = true
		}

		if nm.Generated != nil || len(nm.Sources) > 0 {
			plan.Nodes = append(plan.Nodes, nm)
		}
	}
	return plan, nil
}

// MigrateApply applies a plan's deterministic edits to disk, order-preserving
// and additive-only. root is the bundle root; b is the loaded bundle the plan
// was computed against. It writes each edited node in place, then bumps the
// version markers (.okf sidecar and bundle-root index.md). It never touches a
// judgment item — those stay for the plan's consumer.
func MigrateApply(root string, b *Bundle, plan MigratePlan) error {
	for _, nm := range plan.Nodes {
		abs := filepath.Join(root, filepath.FromSlash(nm.Path))
		raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading a node from the user's own bundle
		if err != nil {
			return fmt.Errorf("read %s: %w", nm.Path, err)
		}
		out, err := renderMigratedNode(raw, nm)
		if err != nil {
			return fmt.Errorf("migrate %s: %w", nm.Path, err)
		}
		if bytes.Equal(out, raw) {
			continue
		}
		if err := os.WriteFile(abs, out, 0o644); err != nil { //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
			return fmt.Errorf("write %s: %w", nm.Path, err)
		}
	}
	return bumpVersionMarkers(root, plan.TargetVersion)
}

// renderMigratedNode returns the node file bytes with the planned deterministic
// edits applied to its frontmatter, order-preserving and additive: the only
// keys that change are `timestamp` (removed) and `generated` / `sources`
// (added, appended after the existing keys). The body is preserved verbatim. It
// is PURE — the single writer both --apply and its --dry-run render through, so
// a dry-run's rendered bytes are byte-identical to the applied bytes.
func renderMigratedNode(raw []byte, nm NodeMigration) ([]byte, error) {
	yamlBlock, rawAfter, ok := splitFrontmatterRaw(raw)
	if !ok {
		// A node with no frontmatter block carries neither a timestamp nor a
		// body-citation edit that touches frontmatter; nothing to do.
		return raw, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	root := frontmatterMapping(&doc)
	if root == nil {
		return nil, fmt.Errorf("frontmatter is not a mapping")
	}

	if nm.Generated != nil {
		removeKey(root, "timestamp")
		setMapping(root, "generated", []kv{{"by", nm.Generated.By}, {"at", nm.Generated.At}})
	}
	if len(nm.Sources) > 0 {
		appendSources(root, nm.Sources)
	}

	var fmBuf bytes.Buffer
	enc := yaml.NewEncoder(&fmBuf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
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
	seq := findSequence(mapping, "sources")
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "sources"}, seq)
	}
	for _, s := range sources {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "resource"},
			scalarValue(s.Resource),
		)
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

// findSequence returns the sequence node value of key, or nil.
func findSequence(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key && mapping.Content[i+1].Kind == yaml.SequenceNode {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// existingSourceResources returns the set of resources already present in a
// node's frontmatter `sources`, so a re-plan on an already-migrated node does
// not re-add them (idempotence).
func existingSourceResources(n *Node) map[string]bool {
	out := map[string]bool{}
	for _, s := range n.Sources() {
		out[s.Resource] = true
	}
	return out
}

// citationBulletRe matches a plain bullet entry (`- ...` / `* ...`) that the
// analyze.go entry regexes (which require a bracket key or ordered marker) do
// not — the spec's own v0.1 worked example lists citations as bare bullets.
var citationBulletRe = regexp.MustCompile(`^\s*[-*]\s+\S`)

// parseCitationItems returns the verbatim entry lines under a `# Citations`
// heading (legend lines excluded), in body order. It recognizes the bracket-key
// and ordered-list shapes analyze.go's citationCount uses, plus plain bullets
// (the spec's v0.1 example lists sources as bare `-` bullets). A blank line ends
// the section so trailing prose is not swept in.
func parseCitationItems(body string) []string {
	loc := citationsHeadRe.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(body[loc[1]:], "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			break // next heading ends the Citations section
		}
		if citationLegendRe.MatchString(line) {
			continue
		}
		if citationEntryRe.MatchString(line) || citationOrderedRe.MatchString(line) || citationBulletRe.MatchString(line) {
			out = append(out, strings.TrimRight(line, " 	"))
		}
	}
	return out
}

// citationResource extracts the follow-able resource from a citation item: a
// markdown-link target (kept in the author's form per §6), or a leading bare
// URL. It returns ok=false for a prose finding with no resource — §5.1 requires
// a resource, so such an item is a judgment call, never a fabricated source.
func citationResource(item string) (string, bool) {
	m := citationItemRe.FindStringSubmatch(item)
	text := item
	if m != nil {
		text = strings.TrimSpace(m[1])
	}
	if link := mdLinkTargetRe.FindStringSubmatch(text); link != nil {
		return strings.TrimSpace(link[1]), true
	}
	if url := bareURLRe.FindStringSubmatch(text); url != nil {
		return url[1], true
	}
	return "", false
}

// bumpVersionMarkers sets the bundle's declared version to target in both the
// .okf sidecar (§12) and the bundle-root index.md okf_version marker (§8/§12 —
// the only place index frontmatter is permitted). Each is a no-op when already
// at target (idempotence) or absent.
func bumpVersionMarkers(root, target string) error {
	if err := bumpOkfSidecar(root, target); err != nil {
		return err
	}
	return bumpRootIndexMarker(root, target)
}

// bumpOkfSidecar rewrites the okf_version line of the .okf sidecar to target,
// preserving every other line. Absent sidecar is a no-op.
func bumpOkfSidecar(root, target string) error {
	abs := filepath.Join(root, ".okf")
	data, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle .okf sidecar
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read .okf: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		key, _, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "okf_version" {
			newLine := "okf_version: " + target
			if lines[i] != newLine {
				lines[i] = newLine
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	return os.WriteFile(abs, []byte(strings.Join(lines, "\n")), 0o644) //nolint:gosec // G306: the .okf sidecar is a shareable bundle artifact; 0o644 is intended
}

// bumpRootIndexMarker rewrites the bundle-root index.md okf_version marker to
// target, order-preserving and body-verbatim. Absent index or absent marker is
// a no-op — this never invents a marker where the author had none.
func bumpRootIndexMarker(root, target string) error {
	abs := filepath.Join(root, "index.md")
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle index.md
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read index.md: %w", err)
	}
	yamlBlock, rawAfter, ok := splitFrontmatterRaw(raw)
	if !ok {
		return nil // no frontmatter: nothing to bump
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return fmt.Errorf("parse index.md frontmatter: %w", err)
	}
	m := frontmatterMapping(&doc)
	if m == nil {
		return nil
	}
	cur := findScalar(m, "okf_version")
	if cur == "" || cur == target {
		return nil // no marker to bump, or already at target (idempotent no-op)
	}
	setScalar(m, "okf_version", target)

	var fmBuf bytes.Buffer
	enc := yaml.NewEncoder(&fmBuf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode index.md frontmatter: %w", err)
	}
	_ = enc.Close()

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(fmBuf.Bytes())
	out.WriteString("---\n")
	out.Write(rawAfter)
	if bytes.Equal(out.Bytes(), raw) {
		return nil
	}
	return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
}

// findScalar returns the scalar value of key in a mapping, or "".
func findScalar(mapping *yaml.Node, key string) string {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1].Value
		}
	}
	return ""
}
