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
	"sort"
	"strings"
	"time"
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

// bundleRootOkfVersion resolves the okf_version to emit on the bundle-ROOT
// index frontmatter, per the marker-compatibility contract (OKF §11):
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

// renderIndexFrontmatter emits the bundle-root index frontmatter block. Per OKF
// §6 an index.md contains no frontmatter, with a single §11 carve-out: the
// bundle-root index MAY carry a frontmatter block containing okf_version — "the
// only place frontmatter is permitted in an index.md" — and nothing else. So
// this emits an okf_version-only block when the marker-compatibility contract
// says the key should be present (see bundleRootOkfVersion), and emits NO
// frontmatter otherwise. `type: Index` is never emitted. The value is quoted so
// the marker is an unambiguous YAML string regardless of how it was declared.
func renderIndexFrontmatter(b *Bundle) string {
	if v, ok := bundleRootOkfVersion(b); ok {
		return fmt.Sprintf("---\nokf_version: %q\n---\n\n# Knowledge Base\n\n", v)
	}
	return "# Knowledge Base\n\n"
}

// RenderIndex produces a deterministic, neighborhood-grouped index.md body for
// the bundle: an optional okf_version-only frontmatter block (§11) on the
// bundle root, then one section per top-level neighborhood (sorted), each
// listing its concept nodes (sorted by path) as titled markdown links annotated
// with the node's type. Reserved files are excluded. The output is byte-stable
// across runs (all ordering is via sort), carries NO frontmatter beyond the
// §11 okf_version carve-out (§6), and passes Validate.
//
// The bundle-root index frontmatter preserves an existing okf_version marker (or
// adopts a .okf sidecar's), so `index build` cannot silently strip the key the
// OKF corpus loader uses to discover bundle roots. See bundleRootOkfVersion.
func RenderIndex(b *Bundle) string {
	groups := map[string][]string{}
	for p := range b.Nodes {
		nb := neighborhood(p)
		groups[nb] = append(groups[nb], p)
	}
	names := make([]string, 0, len(groups))
	for nb := range groups {
		names = append(names, nb)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString(renderIndexFrontmatter(b))
	if len(names) == 0 {
		sb.WriteString("_No nodes yet._\n")
		return sb.String()
	}
	for _, nb := range names {
		heading := nb
		if heading == "" {
			heading = "(root)"
		}
		sb.WriteString("\n## " + heading + "\n\n")
		paths := groups[nb]
		sort.Strings(paths)
		for _, p := range paths {
			n := b.Nodes[p]
			sb.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", nodeTitle(n), p, n.Type()))
		}
	}
	return sb.String()
}

// WriteIndex regenerates index.md on disk from the current bundle. It is the
// single writer for the reserved index (both `index build` and the automatic
// create/edit/delete/rename maintenance call it) so the two paths cannot
// diverge on how the index is produced.
func WriteIndex(b *Bundle) error {
	return os.WriteFile(filepath.Join(b.Root, "index.md"), []byte(RenderIndex(b)), 0o644)
}

// IndexInSync reports whether the on-disk index.md matches what RenderIndex would
// generate for the current bundle. When stale, it returns a short human-readable
// report. A missing index.md counts as out of sync.
func IndexInSync(b *Bundle) (bool, string) {
	want := RenderIndex(b)
	onDisk, err := os.ReadFile(filepath.Join(b.Root, "index.md"))
	if err != nil {
		return false, "index.md is missing or unreadable; run `okfctl index build`"
	}
	if string(onDisk) == want {
		return true, ""
	}
	return false, "index.md is out of date; run `okfctl index build` to regenerate"
}

// AppendLog prepends a timestamped entry to log.md (newest-first), creating the
// file with a heading when absent. A multi-line message is flattened to its first
// line to keep the log well-formed; an empty message is rejected.
func AppendLog(root, message string) error {
	message = strings.TrimSpace(message)
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		message = message[:i]
	}
	if message == "" {
		return fmt.Errorf("log message must not be empty")
	}
	entry := fmt.Sprintf("- %s — %s\n", time.Now().UTC().Format("2006-01-02"), message)

	p := filepath.Join(root, "log.md")
	existing, err := os.ReadFile(p)
	if err != nil {
		return os.WriteFile(p, []byte(logHeader+entry), 0o644)
	}
	// Strip the header, then drop the scaffold placeholder when it is the only
	// content — otherwise the fresh-scaffold "no entries yet" hint stays pinned
	// below every real entry.
	rest := strings.TrimPrefix(string(existing), logHeader)
	rest = strings.TrimPrefix(rest, logPlaceholder)
	return os.WriteFile(p, []byte(logHeader+entry+rest), 0o644)
}

// ReadLog returns the log.md body (empty string if the file is absent).
func ReadLog(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "log.md"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
