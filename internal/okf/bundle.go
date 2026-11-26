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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ReservedFiles are the reserved base names that are not concept nodes. A file
// named index.md or log.md is reserved at ANY depth: a knowledge base commonly
// keeps a per-neighborhood index.md and log.md (e.g. wine/index.md,
// security/auth/log.md), and those are structural files (front door /
// append-only change history), not concepts. Recognition is by base name, so
// use isReservedPath to test a bundle-relative path.
var ReservedFiles = map[string]bool{"index.md": true, "log.md": true}

// IsReservedPath reports whether a bundle-relative slash path names a reserved
// file (index.md or log.md) at any depth.
func IsReservedPath(rel string) bool {
	return ReservedFiles[path.Base(rel)]
}

// DefaultSkipDirs is the base-name skip list applied to the bundle walk. These
// are vendored (third-party, checked-in or installed) and derived (build/tool
// output) directories whose .md files nobody authored as knowledge: a Python
// virtualenv or a build-output tree sitting under the bundle root would
// otherwise become part of the graph. The set is matched by directory BASE NAME
// at any depth, so tool/.venv and web/node_modules are both skipped. It is a
// default, not a policy: --no-ignore (WithNoIgnore) restores the full walk, and
// the skip is never silent (Bundle.SkippedDirs records what was pruned so the
// caller can announce it). The bundle root itself is never skipped even if its
// own base name matches.
//
// This deliberately does NOT consult .gitignore (couples curation scope to
// version-control scope, two different questions) and is a built-in default
// rather than a required .okfctlignore (the tool must be usable on a real tree
// with no config); a project-level ignore file composes cleanly on top later.
var DefaultSkipDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	".okfctl":       true, // the tool's own index/state dir
	"node_modules":  true,
	".venv":         true,
	"venv":          true,
	"env":           true,
	"__pycache__":   true,
	".mypy_cache":   true,
	".pytest_cache": true,
	".tox":          true,
	".ruff_cache":   true,
	"site-packages": true,
	"vendor":        true,
	"target":        true, // Rust/JVM build output
	"dist":          true,
	"build":         true,
	".next":         true,
	".cache":        true,
	".idea":         true,
	".vscode":       true,
}

// Bundle is a loaded OKF bundle: concept nodes keyed by bundle-relative path,
// plus the reserved files, plus the derived link graph.
type Bundle struct {
	Root       string
	Nodes      map[string]*Node // concept nodes only (excludes reserved)
	Reserved   map[string]*Node // index.md, log.md
	OkfVersion string           // okf_version from the bundle's .okf, or SpecVersion if absent
	// SkippedDirs holds the bundle-relative slash paths of directories pruned
	// from the walk by the default skip list (see DefaultSkipDirs), sorted.
	// Empty when WithNoIgnore was passed or nothing matched. The CLI announces
	// these on stderr so an excluded subtree is never a silent omission.
	SkippedDirs []string
	// EscapedSymlinks holds the bundle-relative slash paths of symbolic links
	// that resolve outside the bundle root and were therefore not read (§3.2
	// rule 5, §11.2), sorted. It covers .md files, directories, and the .okf
	// sidecar. Empty when no link escapes. The skip is never silent: validate
	// warns per path and the CLI announces the list on stderr.
	EscapedSymlinks []string
	edges           map[string][]string
}

// loadConfig holds the resolved options for a Load call.
type loadConfig struct {
	noIgnore bool
}

// LoadOption configures Load. The zero set of options is the default behavior:
// the bundle walk skips vendored/derived directories (DefaultSkipDirs).
type LoadOption func(*loadConfig)

