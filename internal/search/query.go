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
	"fmt"
	"sort"
)

// Result is one ranked search hit. Snippet carries the best-matching passage
// text when the store has a passage layer; it is empty for a passage-less
// (legacy) index answered off whole-node vectors.
type Result struct {
	Score   float64
	Path    string
	Snippet string
}

// Query embeds q with e (which MUST match the store's model), computes cosine
// similarity, and returns the top-k results sorted by score descending. Ties
// break by path for determinism. When the store has a passage layer it ranks
// passages and dedupes to the best-scoring passage per node, returning that
// passage's text as the Snippet; when it does not (a legacy index), it falls
// back to whole-node Entries with empty snippets.
func Query(s *Store, e Embedder, q string, k int) ([]Result, error) {
	if s.Model != e.Name() {
		return nil, ErrModelMismatch
	}
	qv := e.Encode([]string{q})[0]
	if len(s.Passages) > 0 {
		return rankPassages(s.Passages, qv, k), nil
	}
	return rank(s.Entries, qv, k, ""), nil
}

// Related returns the nearest neighbors of the node at nodePath using its stored
// vector, excluding the node itself. No embedder is needed — it reuses the index.
func Related(s *Store, nodePath string, k int) ([]Result, error) {
	var self *Entry
	for i := range s.Entries {
		if s.Entries[i].Path == nodePath {
			self = &s.Entries[i]
			break
		}
	}
	if self == nil {
		return nil, fmt.Errorf("node %q not found in index", nodePath)
	}
	return rank(s.Entries, self.Vector, k, nodePath), nil
}

// rank scores every entry against vec (skipping exclude), sorts by score desc
// then path, and returns the top k.
func rank(entries []Entry, vec []float64, k int, exclude string) []Result {
	results := make([]Result, 0, len(entries))
	for _, en := range entries {
		if en.Path == exclude {
			continue
		}
		results = append(results, Result{Score: cosine(vec, en.Vector), Path: en.Path})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Path < results[j].Path
	})
	if k > 0 && len(results) > k {
		results = results[:k]
	}
	return results
}

// rankPassages scores every passage against vec, dedupes to the best-scoring
// passage per node (so a long node cannot flood the results with its many
// sections), then sorts by score desc then path and returns the top k. The
// surviving passage's text becomes the result Snippet. Ties within a node break
// by heading path so dedup is deterministic.
func rankPassages(passages []PassageEntry, vec []float64, k int) []Result {
	type best struct {
		score   float64
		heading string
		snippet string
	}
	byNode := map[string]best{}
	for _, p := range passages {
		sc := cosine(vec, p.Vector)
		cur, ok := byNode[p.NodePath]
		if !ok || sc > cur.score || (sc == cur.score && p.HeadingPath < cur.heading) {
			byNode[p.NodePath] = best{score: sc, heading: p.HeadingPath, snippet: p.Text}
		}
	}
	results := make([]Result, 0, len(byNode))
	for node, b := range byNode {
		results = append(results, Result{Score: b.score, Path: node, Snippet: b.snippet})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Path < results[j].Path
	})
	if k > 0 && len(results) > k {
		results = results[:k]
	}
	return results
}
