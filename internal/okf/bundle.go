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
	"path/filepath"
	"regexp"
	"strings"
)

// ReservedFiles are the bundle-level files that are not concept nodes.
var ReservedFiles = map[string]bool{"index.md": true, "log.md": true}

// Bundle is a loaded OKF bundle: concept nodes keyed by bundle-relative path,
// plus the reserved files, plus the derived link graph.
type Bundle struct {
	Root       string
	Nodes      map[string]*Node // concept nodes only (excludes reserved)
	Reserved   map[string]*Node // index.md, log.md
	OkfVersion string           // okf_version from the bundle's .okf, or SpecVersion if absent
	edges      map[string][]string
}

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Load walks root, parses every .md file, and builds the in-memory graph.
func Load(root string) (*Bundle, error) {
	b := &Bundle{
		Root:     root,
		Nodes:    map[string]*Node{},
		Reserved: map[string]*Node{},
		edges:    map[string][]string{},
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, body, ferr := ParseFrontmatter(src)
		if ferr != nil {
			// Preserve the node with nil frontmatter so validate can report it.
			n := &Node{Path: rel, Frontmatter: nil, Body: string(src)}
			b.place(rel, n)
			return nil
		}
		n := &Node{Path: rel, Frontmatter: fm, Body: body}
		b.place(rel, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	b.OkfVersion = readOkfVersion(root)
	b.buildEdges()
	return b, nil
}

// readOkfVersion returns the okf_version declared in the bundle's .okf file, or
// SpecVersion if the file is absent or carries no okf_version key. The .okf is a
// small YAML document (e.g. "okf_version: 0.1"); a missing or unreadable file is
// not an error here — Load stays lenient and falls back to the build's version.
func readOkfVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".okf"))
	if err != nil {
		return SpecVersion
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "okf_version" {
			continue
		}
		if v := strings.TrimSpace(val); v != "" {
			return v
		}
	}
	return SpecVersion
}

func (b *Bundle) place(rel string, n *Node) {
	if ReservedFiles[rel] {
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
