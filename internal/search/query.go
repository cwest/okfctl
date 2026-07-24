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

// Result is one ranked search hit.
type Result struct {
	Score float64
	Path  string
}

// Query embeds q with e (which MUST match the store's model), computes cosine
// similarity against every entry, and returns the top-k results sorted by score
// descending. Ties break by path for determinism.
func Query(s *Store, e Embedder, q string, k int) ([]Result, error) {
	if s.Model != e.Name() {
		return nil, ErrModelMismatch
	}
	qv := e.Encode([]string{q})[0]
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
