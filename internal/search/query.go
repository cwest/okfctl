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
	"math"
	"sort"
	"strings"
	"time"
)

// Result is one ranked search hit. Snippet carries the best-matching passage
// text when the store has a passage layer; it is empty for a passage-less
// (legacy) index answered off whole-node vectors.
type Result struct {
	Score   float64
	Path    string
	Snippet string
}

// NodeMeta is the per-node metadata the scoping filters and recency decay key on,
// resolved from the live *okf.Bundle at query time rather than denormalized onto
// the index. Type and Tags back the §4.1 filters; Generated backs §5.2/§13.1
// recency decay. The index (Entry/PassageEntry) intentionally does NOT carry
// these — contentHash hashes only title+body, so a frontmatter-only edit (a type
// change, a new tag, a refreshed generated.at) does not re-embed, and a cached
// copy denormalized onto the index would go stale silently. Resolving against the
// bundle avoids that trap.
type NodeMeta struct {
	Type         string
	Tags         []string
	Generated    time.Time // §5.2 generated.at (or §13.1 legacy timestamp)
	HasGenerated bool      // false when the node carries no usable date
}

// Filter narrows a semantic query to a subset of nodes BEFORE ranking. Empty
// fields impose no constraint. The filter has two halves applied in order:
//
//  1. Positive inclusion. Within a dimension, repeated values compose with OR
//     (a node matches if it satisfies ANY value); the three dimensions compose
//     with AND (a node must satisfy every populated dimension). An empty positive
//     set for a dimension imposes no constraint on that dimension, so an entirely
//     empty positive set means "all nodes."
//  2. Negative exclusion, applied AFTER the positive set. A node is dropped if it
//     matches ANY value in ANY negative dimension. Exclusion beats inclusion:
//     --path research/ --not-path research/agents/ keeps research/ nodes except
//     those under research/agents/.
//
// PathPrefixes keep nodes whose bundle-relative path starts with any prefix;
// Types keep nodes whose §4.1 type matches any value exactly; Tags keep nodes
// carrying any of the §4.1 tags. The Not* sets mirror them for exclusion.
// A filter that matches zero nodes yields an empty result, never an error and
// never a silent fall-back to the unfiltered set (consistent with lexical
// Search's null-query behavior).
type Filter struct {
	PathPrefixes []string
	Types        []string
	Tags         []string

	NotPathPrefixes []string
	NotTypes        []string
	NotTags         []string
}

// IsEmpty reports whether the filter imposes no constraint at all. It MUST
// account for the negative sets: a negative-only filter (e.g. --not-path
// research/) is a real constraint, and if IsEmpty read it as empty the CLI's
// needBundle short-circuit would skip the metadata walk and silently no-op every
// exclusion.
func (f Filter) IsEmpty() bool {
	return len(f.PathPrefixes) == 0 && len(f.Types) == 0 && len(f.Tags) == 0 &&
		len(f.NotPathPrefixes) == 0 && len(f.NotTypes) == 0 && len(f.NotTags) == 0
}

// keep reports whether the node at path (with metadata m) passes the filter:
// every populated POSITIVE dimension must OR-match, then the node must NOT match
// any NEGATIVE dimension. A node with no metadata entry cannot satisfy a positive
// Type/Tag constraint (we cannot assert a match we cannot see); for exclusion,
// absence of metadata simply means the node matches no Type/Tag value and so is
// not excluded on those dimensions. PathPrefix checks the path itself and needs
// no metadata in either direction.
func (f Filter) keep(path string, m NodeMeta, hasMeta bool) bool {
	// --- Positive inclusion: each populated dimension must OR-match. ---
	if len(f.PathPrefixes) > 0 && !hasAnyPrefix(path, f.PathPrefixes) {
		return false
	}
	if len(f.Types) > 0 {
		if !hasMeta || !containsString(f.Types, m.Type) {
			return false
		}
	}
	if len(f.Tags) > 0 {
		if !hasMeta || !anyTag(m.Tags, f.Tags) {
			return false
		}
	}
	// --- Negative exclusion, applied after the positive set. ---
	if hasAnyPrefix(path, f.NotPathPrefixes) {
		return false
	}
	if hasMeta && containsString(f.NotTypes, m.Type) {
		return false
	}
	if hasMeta && anyTag(m.Tags, f.NotTags) {
		return false
	}
	return true
}

