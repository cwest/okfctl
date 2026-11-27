// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
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
	"path/filepath"
	"sort"
)

// LinkRewrite is a single planned edit to a node's body: replace the link
// target text Old with New, preserving the author's relative link form.
type LinkRewrite struct {
	NodePath string // bundle-relative path of the node whose body is edited
	Old      string // exact link target text being replaced (URL + optional title)
	New      string // replacement target text (same relative form as Old)
}

// PlanMove computes the inbound-link rewrites needed to move old->new. It is
// pure (no disk access) and preserves each author's §6 link form: a
// "/"-rooted link stays "/"-rooted, and a file-relative link is recomputed
// relative to the linking node's directory. A #fragment and a CommonMark
// title are kept.
func PlanMove(b *Bundle, old, new string) ([]LinkRewrite, error) {
	old = filepath.ToSlash(filepath.Clean(old))
	new = filepath.ToSlash(filepath.Clean(new))
	if IsReservedPath(old) || IsReservedPath(new) {
		return nil, fmt.Errorf("cannot move reserved file (%s -> %s)", old, new)
	}
	if IsDotPath(new) {
		return nil, dotPathMoveErr(new)
	}
	if b.Nodes[old] == nil {
		return nil, fmt.Errorf("node not found: %s", old)
	}
	if b.hasMarkdown(new) {
		return nil, fmt.Errorf("destination already exists: %s", new)
	}

	// Inbound links come from concepts AND from non-concept .md files (OKF
	// v0.3 §3.2 rule 2): a README.md that links the moved concept is not a
	// concept, but its link must keep resolving.
	var rewrites []LinkRewrite
	for path, n := range b.linkSources() {
		if path == old {
			continue // the moved node's own outbound links travel with it
		}
		for _, l := range scanNodeLinks(b, path, n.Body) {
			if l.resolved != old {
				continue
			}
			rewrites = append(rewrites, LinkRewrite{
				NodePath: path,
				Old:      l.rawTarget,
				New:      l.retarget(linkURLFor(path, new, l.form)),
			})
		}
	}
	sort.Slice(rewrites, func(i, j int) bool {
		if rewrites[i].NodePath != rewrites[j].NodePath {
			return rewrites[i].NodePath < rewrites[j].NodePath
		}
		return rewrites[i].Old < rewrites[j].Old
	})
	return rewrites, nil
}

// ApplyMove performs the file move old->new on disk and applies the planned
// inbound-link rewrites. It is the single writer for a move. root is the bundle
// root; b is the loaded bundle the plan was computed against.
func ApplyMove(root string, b *Bundle, old, new string, rewrites []LinkRewrite) error {
	old = filepath.ToSlash(filepath.Clean(old))
	new = filepath.ToSlash(filepath.Clean(new))
	if IsReservedPath(old) || IsReservedPath(new) {
		return fmt.Errorf("cannot move reserved file (%s -> %s)", old, new)
	}
	if IsDotPath(new) {
		return dotPathMoveErr(new)
	}
	oldAbs := filepath.Join(root, filepath.FromSlash(old))
	newAbs := filepath.Join(root, filepath.FromSlash(new))
	if _, err := os.Stat(newAbs); err == nil {
		return fmt.Errorf("destination already exists: %s", new)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", new, err)
	}

	// Apply link rewrites to the FULL on-disk file content (not the parsed
	// body) so the YAML frontmatter block is preserved by construction. The
	// scanner's byte offsets are relative to whatever text it scanned; scanning
	// the full file keeps offsets and content in the same coordinate space.
	// (A frontmatter block contains no node-resolving markdown link, so
	// scanning the whole file cannot introduce a spurious match.)
	edited := map[string]string{}
	for _, rw := range rewrites {
		if !b.hasMarkdown(rw.NodePath) {
			return fmt.Errorf("rewrite target not found: %s", rw.NodePath)
		}
		content, ok := edited[rw.NodePath]
		if !ok {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rw.NodePath))) //nolint:gosec // G304: reading a node from the user's own bundle
			if err != nil {
				return fmt.Errorf("read %s: %w", rw.NodePath, err)
			}
			content = string(raw)
		}
		var replaced bool
		for _, l := range scanNodeLinks(b, rw.NodePath, content) {
			if l.rawTarget == rw.Old && l.resolved == old {
				content = content[:l.capStart] + rw.New + content[l.capEnd:]
				replaced = true
				break
			}
		}
		if !replaced {
			return fmt.Errorf("could not locate link %q in %s", rw.Old, rw.NodePath)
		}
		edited[rw.NodePath] = content
	}
	for path, content := range edited {
		abs := filepath.Join(root, filepath.FromSlash(path))
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil { //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	// Bundle directories hold shareable knowledge documents; 0o755 is intended.
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil { //nolint:gosec // G301: shareable bundle content dir
		return fmt.Errorf("mkdir for %s: %w", new, err)
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("move %s -> %s: %w", old, new, err)
	}
	return nil
}

// dotPathMoveErr refuses a move destination that is not part of the bundle: a
// path with a segment starting with "." (OKF v0.3 §3.2 rule 4). The moved
// concept would silently drop out of every consumer's view.
func dotPathMoveErr(dst string) error {
	return fmt.Errorf("destination is not part of the bundle: %s (a file or directory name starting with \".\" is excluded, OKF v0.3 §3.2)", dst)
}

// PlanRemoveOrphans returns the bundle-relative nodes that become orphaned
// (zero inbound links) as a direct consequence of removing path. It is pure.
func PlanRemoveOrphans(b *Bundle, path string) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	if IsReservedPath(path) {
		return nil, fmt.Errorf("cannot remove reserved file: %s", path)
	}
	if b.Nodes[path] == nil {
		return nil, fmt.Errorf("node not found: %s", path)
	}
	// inbound[target] = count of distinct nodes linking to target (excluding
	// the node being removed).
	inbound := map[string]int{}
	for src := range b.Nodes {
		if src == path {
			continue
		}
		for _, dst := range b.OutboundLinks(src) {
			inbound[dst]++
		}
	}
	var orphaned []string
	for _, dst := range b.OutboundLinks(path) {
		if dst == path {
			continue
		}
		if inbound[dst] == 0 {
			orphaned = append(orphaned, dst)
		}
	}
	sort.Strings(orphaned)
	// De-dup (a node linked twice from the removed node).
	out := orphaned[:0]
	var prev string
	for i, o := range orphaned {
		if i == 0 || o != prev {
			out = append(out, o)
		}
		prev = o
	}
	return out, nil
}
