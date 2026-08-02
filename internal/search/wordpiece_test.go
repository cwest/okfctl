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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The docs (README.md, skills/okf-semantic-search/SKILL.md) promise the standard
// model2vec / Hugging Face layout, where the tokenizer ships as tokenizer.json.
// These tests pin all three vocab-source shapes LoadWordPiece must handle.

// TestLoadWordPiece_VocabTxtOnly is the POSITIVE CONTROL for the existing fast
// path: a directory with only vocab.txt (line N == id N, the potion-base-8M
// shape) loads unchanged.
func TestLoadWordPiece_VocabTxtOnly(t *testing.T) {
	dir := t.TempDir()
	// Line 0 = [UNK], then a couple of content tokens.
	writeFile(t, filepath.Join(dir, "vocab.txt"), "[UNK]\nwine\ntannin\n")

	wp, err := LoadWordPiece(dir)
	if err != nil {
		t.Fatalf("LoadWordPiece(vocab.txt-only) errored: %v", err)
	}
	if wp.unkID != 0 {
		t.Errorf("unkID = %d, want 0 (line number of [UNK])", wp.unkID)
	}
	if got := wp.vocab["tannin"]; got != 2 {
		t.Errorf("vocab[tannin] = %d, want 2 (line number)", got)
	}
	if wp.prefix != "##" || wp.maxChars != 100 {
		t.Errorf("defaults = prefix %q maxChars %d, want \"##\" 100", wp.prefix, wp.maxChars)
	}
}

// TestLoadWordPiece_TokenizerJSONOnly is the core fix: a directory with only
// tokenizer.json (the documented HF layout, no vocab.txt) loads by decoding the
// WordPiece model section. Vocab ids come from the map values, NOT line numbers.
func TestLoadWordPiece_TokenizerJSONOnly(t *testing.T) {
	dir := t.TempDir()
	// A minimal but well-formed WordPiece tokenizer.json. Note [UNK] id is 1,
	// exactly like the real potion-base-8M — ids come from the map, not order.
	writeFile(t, filepath.Join(dir, "tokenizer.json"), `{
	  "model": {
	    "type": "WordPiece",
	    "unk_token": "[UNK]",
	    "continuing_subword_prefix": "##",
	    "max_input_chars_per_word": 100,
	    "vocab": {"[PAD]": 0, "[UNK]": 1, "wine": 2, "tannin": 3}
	  }
	}`)

	wp, err := LoadWordPiece(dir)
	if err != nil {
		t.Fatalf("LoadWordPiece(tokenizer.json-only) errored: %v", err)
	}
	if wp.unkID != 1 {
		t.Errorf("unkID = %d, want 1 (from vocab map, not line number)", wp.unkID)
	}
	if got := wp.vocab["tannin"]; got != 3 {
		t.Errorf("vocab[tannin] = %d, want 3 (from vocab map)", got)
	}
	if wp.prefix != "##" || wp.maxChars != 100 {
		t.Errorf("params = prefix %q maxChars %d, want \"##\" 100", wp.prefix, wp.maxChars)
	}
	// The tokenizer must actually work off the decoded vocab.
	if ids := wp.Tokenize("wine"); len(ids) != 1 || ids[0] != 2 {
		t.Errorf("Tokenize(wine) = %v, want [2]", ids)
	}
}

// TestLoadWordPiece_VocabTxtWins pins precedence: when BOTH files are present,
// the vocab.txt fast path is taken exactly as before (no behavior change for the
// real corpus, which ships both).
func TestLoadWordPiece_VocabTxtWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "vocab.txt"), "[UNK]\nwine\n")
	// A tokenizer.json with a DIFFERENT id scheme; must be ignored.
	writeFile(t, filepath.Join(dir, "tokenizer.json"), `{
	  "model": {"type": "WordPiece", "unk_token": "[UNK]",
	  "vocab": {"[UNK]": 99, "wine": 42}}
	}`)

	wp, err := LoadWordPiece(dir)
	if err != nil {
		t.Fatalf("LoadWordPiece(both) errored: %v", err)
	}
	if wp.unkID != 0 || wp.vocab["wine"] != 1 {
		t.Errorf("vocab.txt must win: unkID=%d wine=%d, want 0 and 1", wp.unkID, wp.vocab["wine"])
	}
}