// hasAnyPrefix reports whether path starts with any of the prefixes. An empty
// prefix list matches nothing (used by exclusion, where "no negatives" excludes
// no node).
func hasAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// containsString reports whether want is in xs.
func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// anyTag reports whether the node carries any of the wanted tags.
func anyTag(nodeTags, want []string) bool {
	for _, w := range want {
		if containsTag(nodeTags, w) {
			return true
		}
	}
	return false
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// DefaultDecayFloor is the scale-free lower clamp on the recency multiplier that
// BOTH query surfaces — the okfctl-search CLI (--decay-floor) and the HTTP
// /api/v1/search endpoint (decay_floor) — apply by default. It lives here, in the
// package both surfaces import, so the default is a single source of truth: the
// #65/#67 fix landed it on the CLI, and a later merge left the HTTP surface at an
// unbounded (0) floor, so the two surfaces answered the same query differently.
// Reading one named constant from both call sites is what stops that drift from
// recurring on the next merge-order accident. 0.25 is scale-free (it assumes
// nothing about the cosine distribution of whichever embedder is loaded); it must
// stay in [0, 1] for [DecayFloor, 1] to be a real interval — see DecayOptions.factor.
const DefaultDecayFloor = 0.25

// DecayOptions configures post-ranking recency decay. It is OFF by default (a nil
// *DecayOptions on QueryOptions). When set, a result's raw cosine is multiplied by
// an exponential recency factor derived from the node's §5.2 generated.at (with
// the §13.1 legacy timestamp fallback) and Now, and results re-sort on that
// product. The correctness invariant the card fixes: the relevance floor is
// applied to RAW cosine FIRST (see MinRelevance) so decay only reorders survivors
// — recency can never promote an irrelevant-but-fresh node above a relevant one.
type DecayOptions struct {
	// HalfLifeDays is the age in days at which a node's score is halved. Must be
	// > 0 for decay to apply; <= 0 disables the multiplier (no penalty).
	HalfLifeDays float64
	// Now is the reference instant ages are measured from (injected for tests).
	Now time.Time
	// MinRelevance is the raw-cosine floor. A result whose UNDECAYED cosine is
	// below this is dropped before decay reorders the rest. Zero admits everything.
	MinRelevance float64
	// DecayFloor is the scale-free lower clamp on the recency multiplier itself
	// (distinct from MinRelevance, which is a floor on RAW cosine). Without it,
	// 0.5^(age/halfLife) tends to zero as age grows, so an old-but-perfect match
	// can be multiplied into irrelevance below a mediocre fresh one (#65: 0.0000
	// at half-life 90). Clamping the multiplier at DecayFloor bounds how far
	// recency can demote a still-relevant node, and it assumes nothing about the
	// cosine distribution of whichever embedder is loaded. Zero disables the clamp
	// (exact unbounded 0.5^x, byte-for-byte today's behavior).
	DecayFloor float64
}

// factor returns the multiplicative recency factor in [DecayFloor, 1] for a node
// generated at gen. A node with no usable date (hasGen false) gets factor 1 (no
// penalty): absence of a date is not a reason to demote a node. Half-life decay:
// factor = 0.5 ^ (ageDays / halfLife), clamped below by DecayFloor so recency can
// bound but never erase a still-relevant node (#65). §5.2: generated.at marks the
// content's last meaningful change, the signal that tells a recent edit from a
// stale fact.
//
// DecayFloor is REQUIRED to be in [0, 1] for [DecayFloor, 1] to be a real
// interval: a floor > 1 wins the math.Max for every node and turns the "lower
// clamp" into a flat GAIN on raw cosine (scores leave [-1, 1]); a floor < 0
// re-enables the #65 inversion. The CLI enforces this at parse time
// (cmd/okfctl-search: --decay-floor rejected outside [0, 1]), so this method is
// never reached with an out-of-range floor in normal use. It applies no
// defensive re-clamp of its own: a silent clamp here would change behavior for
// in-range values relative to today (#71 keeps the boundary at the CLI, not in
// the library).
func (d *DecayOptions) factor(gen time.Time, hasGen bool) float64 {
	if d.HalfLifeDays <= 0 || !hasGen {
		return 1
	}
	ageDays := d.Now.Sub(gen).Hours() / 24
	if ageDays <= 0 {
		return 1 // future or same-instant content gets no penalty
	}
	// DecayFloor 0 is a no-op (0.5^x >= 0), so this restores the exact unbounded
	// multiplier digit-for-digit; a positive floor bounds how far decay can demote.
	return math.Max(math.Pow(0.5, ageDays/d.HalfLifeDays), d.DecayFloor)
}

// QueryOptions bundles the additive scoping controls. The zero value (empty
// Filter, nil Meta, nil Decay, nil LexicalGate) makes QueryWith behave
// identically to Query.
type QueryOptions struct {
	// Meta maps node path -> metadata for filters and decay. Nil disables both
	// (a filter with no metadata to resolve against can constrain nothing).
	Meta map[string]NodeMeta
	// Filter narrows the candidate set before ranking.
	Filter Filter
	// Decay, when non-nil, applies post-ranking recency decay.
	Decay *DecayOptions
	// LexicalGate, when non-nil, gates the semantic result list against a
	// term-wise lexical match set and preserves the lexical tail (#66). It is
	// applied AFTER ranking and decay, before the top-k cut, and degrades to a
	// pure no-op for an all-stopword or over-broad query.
	LexicalGate *LexicalGateOptions
}

// Query embeds q with e (which MUST match the store's model), computes cosine
// similarity, and returns the top-k results sorted by score descending. Ties
// break by path for determinism. When the store has a passage layer it ranks
// passages and dedupes to the best-scoring passage per node, returning that
// passage's text as the Snippet; when it does not (a legacy index), it falls
// back to whole-node Entries with empty snippets.
func Query(s *Store, e Embedder, q string, k int) ([]Result, error) {
	return QueryWith(s, e, q, k, QueryOptions{})
}

// QueryWith is Query plus additive scoping (path/type/tag filters, §4.1) and
// optional post-ranking recency decay (§5.2/§13.1). With an empty Filter and nil
// Decay it is byte-for-byte equivalent to Query. Filters are applied PRE-ranking
// (against opts.Meta resolved from the live bundle); decay is applied POST-ranking
// with the relevance floor on RAW cosine.
func QueryWith(s *Store, e Embedder, q string, k int, opts QueryOptions) ([]Result, error) {
	if s.Model != e.Name() {
		return nil, ErrModelMismatch
	}
	qv := e.Encode([]string{q})[0]

	// Rank first with an effectively-unbounded k so filtering/decay never lose a
	// result to a premature top-k cut; apply the real k after the full pipeline.
	var ranked []Result
	if len(s.Passages) > 0 {
		ranked = rankPassages(s.Passages, qv, 0, opts.Filter, opts.Meta)
	} else {
		ranked = rank(s.Entries, qv, 0, "", opts.Filter, opts.Meta)
	}

	if opts.Decay != nil {
		ranked = applyDecay(ranked, opts.Decay, opts.Meta)
	}

	// Lexical gate (#66): applied after ranking+decay, before the top-k cut, so
	// the intersection is drawn from the fully-ranked band and the preserved
	// lexical tail carries real semantic scores. Nil or a degrading gate leaves
	// ranked untouched — the default path is byte-identical to plain Query.
	ranked = applyLexicalGate(ranked, opts.LexicalGate, k)

	if k > 0 && len(ranked) > k {
		ranked = ranked[:k]
	}
	return ranked, nil
}

// applyDecay enforces the relevance floor on RAW cosine, then re-scores the
// survivors by cosine×recency-factor and re-sorts. Sub-floor results are dropped
// entirely so a fresh-but-irrelevant node can never be promoted above a relevant
// older one. Ties break by path for determinism.
func applyDecay(in []Result, d *DecayOptions, meta map[string]NodeMeta) []Result {
	out := make([]Result, 0, len(in))
	for _, r := range in {
		if r.Score < d.MinRelevance {
			continue // §floor: cut on RAW cosine before decay reorders
		}
		m, ok := meta[r.Path]
		r.Score *= d.factor(m.Generated, ok && m.HasGenerated)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Path < out[j].Path
	})
	return out
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
	return rank(s.Entries, self.Vector, k, nodePath, Filter{}, nil), nil
}

