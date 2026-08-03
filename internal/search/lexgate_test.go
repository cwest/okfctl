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
	"reflect"
	"testing"
)

// TestLexTerms_DropsStopwordsAndStems is the tokenizer contract: a question-shaped
// query loses its stopwords and keeps the content terms, stemmed. This is the
// fix for the reproduced 0-hit failure — a phrase-wise body match on the whole
// question returns nothing, so the gate must reduce the query to its content
// terms before matching.
func TestLexTerms_DropsStopwordsAndStems(t *testing.T) {
	got := LexTerms("how should an agent decide when to delegate work")
	want := []string{"agent", "decid", "delegat", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LexTerms question-shaped: got %v want %v", got, want)
	}
}

// TestLexTerms_AllStopwordsEmpty pins the empty-term degrade signal: a query made
// entirely of stopwords yields no content terms, which the gate reads as
// "degrade to pure semantic".
func TestLexTerms_AllStopwordsEmpty(t *testing.T) {
	if got := LexTerms("how should the"); len(got) != 0 {
		t.Errorf("all-stopword query should yield no terms; got %v", got)
	}
}

// TestStem_PluralSymmetry is the load-bearing morphology fix: hash and hashes
// (18 vs 0 raw-substring hits on the real corpus) must collapse to one stem, and
// so must agent/agents (172 vs 100).
func TestStem_PluralSymmetry(t *testing.T) {
	pairs := [][2]string{
		{"hash", "hashes"},
		{"agent", "agents"},
		{"delegate", "delegated"},
		{"index", "indexing"},
	}
	for _, p := range pairs {
		if stem(p[0]) != stem(p[1]) {
			t.Errorf("stem(%q)=%q != stem(%q)=%q; asymmetry not fixed",
				p[0], stem(p[0]), p[1], stem(p[1]))
		}
	}
}

// TestStem_ShortTokensIntact pins that the stemmer does not over-strip short
// tokens: a two/three-letter word must survive so a real identifier like "ci"
// or "go" is not mangled into nothing.
func TestStem_ShortTokensIntact(t *testing.T) {
	for _, w := range []string{"is", "go", "ci", "os"} {
		if got := stem(w); got != w {
			t.Errorf("stem(%q) should leave a short token intact; got %q", w, got)
		}
	}
}

// TestLexicalMatchSet_StemSymmetry is the match-set half of the asymmetry fix:
// on a corpus with one node mentioning "hash" and another mentioning "hashes",
// querying either term returns an OVERLAPPING set. Raw substring gave 18 vs 0;
// term-wise stemmed matching must give the same nodes both directions.
func TestLexicalMatchSet_StemSymmetry(t *testing.T) {
	texts := map[string]string{
		"a.md": "Content hashes key the re-embed decision.",
		"b.md": "A content hash keys the cache.",
		"c.md": "Unrelated prose about wine and tannin.",
	}
	fromSingular := LexicalMatchSet(texts, LexTerms("hash"))
	fromPlural := LexicalMatchSet(texts, LexTerms("hashes"))

	if !reflect.DeepEqual(fromSingular, fromPlural) {
		t.Errorf("hash/hashes gate to different sets:\n hash=%v\n hashes=%v", fromSingular, fromPlural)
	}
	// Both must include BOTH the hash node and the hashes node.
	for _, want := range []string{"a.md", "b.md"} {
		if !fromSingular[want] {
			t.Errorf("expected %q in the match set; got %v", want, fromSingular)
		}
	}
	if fromSingular["c.md"] {
		t.Errorf("unrelated node c.md must not match; got %v", fromSingular)
	}
}

// TestLexicalMatchSet_ZeroMatchEmpty pins that a content term matching nothing
// yields an empty set (not an error, not the whole corpus).
func TestLexicalMatchSet_ZeroMatchEmpty(t *testing.T) {
	texts := map[string]string{"a.md": "wine and tannin"}
	if got := LexicalMatchSet(texts, LexTerms("kubernetes")); len(got) != 0 {
		t.Errorf("no-match term should yield empty set; got %v", got)
	}
}

// TestLexicalMatchSet_EmptyTermsEmpty pins that no content terms (an all-stopword
// query) yields an empty set — the empty-term degrade signal for the gate.
func TestLexicalMatchSet_EmptyTermsEmpty(t *testing.T) {
	texts := map[string]string{"a.md": "wine and tannin"}
	if got := LexicalMatchSet(texts, LexTerms("how should the")); len(got) != 0 {
		t.Errorf("empty terms should yield empty set; got %v", got)
	}
}
