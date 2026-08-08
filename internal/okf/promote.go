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
)

// PromoteChange is a single planned promotion of one directory-as-concept
// index.md into a sibling concept file, together with the inbound-link rewrites
// that keep every reference to it resolving after the move. OldPath is the
// non-root index.md that carries frontmatter; NewPath is the sibling concept
// file it becomes (dir/<basename>.md). Rewrites are the edits to OTHER files'
// bodies whose links pointed at the old directory-concept in either directory
// spelling.
type PromoteChange struct {
	OldPath  string // bundle-relative, e.g. "gke-pm-map/index.md"
	NewPath  string // bundle-relative, e.g. "gke-pm-map/gke-pm-map.md"
	Rewrites []LinkRewrite
}

// PromotableIndexes returns the sorted bundle-relative paths of every NON-ROOT
// index.md that carries a non-empty frontmatter block — the exact shape
// validateReserved flags as "index files contain no frontmatter (§8)". The
// bundle-root index.md is excluded (its §12 okf_version carve-out is legal), and
// a non-root index with no frontmatter is already conformant and excluded. A
// non-root index whose frontmatter failed to parse (Frontmatter == nil) is a
// different failure class and is not promotable — promote does not guess at
// broken YAML.
func PromotableIndexes(b *Bundle) []string {
	var out []string
	for rel, n := range b.Reserved {
		if path.Base(rel) != "index.md" {
			continue
		}
		if rel == "index.md" {
			continue // bundle-root index: §12 carve-out, never promoted
		}
		if len(n.Frontmatter) == 0 {
			continue // unparseable (different class) or conformant (nothing to do)
		}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

// PromotePlan computes the promotion of every directory-as-concept index into a
// sibling concept file, plus the inbound-link rewrites needed to keep references
// resolving. basename is the concept file's base name for every promoted node
// ("" means default to the directory's own base name). It is PURE: it reads the
// loaded bundle and writes nothing to disk.
//
// A destination that already exists as a concept node is a hard error — promote
// never overwrites authored content.
func PromotePlan(b *Bundle, basename string) ([]PromoteChange, error) {
	indexes := PromotableIndexes(b)

	// Compute each promotion's destination and validate up front.
	type promo struct {
		oldPath string
		dir     string
		newPath string
	}
	promos := make([]promo, 0, len(indexes))
	for _, idx := range indexes {
		dir := path.Dir(idx) // e.g. "gke-pm-map/index.md" -> "gke-pm-map"
		base := basename
		if base == "" {
			base = path.Base(dir)
		}
		newPath := path.Join(dir, base+".md")
		if b.Nodes[newPath] != nil {
			return nil, fmt.Errorf("cannot promote %s: destination %s already exists as a node", idx, newPath)
		}
		promos = append(promos, promo{oldPath: idx, dir: dir, newPath: newPath})
	}

	// dir -> new concept path, for link resolution.
	newFor := map[string]string{}
	for _, p := range promos {
		newFor[p.dir] = p.newPath
	}

	// Build inbound-link rewrites by scanning every concept node AND reserved
	// file body for links that resolve to a promoted DIRECTORY in either the
	// "dir/index.md" or "dir/" spelling. A reserved file (index.md especially)
	// confers navigation, so its links are rewritten too.
	changes := make([]PromoteChange, len(promos))
	for i, p := range promos {
		changes[i] = PromoteChange{OldPath: p.oldPath, NewPath: p.newPath}
	}
	idxOf := map[string]int{}
	for i, p := range promos {
		idxOf[p.dir] = i
	}

	scan := func(srcPath, body string) {
		srcDir := path.Dir(srcPath)
		for _, dl := range scanDirLinks(body) {
			targetDir, ok := resolvePromotedDir(srcDir, dl.url, newFor)
			if !ok {
				continue
			}
			ci := idxOf[targetDir]
			newConcept := newFor[targetDir]
			newURL := rewriteURLForm(srcDir, dl.url, targetDir, newConcept)
			changes[ci].Rewrites = append(changes[ci].Rewrites, LinkRewrite{
				NodePath: srcPath,
				Old:      dl.rawTarget,
				New:      newURL + dl.titleTail,
			})
		}
	}
	for srcPath, n := range b.Nodes {
		scan(srcPath, n.Body)
	}
	for srcPath, n := range b.Reserved {
		scan(srcPath, n.Body)
	}

	// Stable ordering of rewrites within each change.
	for i := range changes {
		rw := changes[i].Rewrites
		sort.Slice(rw, func(a, c int) bool {
			if rw[a].NodePath != rw[c].NodePath {
				return rw[a].NodePath < rw[c].NodePath
			}
			return rw[a].Old < rw[c].Old
		})
	}
	return changes, nil
}

// dirLink is one raw markdown link found in a body, without node resolution —
// promote resolves against DIRECTORIES, not concept nodes, so it cannot use
// scanNodeLinks (which only surfaces links that already resolve to a node).
type dirLink struct {
	rawTarget string // full capture incl. optional title, e.g. `foo/ "T"`
	url       string // first whitespace field, e.g. `foo/`
	titleTail string // remainder after url incl. leading space
}

// scanDirLinks returns every non-image, in-bundle markdown link in body (its
// raw target, url, and title tail). External (http/https/#) links are excluded.
// It intentionally does NOT resolve targets — the caller resolves each url
// against the set of promoted directories.
func scanDirLinks(body string) []dirLink {
	var out []dirLink
	for _, loc := range mdLinkRe.FindAllStringSubmatchIndex(body, -1) {
		if loc[0] > 0 && body[loc[0]-1] == '!' {
			continue // image, not a link
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
		out = append(out, dirLink{rawTarget: raw, url: url, titleTail: raw[len(url):]})
	}
	return out
}

// resolvePromotedDir reports the promoted directory a link url points at, if
// any, in either the "dir/index.md" or "dir/" spelling. srcDir is the linking
// file's directory ("" at the bundle root). Resolution mirrors the shared
// edge-builder's three forms: "/"-absolute (bundle-root relative), root-relative
// (from the bundle root), then dir-relative (from srcDir). Only a url that
// resolves to a directory present in newFor (a directory being promoted) matches.
func resolvePromotedDir(srcDir, url string, newFor map[string]string) (string, bool) {
	// Strip a trailing "index.md" or normalise a trailing slash so both
	// spellings collapse to the directory they name.
	dirPart, ok := linkDirPart(url)
	if !ok {
		return "", false
	}
	var candidate string
	if strings.HasPrefix(dirPart, "/") {
		candidate = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(dirPart, "/")))
	} else if c := filepath.ToSlash(filepath.Clean(dirPart)); newFor[c] != "" {
		candidate = c
	} else {
		candidate = filepath.ToSlash(filepath.Clean(path.Join(srcDir, dirPart)))
	}
	if candidate == "." {
		candidate = ""
	}
	if newFor[candidate] != "" {
		return candidate, true
	}
	return "", false
}

// linkDirPart reduces a link url to the directory it names when the url is a
// directory-style reference to an index — either "<dir>/index.md" or "<dir>/"
// (trailing slash). It returns the directory portion and true; for any other
// url (a plain concept file, an anchor-only link, etc.) it returns false. A
// bare "index.md" or "/index.md" reduces to the root directory ("" / "/").
func linkDirPart(url string) (string, bool) {
	// Drop any anchor.
	if i := strings.IndexByte(url, '#'); i >= 0 {
		url = url[:i]
	}
	switch {
	case strings.HasSuffix(url, "/index.md"):
		return strings.TrimSuffix(url, "index.md"), true // keep trailing slash
	case url == "index.md":
		return "", true
	case url == "/index.md":
		return "/", true
	case strings.HasSuffix(url, "/"):
		return url, true
	default:
		return "", false
	}
}

// rewriteURLForm renders the new link url pointing at the promoted concept file,
// preserving the author's relative form: a "/"-absolute link stays absolute, a
// root-relative link stays root-relative, and a dir-relative link is recomputed
// relative to the linking file's directory. srcDir is the linking file's
// directory; oldURL is the original link text; targetDir is the promoted
// directory; newConcept is its new bundle-relative concept path.
func rewriteURLForm(srcDir, oldURL, targetDir, newConcept string) string {
	base := oldURL
	if i := strings.IndexByte(base, '#'); i >= 0 {
		base = base[:i]
	}
	switch {
	case strings.HasPrefix(base, "/"):
		return "/" + newConcept
	case newConceptIsRootRel(base, targetDir):
		return newConcept
	default:
		// dir-relative: recompute relative to the linking file's directory.
		r, err := filepath.Rel(srcDir, filepath.FromSlash(newConcept))
		if err != nil {
			return newConcept
		}
		return filepath.ToSlash(r)
	}
}

// newConceptIsRootRel reports whether a non-absolute link url was written
// root-relative (its cleaned directory part equals the promoted directory from
// the bundle root) rather than relative to the linking file's own directory.
func newConceptIsRootRel(url, targetDir string) bool {
	dirPart, ok := linkDirPart(url)
	if !ok {
		return false
	}
	c := filepath.ToSlash(filepath.Clean(dirPart))
	if c == "." {
		c = ""
	}
	return c == targetDir
}

// PromoteApply performs each planned promotion on disk: it applies the inbound
// link rewrites, writes the promoted directory-concept index into its new
// sibling concept file with the body preserved VERBATIM and `created` immutable,
// and removes the old index.md (a clean, frontmatter-free index is regenerated
// by WriteIndex at the command layer). root is the bundle root; b is the loaded
// bundle the plan was computed against. It is the single writer for a promotion.
func PromoteApply(root string, b *Bundle, changes []PromoteChange) error {
	// 1) Apply link rewrites to the FULL on-disk file content, so the YAML
	//    frontmatter block is preserved by construction and the scanner's byte
	//    offsets and content stay in the same coordinate space.
	edited := map[string]string{}
	for _, ch := range changes {
		for _, rw := range ch.Rewrites {
			content, ok := edited[rw.NodePath]
			if !ok {
				raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rw.NodePath))) //nolint:gosec // G304: reading a node from the user's own bundle
				if err != nil {
					return fmt.Errorf("read %s: %w", rw.NodePath, err)
				}
				content = string(raw)
			}
			replaced := false
			for _, dl := range scanDirLinks(content) {
				if dl.rawTarget == rw.Old {
					// Locate this exact raw target and replace the first match.
					if idx := indexOfLink(content, dl.rawTarget); idx >= 0 {
						content = content[:idx] + rw.New + content[idx+len(dl.rawTarget):]
						replaced = true
						break
					}
				}
			}
			if !replaced {
				return fmt.Errorf("could not locate link %q in %s", rw.Old, rw.NodePath)
			}
			edited[rw.NodePath] = content
		}
	}
	for p, content := range edited {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil { //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
			return fmt.Errorf("write %s: %w", p, err)
		}
	}

	// 2) Move each directory-concept index into its new concept file (body
	//    verbatim, created immutable), then remove the old index.md.
	for _, ch := range changes {
		oldAbs := filepath.Join(root, filepath.FromSlash(ch.OldPath))
		newAbs := filepath.Join(root, filepath.FromSlash(ch.NewPath))
		raw, err := os.ReadFile(oldAbs) //nolint:gosec // G304: reading a node from the user's own bundle
		if err != nil {
			return fmt.Errorf("read %s: %w", ch.OldPath, err)
		}
		if _, err := os.Stat(newAbs); err == nil {
			return fmt.Errorf("destination already exists: %s", ch.NewPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", ch.NewPath, err)
		}
		// The promoted file is byte-identical to the old index — same
		// frontmatter (created untouched) and the same verbatim body. Only its
		// PATH changes; that alone flips it from an illegal directory index to a
		// legal concept node.
		if err := os.WriteFile(newAbs, raw, 0o644); err != nil { //nolint:gosec // G306: a bundle node is a shareable knowledge document; 0o644 is intended
			return fmt.Errorf("write %s: %w", ch.NewPath, err)
		}
		if err := os.Remove(oldAbs); err != nil {
			return fmt.Errorf("remove %s: %w", ch.OldPath, err)
		}
	}
	return nil
}

// indexOfLink returns the byte offset of the raw link target rawTarget as it
// appears inside a markdown link "](<rawTarget>)". It anchors on the "](" +
// rawTarget + ")" shape so a bare substring collision elsewhere in the body is
// never mistaken for the link target.
func indexOfLink(content, rawTarget string) int {
	needle := "](" + rawTarget + ")"
	if i := strings.Index(content, needle); i >= 0 {
		return i + len("](")
	}
	return -1
}
