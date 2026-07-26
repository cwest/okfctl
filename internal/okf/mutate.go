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
	"strings"
)

// LinkRewrite is a single planned edit to a node's body: replace the link
// target text Old with New, preserving the author's relative link form.
type LinkRewrite struct {
	NodePath string // bundle-relative path of the node whose body is edited
	Old      string // exact link target text being replaced (URL + optional title)
	New      string // replacement target text (same relative form as Old)
}

// linkForm distinguishes how a link target resolved to a node.
type linkForm int

const (
	formNone linkForm = iota
	formRootRel
	formDirRel
)

// scannedLink is one in-bundle markdown link found in a node body, with enough
// context for both edge-building and move-rewriting. Both buildEdges and
// PlanMove consume this single scanner so they can never disagree about what
// "links to X" means.
type scannedLink struct {
	rawTarget string   // full capture incl. optional title, e.g. `foo.md "T"`
	url       string   // first whitespace field, e.g. `foo.md`
	titleTail string   // remainder after url incl. leading space, e.g. ` "T"`
	resolved  string   // bundle-relative node the link resolves to ("" if none)
	form      linkForm // how it resolved (root-relative vs dir-relative)
	capStart  int      // byte offset of the url within the body
	capEnd    int      // byte offset of the raw capture end within the body
}

// scanNodeLinks returns every in-bundle markdown link in body, resolving each
// target the same two ways buildEdges does: root-relative first, then
// dir-relative against the linking node's directory (dir). Non-resolving,
// external (http/https/#), and image (![alt](src)) links are excluded.
func scanNodeLinks(b *Bundle, dir, body string) []scannedLink {
	var out []scannedLink
	for _, loc := range mdLinkRe.FindAllStringSubmatchIndex(body, -1) {
		// Skip images: a '!' immediately before '[' marks ![alt](src).
		if loc[0] > 0 && body[loc[0]-1] == '!' {
			continue
		}
		raw := body[loc[2]:loc[3]]
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		url := fields[0]
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "#") {
			continue
		}
		// titleTail is everything in the raw capture after the url (its
		// leading whitespace + any CommonMark title), preserved verbatim.
		titleTail := raw[len(url):]

		var resolved string
		var form linkForm
		if norm := filepath.ToSlash(filepath.Clean(url)); b.Nodes[norm] != nil {
			resolved, form = norm, formRootRel
		} else if rel := filepath.ToSlash(filepath.Clean(filepath.Join(dir, url))); b.Nodes[rel] != nil {
			resolved, form = rel, formDirRel
		} else {
			continue
		}
		out = append(out, scannedLink{
			rawTarget: raw,
			url:       url,
			titleTail: titleTail,
			resolved:  resolved,
			form:      form,
			capStart:  loc[2],
			capEnd:    loc[3],
		})
	}
	return out
}

// PlanMove computes the inbound-link rewrites needed to move old->new. It is
// pure (no disk access) and preserves each author's relative link form: a link
// that resolved root-relative stays root-relative; a link that resolved
// dir-relative is recomputed relative to the linking node's directory.
func PlanMove(b *Bundle, old, new string) ([]LinkRewrite, error) {
	old = filepath.ToSlash(filepath.Clean(old))
	new = filepath.ToSlash(filepath.Clean(new))
	if IsReservedPath(old) || IsReservedPath(new) {
		return nil, fmt.Errorf("cannot move reserved file (%s -> %s)", old, new)
	}
	if b.Nodes[old] == nil {
		return nil, fmt.Errorf("node not found: %s", old)
	}
	if b.Nodes[new] != nil {
		return nil, fmt.Errorf("destination already exists: %s", new)
	}

	var rewrites []LinkRewrite
	for path, n := range b.Nodes {
		if path == old {
			continue // the moved node's own outbound links travel with it
		}
		dir := filepath.Dir(path)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			if l.resolved != old {
				continue
			}
			var newURL string
			switch l.form {
			case formRootRel:
				newURL = new
			case formDirRel:
				r, err := filepath.Rel(dir, new)
				if err != nil {
					return nil, fmt.Errorf("relpath %s -> %s: %w", dir, new, err)
				}
				newURL = filepath.ToSlash(r)
			}
			rewrites = append(rewrites, LinkRewrite{
				NodePath: path,
				Old:      l.rawTarget,
				New:      newURL + l.titleTail,
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
		if b.Nodes[rw.NodePath] == nil {
			return fmt.Errorf("rewrite target not found: %s", rw.NodePath)
		}
		content, ok := edited[rw.NodePath]
		if !ok {
			raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rw.NodePath)))
			if err != nil {
				return fmt.Errorf("read %s: %w", rw.NodePath, err)
			}
			content = string(raw)
		}
		dir := filepath.Dir(rw.NodePath)
		var replaced bool
		for _, l := range scanNodeLinks(b, dir, content) {
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
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(newAbs), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", new, err)
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("move %s -> %s: %w", old, new, err)
	}
	return nil
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
