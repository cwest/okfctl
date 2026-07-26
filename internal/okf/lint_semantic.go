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
	"sort"
)

// Neighbor is one ranked semantic neighbor of a node: the neighbor's path and
// its cosine similarity to the subject node.
type Neighbor struct {
	Path  string
	Score float64
}

// SemanticIndex maps a node path to its ranked neighbors (self excluded). It is
// deliberately a plain data shape rather than a search-package type: the checks
// below are pure functions over similarity scores, so internal/okf stays free of
// any dependency on the index format or the embedder that produced it. The
// caller (cmd) reads the real index and adapts it into this shape.
type SemanticIndex map[string][]Neighbor

// SemanticOptions tunes the similarity-driven checks.
type SemanticOptions struct {
	// SimilarityThreshold is the cosine score at or above which two UNLINKED
	// nodes are reported as a possible missing link. Zero means the default.
	SimilarityThreshold float64
	// IsolationFloor is the score a node's BEST neighbor must reach for the node
	// to count as semantically connected. Zero means the default.
	IsolationFloor float64
}

const (
	defaultSimilarityThreshold = 0.80
	// defaultIsolationFloor is deliberately low. Calibrated against
	// potion-base-8M over a small wine corpus, same-topic-different-wording
	// nodes score ~0.27-0.33 while a genuinely off-topic node (a Kubernetes
	// concept among wine notes) scores ~0.13. A 0.30 floor therefore flags
	// legitimate on-topic nodes as "dead concepts" — a false positive that
	// trains users to ignore the check. 0.20 separates the true outlier from
	// merely-loosely-related kin. Mean-pooled static embeddings produce
	// compressed absolute scores; the RANKING is reliable, the magnitudes are
	// not, so the floor targets the clear outlier rather than a semantic ideal.
	defaultIsolationFloor = 0.20
)

// LintSemantic runs the similarity-driven curation checks the PRD (§8.6) calls
// for: pairs that read alike but carry no edge, and nodes with no semantically
// close kin at all. It is the semantic counterpart to the structural checks in
// Lint — structural asks "is anything linked to this?", semantic asks "is
// anything even about the same thing?".
//
// idx supplies the neighbor sets; a node absent from idx is reported once as
// index drift rather than silently skipped, so a partial answer never reads as a
// complete one. Findings are sorted by path then check, so the same inputs
// always produce byte-identical output.
func LintSemantic(b *Bundle, idx SemanticIndex, opts SemanticOptions) []LintFinding {
	threshold := opts.SimilarityThreshold
	if threshold <= 0 {
		threshold = defaultSimilarityThreshold
	}
	floor := opts.IsolationFloor
	if floor <= 0 {
		floor = defaultIsolationFloor
	}

	var findings []LintFinding
	findings = append(findings, lintStaleIndex(b, idx)...)
	findings = append(findings, lintSimilarUnlinked(b, idx, threshold)...)
	findings = append(findings, lintNoSemanticNeighbors(b, idx, floor)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Check < findings[j].Check
	})
	return findings
}

// edgeExists reports whether either node links to the other. A missing-link
// finding is about the PAIR being disconnected, so a link in either direction
// clears it.
func edgeExists(b *Bundle, a, z string) bool {
	if n, ok := b.Nodes[a]; ok && linkedTargets(b, a, n)[z] {
		return true
	}
	if n, ok := b.Nodes[z]; ok && linkedTargets(b, z, n)[a] {
		return true
	}
	return false
}

// lintSimilarUnlinked reports node pairs scoring at or above threshold with no
// edge between them — the PRD's "0.91 similar, no edge, missing link?" finding.
// Each pair is reported ONCE, on its lexicographically-first path, because the
// finding is a property of the pair and reporting it twice would double-count
// the same curation decision.
func lintSimilarUnlinked(b *Bundle, idx SemanticIndex, threshold float64) []LintFinding {
	type pair struct{ a, z string }
	best := map[pair]float64{}

	for path, neighbors := range idx {
		if _, ok := b.Nodes[path]; !ok {
			continue // index entry for a node no longer in the bundle
		}
		for _, nb := range neighbors {
			if nb.Score < threshold || nb.Path == path {
				continue
			}
			if _, ok := b.Nodes[nb.Path]; !ok {
				continue
			}
			a, z := path, nb.Path
			if z < a {
				a, z = z, a
			}
			// Scores are symmetric in principle; keep the max defensively so an
			// asymmetric index cannot make output depend on map order.
			if s, seen := best[pair{a, z}]; !seen || nb.Score > s {
				best[pair{a, z}] = nb.Score
			}
		}
	}

	var out []LintFinding
	for p, score := range best {
		if edgeExists(b, p.a, p.z) {
			continue
		}
		out = append(out, LintFinding{
			Check: "similar-unlinked",
			Path:  p.a,
			Message: fmt.Sprintf("%.2f semantically similar to %s with no link between them — missing cross-reference?",
				score, p.z),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })
	return out
}

// lintNoSemanticNeighbors reports nodes whose closest neighbor falls below the
// floor — the PRD's "orphan with no semantic neighbors, dead concept?" finding.
// This differs from the structural orphan check: a node can be well-linked yet
// semantically isolated, or unlinked yet clearly on-topic.
func lintNoSemanticNeighbors(b *Bundle, idx SemanticIndex, floor float64) []LintFinding {
	var out []LintFinding
	for path := range b.Nodes {
		neighbors, ok := idx[path]
		if !ok {
			continue // drift, already reported by lintStaleIndex
		}
		if len(neighbors) == 0 {
			// A single-node bundle has nothing to be similar to; that is not a
			// curation finding.
			continue
		}
		bestScore := neighbors[0].Score
		for _, nb := range neighbors {
			if nb.Score > bestScore {
				bestScore = nb.Score
			}
		}
		if bestScore < floor {
			out = append(out, LintFinding{
				Check: "no-semantic-neighbors",
				Path:  path,
				Message: fmt.Sprintf("no semantically close node (best neighbor %.2f, below %.2f) — dead concept, or missing context?",
					bestScore, floor),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// lintStaleIndex reports bundle nodes missing from the index in ONE bundle-level
// finding. Without it, a node added since the last `index build` would simply not
// be checked, and a partial semantic pass would look identical to a clean one.
func lintStaleIndex(b *Bundle, idx SemanticIndex) []LintFinding {
	var missing []string
	for path := range b.Nodes {
		if _, ok := idx[path]; !ok {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []LintFinding{{
		Check: "stale-index",
		Path:  "",
		Message: fmt.Sprintf("%d node(s) absent from the semantic index and not checked (%s) — rerun 'okfctl-search index build'",
			len(missing), joinPaths(missing)),
	}}
}

// joinPaths renders a bounded, deterministic path list so one very stale index
// cannot produce an unreadable multi-kilobyte finding.
func joinPaths(paths []string) string {
	const max = 5
	if len(paths) <= max {
		return joinComma(paths)
	}
	return fmt.Sprintf("%s, and %d more", joinComma(paths[:max]), len(paths)-max)
}

func joinComma(paths []string) string {
	out := ""
	for i, p := range paths {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
