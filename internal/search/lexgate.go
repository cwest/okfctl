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
	"sort"
	"strings"
	"unicode"
)

// This file implements the term-wise lexical matcher behind --lexical-gate
// (cwest/okfctl#66). It exists because the two search surfaces never meet: the
// semantic query is embedding-only, and core `okfctl search --field body` is a
// PHRASE-WISE, raw-substring match. Two reproduced failures on the real corpus
// drive every choice here:
//
//   1. Phrase-wise body match on a question ("how should an agent decide when to
//      delegate work") returns 0 hits — so the gate must reduce a query to its
//      CONTENT TERMS (stopwords dropped) and match term-wise, never as a phrase.
//   2. Raw-substring match is asymmetric: `hash` matches 18 nodes, `hashes` 0;
//      `agent` 172, `agents` 100 — so the gate must NORMALIZE morphology (stem)
//      so a term and its inflection collapse to one match set.
//
// okfctl is a spec consumer, not an NLP toolkit, so the transforms are the
// lightest that fix the proven failures: a fixed English stopword set and a
// suffix stripper, not a full Porter stemmer (whose aggressive rewrites would
// over-collapse distinct identifiers).

// stopwords is a small closed set of English function words plus the
// question-shaped leaders proven necessary by the 0-hit reproduction. Content
// words (nouns, verbs like "delegate", domain terms like "agent") are NOT here —
// removing them would defeat the gate. Kept deliberately short: an over-broad
// stopword list silently drops real query signal.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "but": true, "by": true, "can": true, "did": true, "do": true,
	"does": true, "for": true, "from": true, "how": true, "i": true, "if": true,
	"in": true, "into": true, "is": true, "it": true, "its": true, "me": true,
	"my": true, "no": true, "not": true, "of": true, "on": true, "or": true,
	"our": true, "should": true, "so": true, "than": true, "that": true,
	"the": true, "their": true, "them": true, "then": true, "there": true,
	"these": true, "they": true, "this": true, "to": true, "up": true,
	"was": true, "we": true, "were": true, "what": true, "when": true,
	"where": true, "which": true, "who": true, "why": true, "will": true,
	"with": true, "would": true, "you": true, "your": true,
}

// minStemLen is the shortest a token may be REDUCED to by suffix stripping. A
// suffix is only removed if at least this many runes remain, so short tokens
// ("is", "os", "ci", "go") and short identifiers survive intact rather than
// being mangled to a single letter.
const minStemLen = 3

// stem applies a deliberately light suffix strip so a term and its common
// inflections collapse to one form: it removes at most ONE of the ordered
// suffixes {ing, ed, es, s, e} and only when the remaining stem is >= minStemLen
// runes. Ordered longest-first so "hashes" -> "hash" (via es), not "hashe".
// This is the fix for the hash/hashes and agent/agents asymmetry; it is NOT a
// linguistic stemmer and makes no claim beyond collapsing those inflections.
func stem(tok string) string {
	// Order matters: try longer, more specific suffixes before shorter ones.
	for _, suf := range []string{"ing", "ed", "es", "s", "e"} {
		if len(tok) > len(suf) && strings.HasSuffix(tok, suf) {
			if trimmed := tok[:len(tok)-len(suf)]; len([]rune(trimmed)) >= minStemLen {
				return trimmed
			}
		}
	}
	return tok
}

// tokenize lowercases text and splits it into alphanumeric tokens, discarding
// every non-alphanumeric rune as a boundary. This is the ONE tokenizer used for
// both the query and node text, so the query side and the match side always
// share a coordinate space.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return fields
}