// WithNoIgnore restores the full walk: no directory is skipped, so the loaded
// graph is byte-identical to the pre-skip-list behavior. It is the escape hatch
// for a bundle that deliberately authored real content into a directory whose
// name happens to match the skip list.
func WithNoIgnore() LoadOption {
	return func(c *loadConfig) { c.noIgnore = true }
}

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Load walks root, parses every .md file, and builds the in-memory graph.
//
// By default the walk prunes vendored and derived directories (DefaultSkipDirs)
// so content nobody authored as knowledge never enters the graph; pass
// WithNoIgnore to restore the full walk. Applying the skip once here means every
// consumer (lint, analyze, validate, search, graph, index) inherits identical
// scope — divergent per-command scope would be its own bug class.
func Load(root string, opts ...LoadOption) (*Bundle, error) {
	cfg := loadConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	b := &Bundle{
		Root:     root,
		Nodes:    map[string]*Node{},
		Reserved: map[string]*Node{},
		edges:    map[string][]string{},
	}
	// §3.2 rule 5 / §11.2: a consumer reads only files under the bundle root
	// and does not follow a symbolic link out of it. The boundary is the root's
	// REAL path, so a root that is itself a symlink still contains its files.
	realRoot, err := resolvedRoot(root)
	if err != nil {
		return nil, err
	}
	skipped := map[string]bool{}
	escaped := map[string]bool{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// WalkDir never descends a symlinked directory, so a symlink entry is
		// the only way a walked path can leave the root. Resolve it and skip
		// it when it lands outside. Only entries Load would otherwise act on
		// are recorded: .md files and directories (assets are never read).
		if p != root && d.Type()&fs.ModeSymlink != 0 {
			if out, isDir := symlinkEscapes(realRoot, p); out && (isDir || strings.HasSuffix(d.Name(), ".md")) {
				if rel, relErr := filepath.Rel(root, p); relErr == nil {
					escaped[filepath.ToSlash(rel)] = true
				}
				return nil
			}
		}
		if d.IsDir() {
			// Prune vendored/derived subtrees before descending. The bundle
			// root is never a skip candidate (p == root), even if its own base
			// name matches — we skip subtrees, not the bundle itself.
			if !cfg.noIgnore && p != root && DefaultSkipDirs[d.Name()] {
				rel, relErr := filepath.Rel(root, p)
				if relErr == nil {
					skipped[filepath.ToSlash(rel)] = true
				}
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// p is a bundle file discovered by walking the user's bundle root;
		// reading the user's own bundle is the loader's purpose.
		src, err := os.ReadFile(p) //nolint:gosec // G304: reading a file from the user's own bundle root
		if err != nil {
			return err
		}
		fm, body, present, ferr := ParseFrontmatterDetailed(src)
		if ferr != nil {
			// Preserve the node with nil frontmatter so validate can report it.
			// A malformed block was physically present, so record that.
			n := &Node{Path: rel, Frontmatter: nil, Body: string(src), HasFrontmatterBlock: true}
			b.place(rel, n)
			return nil
		}
		n := &Node{Path: rel, Frontmatter: fm, Body: body, HasFrontmatterBlock: present}
		b.place(rel, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(skipped) > 0 {
		b.SkippedDirs = make([]string, 0, len(skipped))
		for s := range skipped {
			b.SkippedDirs = append(b.SkippedDirs, s)
		}
		sort.Strings(b.SkippedDirs)
	}
	if out, _ := symlinkEscapes(realRoot, filepath.Join(root, ".okf")); out {
		escaped[".okf"] = true
	}
	if len(escaped) > 0 {
		b.EscapedSymlinks = make([]string, 0, len(escaped))
		for s := range escaped {
			b.EscapedSymlinks = append(b.EscapedSymlinks, s)
		}
		sort.Strings(b.EscapedSymlinks)
	}
	b.OkfVersion = readOkfVersion(root)
	b.buildEdges()
	return b, nil
}

// readOkfVersion returns the okf_version declared in the bundle's .okf file, or
// SpecVersion if the file is absent or carries no (non-empty) okf_version key.
// The .okf is a small YAML document (e.g. "okf_version: 0.1"); a missing or
// unreadable file is not an error here — Load stays lenient and falls back to
// the build's version. It reads through the generalized sidecar parser
// (readOkfSidecar) so a bundle carrying other sidecar keys — e.g. a §9.2
// `templates` reference — does not change the version read.
func readOkfVersion(root string) string {
	if v := strings.TrimSpace(readOkfSidecar(root)["okf_version"]); v != "" {
		return v
	}
	return SpecVersion
}

// EscapedSymlinkMessage is the warning text for a Bundle.EscapedSymlinks entry.
const EscapedSymlinkMessage = "symlink resolves outside the bundle root; not read (OKF §3.2 rule 5, §11.2)"

// resolvedRoot returns the absolute real path of the bundle root, the boundary
// for §3.2 rule 5.
func resolvedRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// symlinkEscapes reports whether p is a symbolic link whose real path lies
// outside realRoot (§3.2 rule 5, §11.2), and whether its target is a directory.
// A path that is not a symlink, or a dangling link that cannot be resolved,
// reports false: nothing outside the root is reachable through it, and the
// existing read error surfaces as before.
func symlinkEscapes(realRoot, p string) (escapes, isDir bool) {
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		return false, false
	}
	// Resolve from an absolute path: EvalSymlinks keeps a relative input
	// relative, which would never compare as inside the absolute realRoot.
	abs, err := filepath.Abs(p)
	if err != nil {
		return false, false
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return false, false
	}
	if st, err := os.Stat(real); err == nil {
		isDir = st.IsDir()
	}
	return !withinRoot(realRoot, real), isDir
}

// withinRoot reports whether the real path real is realRoot or lies under it.
func withinRoot(realRoot, real string) bool {
	rel, err := filepath.Rel(realRoot, real)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// escapesBundleRoot reports whether the bundle file p (a path under root) is a
// symlink that resolves outside root. Callers that read a single bundle file
// directly, outside Load's walk, use it to honor §11.2 the same way.
func escapesBundleRoot(root, p string) bool {
	realRoot, err := resolvedRoot(root)
	if err != nil {
		return false
	}
	out, _ := symlinkEscapes(realRoot, p)
	return out
}

func (b *Bundle) place(rel string, n *Node) {
	if IsReservedPath(rel) {
		b.Reserved[rel] = n
		return
	}
	b.Nodes[rel] = n
}

// buildEdges derives the in-bundle link graph from each concept node's body.
//
// For every CommonMark inline link [text](target) it resolves target to a node
// two ways, in order: first root-relative via filepath.Clean(target), then
// dir-relative via filepath.Clean(join(dir, target)) where dir is the linking
// node's directory. Only in-bundle .md targets that resolve to a known node
// become edges. External links (http/https), in-page anchors (#...), and any
// target that does not resolve to a node are ignored. Image syntax
// (![alt](src), a '!' immediately preceding '[') is not a link and is skipped.
// An optional CommonMark title (e.g. `path.md "Title"`) is stripped before
// resolution, keeping only the leading whitespace-delimited URL.
func (b *Bundle) buildEdges() {
	for path, n := range b.Nodes {
		dir := filepath.Dir(path)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			b.edges[path] = append(b.edges[path], l.resolved)
		}
	}
}

// OutboundLinks returns the in-bundle nodes that path links to.
func (b *Bundle) OutboundLinks(path string) []string { return b.edges[path] }
