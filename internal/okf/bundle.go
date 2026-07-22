// Copyright 2026 Casey West
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
	Root     string
	Nodes    map[string]*Node // concept nodes only (excludes reserved)
	Reserved map[string]*Node // index.md, log.md
	edges    map[string][]string
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
	b.buildEdges()
	return b, nil
}

func (b *Bundle) place(rel string, n *Node) {
	if ReservedFiles[rel] {
		b.Reserved[rel] = n
		return
	}
	b.Nodes[rel] = n
}

func (b *Bundle) buildEdges() {
	for path, n := range b.Nodes {
		dir := filepath.Dir(path)
		for _, m := range mdLinkRe.FindAllStringSubmatch(n.Body, -1) {
			target := m[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
				continue
			}
			norm := filepath.ToSlash(filepath.Clean(target))
			if _, ok := b.Nodes[norm]; ok {
				b.edges[path] = append(b.edges[path], norm)
				continue
			}
			rel := filepath.ToSlash(filepath.Clean(filepath.Join(dir, target)))
			if _, ok := b.Nodes[rel]; ok {
				b.edges[path] = append(b.edges[path], rel)
			}
		}
	}
}

// OutboundLinks returns the in-bundle nodes that path links to.
func (b *Bundle) OutboundLinks(path string) []string { return b.edges[path] }