// rank scores every entry against vec (skipping exclude and any entry the filter
// rejects), sorts by score desc then path, and returns the top k (k<=0 returns
// all). The filter is applied PRE-ranking so a filtered-out node never occupies a
// top-k slot.
func rank(entries []Entry, vec []float64, k int, exclude string, filter Filter, meta map[string]NodeMeta) []Result {
	results := make([]Result, 0, len(entries))
	for _, en := range entries {
		if en.Path == exclude {
			continue
		}
		if !filter.IsEmpty() {
			m, ok := meta[en.Path]
			if !filter.keep(en.Path, m, ok) {
				continue
			}
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
// by heading path so dedup is deterministic. The filter keys on the passage's
// NodePath metadata and is applied PRE-ranking — critical now that the ranked unit
// is a passage, which carries no §4.1 type/tag of its own: the constraint must
// resolve to the owning node or a filter would silently stop applying to passages.
func rankPassages(passages []PassageEntry, vec []float64, k int, filter Filter, meta map[string]NodeMeta) []Result {
	type best struct {
		score   float64
		heading string
		snippet string
	}
	byNode := map[string]best{}
	for _, p := range passages {
		if !filter.IsEmpty() {
			m, ok := meta[p.NodePath]
			if !filter.keep(p.NodePath, m, ok) {
				continue
			}
		}
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
