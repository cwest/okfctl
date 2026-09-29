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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// neighborhood returns the top-level directory of a bundle-relative slash path,
// or "" for a root-level node (rendered under a "(root)" group).
func neighborhood(rel string) string {
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

// nodeTitle returns the node's title frontmatter, falling back to the file's
// base name (without .md) when absent.
func nodeTitle(n *Node) string {
	if t, ok := n.Frontmatter["title"].(string); ok && strings.TrimSpace(t) != "" {
		return t
	}
	return strings.TrimSuffix(path.Base(n.Path), ".md")
}

// nodeDescription returns the node's description frontmatter (OKF §8: entries
// SHOULD carry the linked concept's description), or "" when absent, blank, or
// not a scalar string. A multi-line description is flattened to its first line
// so an index entry stays a single well-formed bullet.
func nodeDescription(n *Node) string {
	v, ok := n.Frontmatter["description"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// indexDir returns the bundle-relative directory of a slash path ("" for a
// root-level file), matching the coordinate space IndexDirs / RenderDirIndex use.
func indexDir(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

// dirTitle derives a human title for a directory that has no index.md of its
// own: the base name, first letter upper-cased. e.g. "wine/red" -> "Red".
func dirTitle(dir string) string {
	base := path.Base(dir)
	if base == "" || base == "." {
		return dir
	}
	return strings.ToUpper(base[:1]) + base[1:]
}

// IndexDirs returns the sorted bundle-relative directories that should carry an
// index.md (OKF §8: an index MAY appear in any directory and enumerates that
// directory's contents). A directory qualifies when it directly holds a concept
// node OR is an ancestor of a directory that does — exactly the set a reader
// traverses for progressive disclosure. The bundle root ("") is always included
// when the bundle has any concept node. Empty directories, and directories whose
// entire subtree has no concept, are excluded. This is the single source of
// truth shared by WriteIndex (build) and IndexInSync (check).
func IndexDirs(b *Bundle) []string {
	// The bundle root always carries an index (progressive disclosure starts at
	// the front door), even for an empty bundle.
	set := map[string]bool{"": true}
	for p := range b.Nodes {
		// Add the node's own directory and every ancestor up to the root.
		d := indexDir(p)
		for {
			set[d] = true
			if d == "" {
				break
			}
			d = indexDir(d)
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// childDirIndexTitleDesc resolves the title and description to render for a
// content-bearing child directory. When the child directory has its own
// index.md node on disk with a title/description frontmatter, those are used;
// otherwise the title falls back to a Title-cased directory name and the
// description is omitted. (Nested indexes carry no frontmatter per §8, so in
// practice this falls back to the derived title — but reading an existing
// index's frontmatter keeps the door open for a curator-authored one.)
func childDirIndexTitleDesc(b *Bundle, childDir string) (string, string) {
	if n, ok := b.Reserved[childDir+"/index.md"]; ok && n.Frontmatter != nil {
		title := dirTitle(childDir)
		if t, ok := n.Frontmatter["title"].(string); ok && strings.TrimSpace(t) != "" {
			title = strings.TrimSpace(t)
		}
		return title, nodeDescription(n)
	}
	return dirTitle(childDir), ""
}

// bundleRootOkfVersion resolves the okf_version to emit on the bundle-ROOT
// index frontmatter, per the marker-compatibility contract (OKF §12):
//
//  1. If the on-disk bundle-root index.md already declares okf_version, that
//     value is PRESERVED exactly (a curator-committed version is never bumped or
//     invented over).
//  2. Otherwise, if a .okf sidecar exists at the bundle root, its okf_version is
//     emitted (so a scaffolded bundle carries the marker its .okf pins).
//  3. Otherwise no marker is emitted — a bundle with neither an index-declared
//     key nor a .okf sidecar keeps the pre-existing behavior (no key).
//
// okf_version on the bundle-root index is the sole marker the OKF corpus loader
// uses to discover bundle roots, so `index build` must not drop it; this keeps
// okfctl's generated index compatible with that in-index marker convention while
// the .okf sidecar remains okfctl's own pin.
//
// The returned bool reports whether any marker should be emitted at all.
func bundleRootOkfVersion(b *Bundle) (string, bool) {
	if idx, ok := b.Reserved["index.md"]; ok {
		if v, ok := idx.Frontmatter["okf_version"]; ok {
			if s := strings.TrimSpace(scalarString(v)); s != "" {
				return s, true
			}
		}
	}
	if _, err := os.Stat(filepath.Join(b.Root, ".okf")); err == nil {
		if s := strings.TrimSpace(b.OkfVersion); s != "" {
			return s, true
		}
	}
	return "", false
}

// scalarString renders a YAML-decoded scalar frontmatter value as a string
// without Go's default map/struct formatting artifacts. okf_version is decoded
// as a string when quoted ("0.1") and may decode as a float or int when bare;
// %v gives a stable textual form for all scalar cases.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// rootFrontmatter emits the bundle-root index frontmatter block. Per OKF §8 an
// index.md contains no frontmatter, with a single §12 carve-out: the bundle-root
// index MAY carry a frontmatter block containing okf_version — "the only place
// frontmatter is permitted in an index.md" — and nothing else. So this emits an
// okf_version-only block when the marker-compatibility contract says the key
// should be present (see bundleRootOkfVersion), and "" otherwise. `type: Index`
// is never emitted. The value is quoted so the marker is an unambiguous YAML
// string regardless of how it was declared.
func rootFrontmatter(b *Bundle) string {
	if v, ok := bundleRootOkfVersion(b); ok {
		return fmt.Sprintf("---\nokf_version: %q\n---\n\n", v)
	}
	return ""
}

// dirHeading returns the top `# ` heading for a directory's index. The root
// index is titled "Knowledge Base"; a nested index is titled after its own
// directory (Title-cased base name).
func dirHeading(dir string) string {
	if dir == "" {
		return "# Knowledge Base\n"
	}
	return "# " + dirTitle(dir) + "\n"
}

// childDirsOf returns the sorted immediate content-bearing child directories of
// dir, derived from IndexDirs (the single source of truth for which directories
// carry an index). A child is "immediate" when its parent directory is dir.
func childDirsOf(b *Bundle, dir string) []string {
	var kids []string
	for _, d := range IndexDirs(b) {
		if d == "" {
			continue
		}
		if indexDir(d) == dir {
			kids = append(kids, d)
		}
	}
	sort.Strings(kids)
	return kids
}

// conceptsIn returns the sorted bundle-relative paths of concept nodes that live
// DIRECTLY in dir (not in a subdirectory).
func conceptsIn(b *Bundle, dir string) []string {
	var paths []string
	for p := range b.Nodes {
		if indexDir(p) == dir {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// entry renders one OKF §8 index bullet: `* [Title](url) - description`, or
// `* [Title](url)` when description is empty. The url is dir-relative.
func entry(title, url, desc string) string {
	if desc == "" {
		return fmt.Sprintf("* [%s](%s)\n", title, url)
	}
	return fmt.Sprintf("* [%s](%s) - %s\n", title, url, desc)
}

// subdirEntry renders one §8 Subdirectories bullet: the standard
// `* [Title](url) - description` (or the bare form when description is empty),
// with the tool-owned shape suffix appended on the SAME line before the newline.
// shape is "" when the shape feature is disabled (--no-shape), in which case the
// bullet is byte-identical to the pre-feature output.
func subdirEntry(title, url, desc, shape string) string {
	line := fmt.Sprintf("* [%s](%s)", title, url)
	if desc != "" {
		line += " - " + desc
	}
	line += shape
	return line + "\n"
}

// subdirBulletRE matches one §8 Subdirectories bullet, capturing the link
// target (group 2) and the trailing text after `](url)` (group 3). The trailing
// text holds an optional ` - description` and/or the tool-owned shape suffix; we
// separate those deterministically in preservedSubdirDescriptions rather than
// with a greedy regex, so a description that itself ends in `(…)` survives.
var subdirBulletRE = regexp.MustCompile(`^\* \[([^\]]*)\]\(([^)]+/)\)(.*)$`)

// toolShapeSuffixRE matches a tool-owned shape suffix at the END of a bullet's
// trailing text, STRUCTURALLY (not by equality against a freshly recomputed
// dirShape). The suffix dirShape renders is always a parenthesised group that
// begins with the mandatory concept-count segment (`N concept` / `N concepts`,
// see dirShape) optionally followed by ` · <types/tags>`, then an OPTIONAL
// ` · N in subtree` total. Anchoring on that count-segment signature is what
// makes stripping idempotent across builds: when the child's subtree changes,
// the recomputed suffix differs from the one on disk, but both still match this
// shape, so the stale one is removed rather than captured into the description
// (OKF §8: the description is curator-owned/verbatim, the suffix is
// tool-owned/regenerated). A curator description ending in `(draft)` does NOT
// begin with a concept count, so it is never mistaken for the suffix.
var toolShapeSuffixRE = regexp.MustCompile(` \(\d+ concepts?(?: · [^)]*)?\)(?: · \d+ in subtree)?$`)

// stripKnownShapeSuffix removes the tool-owned shape suffix from the tail of a
// bullet's trailing text. When shape rendering is DISABLED (--no-shape) the tool
// emits no suffix, so nothing is stripped and a description that happens to end
// in `(…)` is left fully intact. When shape is enabled, the last structural
// shape group (see toolShapeSuffixRE) is removed — matched by its shape, not by
// string-equality against what dirShape would render NOW, so a stale on-disk
// suffix from a build where the child's subtree differed is still stripped (OKF
// §8: the suffix is tool-owned and regenerated every build; the description is
// curator-owned and preserved verbatim).
func stripKnownShapeSuffix(trailing string, opts IndexShapeOptions) string {
	if !opts.Enabled {
		return trailing
	}
	return toolShapeSuffixRE.ReplaceAllString(trailing, "")
}

// preservedSubdirDescriptions parses the EXISTING on-disk index.md for dir and
// returns the curator-owned subdirectory descriptions, keyed by dir-relative
// link target (e.g. "topics/"). It is the mechanism behind §8 description
// preservation across `index build`: the tool regenerates headings, order,
// titles, entries, and the shape suffix, but a hand-written subdirectory
// description is curator-owned and must survive verbatim.
//
// Keying is by LINK TARGET, never by title — the title is regenerated every
// build, so a renamed-title entry (or one whose folder was removed) must still
// resolve its description by the stable `child/` link. A description is the text
// after ` - ` with the tool-owned shape suffix removed; the suffix is separated
// STRUCTURALLY (the last parenthesised concept-count group plus its optional
// `· N in subtree` tail — see stripKnownShapeSuffix), so the split is idempotent
// even when the child's subtree changed since the index was last written, and a
// curator description ending in `(…)` is never mistaken for the suffix.
//
// An entry whose link target is no longer content-bearing is simply not looked
// up by the renderer (it iterates the live child set), so a stale description on
// a vanished folder is never resurrected onto a different one.
func preservedSubdirDescriptions(b *Bundle, dir string, opts IndexShapeOptions) map[string]string {
	out := map[string]string{}
	idxRel := "index.md"
	if dir != "" {
		idxRel = dir + "/index.md"
	}
	idx, ok := b.Reserved[idxRel]
	if !ok || idx == nil {
		return out
	}
	for _, line := range strings.Split(idx.Body, "\n") {
		m := subdirBulletRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		linkTarget := m[2] // e.g. "topics/"
		trailing := m[3]   // e.g. " - Parts of speech. (1 concept · Concept)"
		// Strip the tool-owned shape suffix STRUCTURALLY (see stripKnownShapeSuffix
		// / toolShapeSuffixRE): the last parenthesised concept-count group plus its
		// optional `· N in subtree` tail. This is idempotent even when the child's
		// subtree changed since the on-disk index was written — the recomputed
		// suffix differs, but the stale one still matches the shape and is removed
		// rather than captured into the description. Under --no-shape nothing is
		// stripped, so a description ending in `(…)` survives intact.
		trailing = stripKnownShapeSuffix(trailing, opts)
		// What remains is either "" (bare entry) or " - description".
		desc := strings.TrimPrefix(trailing, " - ")
		if desc == trailing {
			// No " - " present: bare entry, no curator description.
			continue
		}
		desc = strings.TrimSpace(desc)
		if desc != "" {
			out[linkTarget] = desc
		}
	}
	return out
}

// RenderDirIndex produces the deterministic index.md body for one directory of
// the bundle (dir is bundle-relative slash form; "" is the bundle root), per OKF
// §8: it enumerates ONLY that directory's own immediate contents — its
// content-bearing child directories (linked dir-relatively as `child/`) under a
// "Subdirectories" section, and the concept nodes living directly in it (linked
// by base name) under a "Concepts" section, each carrying the linked concept's
// description from frontmatter. Links are relative to dir itself, never
// bundle-relative. Only the bundle-root index carries frontmatter (the §12
// okf_version carve-out); every nested index carries none. Output is byte-stable
// (all ordering via sort) and passes Validate.
//
// Subdirectory entries additionally carry the tool-owned shape suffix (see
// dirShape / IndexShapeOptions) so a reader can decide which branch to open
// without opening it — the whole point of §8 progressive disclosure. This is the
// zero-arg form used by build, check, and the auto-maintenance path; it renders
// with DefaultShapeOptions so those three paths stay byte-identical.
func RenderDirIndex(b *Bundle, dir string) string {
	return RenderDirIndexWithOptions(b, dir, DefaultShapeOptions())
}

// RenderDirIndexWithOptions is RenderDirIndex with an explicit shape config. It
// exists for the CLI flags (--shape-tag-min/--shape-tag-max/--no-shape), which
// must be threaded identically through build and check; the zero-arg
// RenderDirIndex delegates here with DefaultShapeOptions.
func RenderDirIndexWithOptions(b *Bundle, dir string, opts IndexShapeOptions) string {
	var sb strings.Builder
	if dir == "" {
		sb.WriteString(rootFrontmatter(b))
	}
	sb.WriteString(dirHeading(dir))

	kids := childDirsOf(b, dir)
	concepts := conceptsIn(b, dir)

	if len(kids) == 0 && len(concepts) == 0 {
		sb.WriteString("\n_No nodes yet._\n")
		return sb.String()
	}

	if len(kids) > 0 {
		sb.WriteString("\n## Subdirectories\n\n")
		// Curator-owned subdirectory descriptions preserved from the existing
		// on-disk index (OKF §8: the description is curator-owned; headings,
		// order, titles, entries, and the shape suffix are tool-owned). Keyed by
		// dir-relative link target so a regenerated title still resolves.
		preserved := preservedSubdirDescriptions(b, dir, opts)
		for _, child := range kids {
			title, desc := childDirIndexTitleDesc(b, child)
			// Precedence (§8): a preserved on-disk description wins; the child
			// index's frontmatter description is only the FALLBACK, so a first
			// build still honors it and every later build keeps the preserved
			// value even though nested indexes carry no frontmatter (§8).
			linkTarget := path.Base(child) + "/"
			if pd, ok := preserved[linkTarget]; ok {
				desc = pd
			}
			// Dir-relative link: the child's base name plus a trailing slash.
			// The shape suffix trails the §8 `[Title](url) - description`
			// grammar on the SAME bullet, so a consumer reading only that
			// grammar loses nothing.
			sb.WriteString(subdirEntry(title, linkTarget, desc, dirShape(b, child, opts)))
		}
	}
	if len(concepts) > 0 {
		sb.WriteString("\n## Concepts\n\n")
		for _, p := range concepts {
			n := b.Nodes[p]
			sb.WriteString(entry(nodeTitle(n), path.Base(p), nodeDescription(n)))
		}
	}
	return sb.String()
}

// RenderIndex renders the bundle-ROOT index. It is RenderDirIndex(b, "") — kept
// as a named helper so existing call sites and tests that mean "the root index"
// stay expressive.
func RenderIndex(b *Bundle) string {
	return RenderDirIndex(b, "")
}

// indexPathFor returns the on-disk absolute path of the index.md for a
// bundle-relative directory ("" -> <root>/index.md).
func indexPathFor(root, dir string) string {
	if dir == "" {
		return filepath.Join(root, "index.md")
	}
	return filepath.Join(root, filepath.FromSlash(dir), "index.md")
}

// WriteIndex regenerates one index.md per content-bearing directory (OKF §8),
// from the current bundle. It is the single writer for the reserved index (both
// `index build` and the automatic create/edit/delete/rename maintenance call it)
// so the two paths cannot diverge on how indexes are produced. Directories are
// created as needed; a directory that already holds concept files always exists,
// but IndexDirs may include an ancestor that is only implied by a deeper node.
//
// WriteIndex also self-heals the tree: an index.md left behind in a directory
// that is no longer content-bearing (e.g. after a node moved or was removed out
// of it) is pruned, so a subsequent `index check` is clean. This is the stale
// parent/sibling index class the pre-§8 flat model left behind.
func WriteIndex(b *Bundle) error {
	return WriteIndexWithOptions(b, DefaultShapeOptions())
}

// WriteIndexWithOptions is WriteIndex with an explicit shape config, for the CLI
// flags. The zero-arg WriteIndex delegates here with DefaultShapeOptions so the
// build/maintenance paths stay byte-identical to a default check.
func WriteIndexWithOptions(b *Bundle, opts IndexShapeOptions) error {
	want := map[string]bool{}
	for _, dir := range IndexDirs(b) {
		want[dir] = true
		p := indexPathFor(b.Root, dir)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // G301: shareable bundle content dir; 0o755 is intended
			return err
		}
		if err := os.WriteFile(p, []byte(RenderDirIndexWithOptions(b, dir, opts)), 0o644); err != nil { //nolint:gosec // G306: a bundle index file is shareable content; 0o644 is intended
			return err
		}
	}
	// Prune orphaned generated indexes: any on-disk index.md whose directory is
	// not in the content-bearing set.
	for relPath := range b.Reserved {
		if path.Base(relPath) != "index.md" {
			continue
		}
		if want[indexDir(relPath)] {
			continue
		}
		if err := os.Remove(indexPathFor(b.Root, indexDir(relPath))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// IndexInSync reports whether the on-disk nested index tree matches what
// WriteIndex would generate for the current bundle. It is in sync only when
// EVERY content-bearing directory has an index.md equal to its RenderDirIndex
// output AND no orphaned generated index.md exists in a directory that should
// carry none. A missing, stale, or orphaned index counts as out of sync, and
// the report names the first offending path.
func IndexInSync(b *Bundle) (bool, string) {
	return IndexInSyncWithOptions(b, DefaultShapeOptions())
}

// IndexInSyncWithOptions is IndexInSync with an explicit shape config, for the
// CLI flags. The zero-arg IndexInSync delegates here with DefaultShapeOptions.
// A build and a check MUST use the same options or the check reports drift — the
// CLI layer threads identical flags into both.
func IndexInSyncWithOptions(b *Bundle, opts IndexShapeOptions) (bool, string) {
	want := map[string]bool{}
	for _, dir := range IndexDirs(b) {
		want[dir] = true
		p := indexPathFor(b.Root, dir)
		onDisk, err := os.ReadFile(p) //nolint:gosec // G304: reading the tool's own generated index file
		if err != nil {
			return false, fmt.Sprintf("%s is missing or unreadable; run `okfctl index build`", filepath.ToSlash(rel(b.Root, p)))
		}
		if string(onDisk) != RenderDirIndexWithOptions(b, dir, opts) {
			return false, fmt.Sprintf("%s is out of date; run `okfctl index build` to regenerate", filepath.ToSlash(rel(b.Root, p)))
		}
	}
	// An index.md that exists in a directory NOT expected to carry one is an
	// orphan left behind by a delete/rename — stale until rebuilt.
	for relPath := range b.Reserved {
		if path.Base(relPath) != "index.md" {
			continue
		}
		if !want[indexDir(relPath)] {
			return false, fmt.Sprintf("%s is an orphaned index (its directory has no content); run `okfctl index build`", relPath)
		}
	}
	return true, ""
}

// rel returns the bundle-relative form of an absolute path, best-effort (the
// absolute path is used verbatim on failure, which only affects a report string).
func rel(root, abs string) string {
	if r, err := filepath.Rel(root, abs); err == nil {
		return r
	}
	return abs
}

// AppendLog records a change in log.md as an OKF v0.2 §9 date-grouped entry
// (newest-first), creating the file with a title when absent.
//
// §9 shape: a `# ` title, then `## YYYY-MM-DD` date headings each followed by
// `* <message>` entries, newest date first. AppendLog:
//   - keeps everything before the first `## ` heading verbatim (any H1 title),
//     writing logHeader only when creating a new file, and drops the scaffold
//     placeholder;
//   - regroups legacy `- YYYY-MM-DD — msg` bullets (the pre-fix inline-dated
//     form) into `* msg` entries under their date group, merging into an
//     existing `## date` group when present — so a log okfctl wrote before this
//     change converges to §9 the next time it is written (the invariant:
//     validate never rejects a log okfctl wrote);
//   - inserts the new entry as `* <message>` (no inline date): into today's
//     group when the newest heading is today, else under a new `## <today>`
//     heading inserted above the first heading.
//
// A multi-line message is flattened to its first line; an empty message is
// rejected.
func AppendLog(root, message string) error {
	message = strings.TrimSpace(message)
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		message = message[:i]
	}
	if message == "" {
		return fmt.Errorf("log message must not be empty")
	}
	today := nowUTC().Format("2006-01-02")

	p := filepath.Join(root, "log.md")
	existing, err := os.ReadFile(p) //nolint:gosec // G304: reading the user's own bundle log.md
	if err != nil {
		// Create a fresh §9 log with the default title.
		out := logHeader + "## " + today + "\n* " + message + "\n"
		return os.WriteFile(p, []byte(out), 0o644) //nolint:gosec // G306: log.md is shareable bundle content; 0o644 is intended
	}

	lg := parseLog(string(existing))
	lg.addEntry(today, message)
	return os.WriteFile(p, []byte(lg.render()), 0o644) //nolint:gosec // G306: log.md is shareable bundle content; 0o644 is intended
}

// ReadLog returns the log.md body (empty string if the file is absent).
func ReadLog(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "log.md")) //nolint:gosec // G304: reading the user's own bundle log.md
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
