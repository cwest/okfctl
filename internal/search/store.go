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

package search

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"

	"github.com/cwest/okfctl/internal/okf"
)

// ErrModelMismatch is returned when a query embedder's model does not match the
// model a store was built under. Vectors are never comparable across models, so
// the guard refuses rather than returning meaningless similarities.
var ErrModelMismatch = errors.New("index model does not match the active embedder; rebuild with 'okfctl-search index build'")

// Entry is one concept node's embedding record: its bundle-relative path, the
// content hash keying re-embed decisions, and the vector itself.
type Entry struct {
	Path   string    `json:"path"`
	Hash   string    `json:"hash"`
	Vector []float64 `json:"vector"`
}

// Store is the flat vector index persisted at .okfctl/index.db. It records the
// embedder Model + Dim (the §8.5 reproducibility discipline) so a query can
// refuse a model-mismatched store, plus one Entry per concept node.
type Store struct {
	Model   string  `json:"model"`
	Dim     int     `json:"dim"`
	Entries []Entry `json:"entries"`
}

// contentHash keys re-embedding: a node whose title+body is unchanged since the
// last build keeps its prior vector.
func contentHash(n *okf.Node) string {
	title, _ := n.Frontmatter["title"].(string)
	sum := sha256.Sum256([]byte(title + "\x00" + n.Body))
	return hex.EncodeToString(sum[:])
}

// embedText is what a node contributes to the vector: its title plus body.
func embedText(n *okf.Node) string {
	title, _ := n.Frontmatter["title"].(string)
	return title + " " + n.Body
}

// BuildIndex embeds every concept node in b under embedder e, reusing an entry
// from prev when the node's content hash is unchanged (no re-embed). Entries are
// sorted by path so the serialization is deterministic for a fixed embedder.
func BuildIndex(b *okf.Bundle, e Embedder, prev *Store) *Store {
	prevByPath := map[string]Entry{}
	if prev != nil && prev.Model == e.Name() {
		for _, en := range prev.Entries {
			prevByPath[en.Path] = en
		}
	}

	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Batch the texts that actually need embedding.
	var toEmbed []string
	var toEmbedPaths []string
	entries := make([]Entry, 0, len(paths))
	for _, p := range paths {
		n := b.Nodes[p]
		h := contentHash(n)
		if old, ok := prevByPath[p]; ok && old.Hash == h {
			entries = append(entries, old) // unchanged: reuse vector
			continue
		}
		entries = append(entries, Entry{Path: p, Hash: h}) // vector filled below
		toEmbed = append(toEmbed, embedText(n))
		toEmbedPaths = append(toEmbedPaths, p)
	}
	if len(toEmbed) > 0 {
		vecs := e.Encode(toEmbed)
		vecByPath := map[string][]float64{}
		for i, p := range toEmbedPaths {
			vecByPath[p] = vecs[i]
		}
		for i := range entries {
			if v, ok := vecByPath[entries[i].Path]; ok {
				entries[i].Vector = v
			}
		}
	}
	return &Store{Model: e.Name(), Dim: e.Dim(), Entries: entries}
}

// Save writes the store as indented JSON (deterministic; sorted entries).
func (s *Store) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Load reads a store from path.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
