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

// PassageEntry is one heading-delimited passage's embedding record. NodePath ties
// it back to its concept node (Entry.Path), HeadingPath is the section heading it
// came from ("" for the node's preamble text), Hash keys per-passage re-embed
// decisions, Text is the passage snippet returned to the caller, and Vector is
// its embedding. Passages are an ADDITIVE layer alongside Entries: whole-node
// vectors (Entries) still drive Related and the model-mismatch guard unchanged,
// while Passages let Query rank on the sub-node section that actually answers a
// query on long, multi-section nodes.
type PassageEntry struct {
	NodePath    string    `json:"node_path"`
	HeadingPath string    `json:"heading_path"`
	Hash        string    `json:"hash"`
	Text        string    `json:"text"`
	Vector      []float64 `json:"vector"`
}

// Store is the flat vector index persisted at .okfctl/index.db. It records the
// embedder Model + Dim (the §8.5 reproducibility discipline) so a query can
// refuse a model-mismatched store, one Entry per concept node, and one
// PassageEntry per heading-delimited passage. Passages is additive and may be
// empty (a legacy index): Query falls back to Entries when it is.
type Store struct {
	Model    string         `json:"model"`
	Dim      int            `json:"dim"`
	Entries  []Entry        `json:"entries"`
	Passages []PassageEntry `json:"passages,omitempty"`
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

// passageContentHash keys per-passage re-embedding: a passage keyed by node title
// and its own text is not re-embedded when unchanged across a rebuild.
func passageContentHash(title, passageText string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + passageText))
	return hex.EncodeToString(sum[:])
}

// passageEmbedText is what a passage contributes to its vector: the node title
// (for topical context — a bare section heading is often too terse to embed well)
// plus the passage text. This mirrors embedText prepending the title.
func passageEmbedText(title, passageText string) string {
	return title + " " + passageText
}

// BuildIndex embeds every concept node in b under embedder e, reusing an entry
// from prev when the node's content hash is unchanged (no re-embed). It also
// builds an additive passage layer: each node body is split on markdown headings
// (chunkByHeadings) and every passage is embedded, again reusing an unchanged
// passage's prior vector. Entries and passages are sorted deterministically so
// the serialization is stable for a fixed embedder.
func BuildIndex(b *okf.Bundle, e Embedder, prev *Store) *Store {
	prevByPath := map[string]Entry{}
	prevPassageByKey := map[string]PassageEntry{}
	if prev != nil && prev.Model == e.Name() {
		for _, en := range prev.Entries {
			prevByPath[en.Path] = en
		}
		for _, p := range prev.Passages {
			prevPassageByKey[passageKey(p.NodePath, p.Hash)] = p
		}
	}

	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Batch every text that actually needs embedding — whole-node and passage —
	// into a single Encode call so the plugin makes one pass over the embedder.
	var toEmbed []string
	entries := make([]Entry, 0, len(paths))
	passages := make([]PassageEntry, 0, len(paths))
	// pending records where each freshly-embedded vector lands: into an entry
	// (passageIdx < 0) or a passage slot.
	type slot struct {
		entryIdx   int
		passageIdx int
	}
	var pending []slot

	for _, p := range paths {
		n := b.Nodes[p]
		title, _ := n.Frontmatter["title"].(string)

		// Whole-node entry (unchanged: keep Related and the model guard intact).
		h := contentHash(n)
		entryIdx := len(entries)
		if old, ok := prevByPath[p]; ok && old.Hash == h {
			entries = append(entries, old) // unchanged: reuse vector
		} else {
			entries = append(entries, Entry{Path: p, Hash: h}) // vector filled below
			toEmbed = append(toEmbed, embedText(n))
			pending = append(pending, slot{entryIdx: entryIdx, passageIdx: -1})
		}

		// Additive passage layer.
		for _, ch := range chunkByHeadings(n.Body) {
			ph := passageContentHash(title, ch.Text)
			passageIdx := len(passages)
			if old, ok := prevPassageByKey[passageKey(p, ph)]; ok {
				passages = append(passages, old) // unchanged: reuse vector
				continue
			}
			passages = append(passages, PassageEntry{
				NodePath:    p,
				HeadingPath: ch.Heading,
				Hash:        ph,
				Text:        ch.Text,
			})
			toEmbed = append(toEmbed, passageEmbedText(title, ch.Text))
			pending = append(pending, slot{entryIdx: -1, passageIdx: passageIdx})
		}
	}

	if len(toEmbed) > 0 {
		vecs := e.Encode(toEmbed)
		for i, s := range pending {
			if s.passageIdx >= 0 {
				passages[s.passageIdx].Vector = vecs[i]
			} else {
				entries[s.entryIdx].Vector = vecs[i]
			}
		}
	}
	return &Store{Model: e.Name(), Dim: e.Dim(), Entries: entries, Passages: passages}
}

// passageKey identifies a passage for prev-store reuse: node path plus content
// hash. The hash already folds in the title and passage text, so a passage whose
// heading was renamed but text is otherwise identical still reuses its vector.
func passageKey(nodePath, hash string) string {
	return nodePath + "\x00" + hash
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