// LexTerms reduces a query string to its stemmed content terms: it tokenizes,
// drops stopwords, stems each survivor, de-duplicates, and returns them sorted
// for determinism. An empty slice means the query carries NO content signal (an
// all-stopword query) — the gate reads that as "degrade to pure semantic".
func LexTerms(query string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokenize(query) {
		if stopwords[tok] {
			continue
		}
		s := stem(tok)
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// LexicalMatchSet returns the set of keys in texts whose text contains at least
// one of terms, matched TERM-WISE on stemmed tokens (not raw substring). texts
// maps an identity (a node path) to the prose to match against (title+body).
// terms are already stemmed (from LexTerms). A node matches when any stemmed
// query term equals any stemmed token of the node's text — so "hash" and
// "hashes" produce overlapping sets, and "agent" does not match "management"
// the way a raw substring would.
//
// The match set is computed against the LIVE bundle text at query time (the
// caller supplies texts), because the vector index carries no prose: contentHash
// keys on title+body only and a value denormalized onto the index would go stale.
func LexicalMatchSet(texts map[string]string, terms []string) map[string]bool {
	if len(terms) == 0 {
		return map[string]bool{}
	}
	want := make(map[string]bool, len(terms))
	for _, t := range terms {
		want[t] = true
	}
	out := map[string]bool{}
	for path, text := range texts {
		for _, tok := range tokenize(text) {
			if want[stem(tok)] {
				out[path] = true
				break
			}
		}
	}
	return out
}

// LexicalGateOptions configures --lexical-gate (cwest/okfctl#66). It is OFF by
// default (a nil *LexicalGateOptions on QueryOptions). When set, the semantic
// result list is GATED against a term-wise lexical match set and the lexical tail
// the semantic band missed is preserved — the two moves that make this a win on
// both embedders rather than a regression on the default one.
//
// The caller resolves Terms (LexTerms) and Match (LexicalMatchSet against the
// live bundle) and passes them in, so the engine stays free of any bundle
// dependency and the same match set is trivially testable in isolation.
type LexicalGateOptions struct {
	// Terms are the stemmed content terms of the query (from LexTerms). EMPTY
	// terms mean the query carried no lexical signal (all stopwords) — the gate
	// degrades to pure semantic (a no-op).
	Terms []string
	// Match is the set of node paths the lexical terms matched, resolved against
	// the live bundle (from LexicalMatchSet).
	Match map[string]bool
	// OverBroadFraction is the degrade threshold: when Match covers MORE than this
	// fraction of TotalNodes, the term carries no discriminating signal (e.g.
	// "agent" at 172/234 = 73% of the real corpus) and the gate degrades to pure
	// semantic. A value <= 0 disables the over-broad degrade.
	OverBroadFraction float64
	// TotalNodes is the bundle's node count, the denominator of the over-broad
	// check. Zero disables the over-broad degrade (no denominator to reason about).
	TotalNodes int
	// WideN is how many top semantic results form the "band" the lexical set is
	// intersected against. It is clamped up to at least k so the band is never
	// narrower than the requested result count. A wide band (default 50) keeps
	// lexical hits that are semantically mid-ranked in the intersection rather
	// than the tail.
	WideN int
}

// degrades reports whether the gate should behave as a pure no-op: no content
// terms, or a match set so broad it carries no signal. Both conditions are the
// documented degrade-to-semantic rule that keeps question-shaped queries exactly
// as good as they are with the gate off.
func (g *LexicalGateOptions) degrades() bool {
	if len(g.Terms) == 0 {
		return true
	}
	if g.OverBroadFraction > 0 && g.TotalNodes > 0 {
		if float64(len(g.Match)) > g.OverBroadFraction*float64(g.TotalNodes) {
			return true
		}
	}
	return false
}

// applyLexicalGate composes the gated result from a fully-ranked semantic list.
// ranked is the complete semantic ranking (best first). The composition, per
// cwest/okfctl#66:
//
//  1. Take the semantic BAND: the first WideN of ranked.
//  2. Emit the intersection (band ∩ Match) in SEMANTIC order.
//  3. APPEND the lexical hits the band missed, ordered by SEMANTIC SCORE
//     descending (ties broken by path for determinism), each carrying its own
//     semantic score from the full ranking. Score order — not path order —
//     because the caller cuts to k AFTER this composition (cwest/okfctl#73): a
//     path-ordered tail lets an alphabetic prefix decide which lexical hits
//     survive a small k, dropping stronger matches. Recall is unchanged (the
//     same set is preserved); only the order within the tail changes.
//  4. The caller cuts to k.
//
// When the gate degrades (empty terms or over-broad match) this returns ranked
// unchanged — the no-op path that makes the gate safe to leave available.
func applyLexicalGate(ranked []Result, g *LexicalGateOptions, k int) []Result {
	if g == nil || g.degrades() {
		return ranked
	}
	wide := g.WideN
	if wide < k {
		wide = k
	}
	if wide <= 0 || wide > len(ranked) {
		wide = len(ranked)
	}

	inBand := make(map[string]bool, wide)
	var intersection []Result
	for i := 0; i < wide; i++ {
		r := ranked[i]
		inBand[r.Path] = true
		if g.Match[r.Path] {
			intersection = append(intersection, r)
		}
	}

	// Lexical-only tail: matched nodes not already in the band, each carrying its
	// semantic score looked up from the full ranking. Ordered by score DESCENDING
	// (ties by path) so the caller's top-k cut keeps the STRONGEST lexical hits,
	// not an alphabetic prefix (cwest/okfctl#73).
	scoreByPath := make(map[string]Result, len(ranked))
	for _, r := range ranked {
		scoreByPath[r.Path] = r
	}
	var tail []Result
	for path := range g.Match {
		if inBand[path] {
			continue
		}
		r, ok := scoreByPath[path]
		if !ok {
			continue // matched a node absent from the index; nothing to rank
		}
		tail = append(tail, r)
	}
	sort.Slice(tail, func(i, j int) bool {
		if tail[i].Score != tail[j].Score {
			return tail[i].Score > tail[j].Score // higher score first
		}
		return tail[i].Path < tail[j].Path // deterministic tie-break
	})

	out := make([]Result, 0, len(intersection)+len(tail))
	out = append(out, intersection...)
	out = append(out, tail...)
	return out
}