// TestLoadWordPiece_TokenizerJSONWrongType is the NEGATIVE CONTROL: a
// tokenizer.json declaring a non-WordPiece model.type (e.g. BPE) must fail with
// a NAMED "tokenizer type not supported" error, not silently attempt a
// WordPiece parse on an incompatible vocab shape.
func TestLoadWordPiece_TokenizerJSONWrongType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tokenizer.json"), `{
	  "model": {"type": "BPE", "unk_token": "[UNK]", "vocab": {"[UNK]": 0}}
	}`)

	_, err := LoadWordPiece(dir)
	if err == nil {
		t.Fatal("LoadWordPiece(BPE tokenizer.json) should error, got nil")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error %q should name the unsupported tokenizer type", err.Error())
	}
	if !strings.Contains(err.Error(), "BPE") {
		t.Errorf("error %q should name the offending type (BPE)", err.Error())
	}
}

// TestLoadWordPiece_TokenizerJSONNoUnk guards a malformed WordPiece
// tokenizer.json whose vocab lacks [UNK].
func TestLoadWordPiece_TokenizerJSONNoUnk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tokenizer.json"), `{
	  "model": {"type": "WordPiece", "vocab": {"wine": 0}}
	}`)

	_, err := LoadWordPiece(dir)
	if err == nil {
		t.Fatal("LoadWordPiece(no [UNK]) should error, got nil")
	}
	if !strings.Contains(err.Error(), "[UNK]") {
		t.Errorf("error %q should name the missing [UNK] token", err.Error())
	}
}

// TestLoadWordPiece_NeitherFile pins the fixed error message: when neither
// vocab.txt nor tokenizer.json exists, the error must name BOTH so it stops
// sending the user to look for a file the docs never told them to create.
func TestLoadWordPiece_NeitherFile(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadWordPiece(dir)
	if err == nil {
		t.Fatal("LoadWordPiece(empty dir) should error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "vocab.txt") || !strings.Contains(msg, "tokenizer.json") {
		t.Errorf("error %q must name BOTH vocab.txt and tokenizer.json", msg)
	}
}

// mkWordPiece writes a tiny vocab.txt (id == line number, 0-indexed) and loads a
// WordPiece over it. The vocab is hand-built to cover the test anchors so these
// tokenizer unit tests need no 30 MB model download — they run unconditionally
// in CI, unlike the model-gated fidelity tests in model2vec_embedder_test.go.
func mkWordPiece(t *testing.T, tokens []string) *WordPiece {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vocab.txt"),
		[]byte(strings.Join(tokens, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wp, err := LoadWordPiece(dir)
	if err != nil {
		t.Fatalf("LoadWordPiece: %v", err)
	}
	return wp
}

// vocab layout (id = index): 0 [PAD] 1 [UNK] 2 tannin 3 structure 4 wine 5 oak
// 6 ##y 7 notes 8 .
func testVocab() []string {
	return []string{"[PAD]", "[UNK]", "tannin", "structure", "wine", "oak", "##y", "notes", "."}
}

// TestTokenize_Anchors pins the core tokenizer semantics on a hand-built vocab:
// content-id lookup, BertNormalizer lowercasing, greedy WordPiece subword split
// (oaky -> oak + ##y), and no [CLS]/[SEP] specials (model2vec tokenizes with
// add_special_tokens=False). Runs in CI without a model download.
func TestTokenize_Anchors(t *testing.T) {
	wp := mkWordPiece(t, testVocab())
	cases := []struct {
		in   string
		want []int
	}{
		{"tannin structure", []int{2, 3}},
		{"Wine", []int{4}},    // lowercase -> wine
		{"oaky", []int{5, 6}}, // WordPiece split: oak + ##y
		{"", nil},             // empty -> no tokens (NO [CLS]/[SEP])
	}
	for _, c := range cases {
		got := wp.Tokenize(c.in)
		if !idsEq(got, c.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestTokenize_UnknownWord pins the [UNK] fallback: an out-of-vocab word and a
// word exceeding max_input_chars_per_word (100) both collapse to a single [UNK].
func TestTokenize_UnknownWord(t *testing.T) {
	wp := mkWordPiece(t, testVocab())
	if got := wp.Tokenize("zzzznotavocabword"); !idsEq(got, []int{1}) { // [UNK]==1
		t.Errorf("unknown word = %v, want [1]", got)
	}
	long := strings.Repeat("a", 101)
	if got := wp.Tokenize(long); !idsEq(got, []int{1}) {
		t.Errorf(">100-char word = %v, want [1] (unk)", got)
	}
}

// TestTokenize_PunctSplit pins the BertPreTokenizer punctuation rule: a trailing
// period splits into its own token.
func TestTokenize_PunctSplit(t *testing.T) {
	wp := mkWordPiece(t, testVocab())
	if got := wp.Tokenize("notes."); !idsEq(got, []int{7, 8}) { // notes + .
		t.Errorf("Tokenize(notes.) = %v, want [7 8]", got)
	}
}

// TestTokenize_NoSpecialTokens guards the add_special_tokens=False contract: the
// tokenizer must never wrap output in [CLS]/[SEP], or those ids would poison the
// embedding mean-pool.
func TestTokenize_NoSpecialTokens(t *testing.T) {
	wp := mkWordPiece(t, testVocab())
	got := wp.Tokenize("tannin")
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("Tokenize(tannin) = %v, want [2] (no specials)", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// idsEq reports whether two token-id slices are element-wise equal, treating a
// nil slice and an empty slice as equal (the tokenizer returns nil for empty
// input). Used by the tokenizer-fidelity assertions.
func idsEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
