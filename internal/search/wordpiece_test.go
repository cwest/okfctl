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

// mkWordPiece writes a tiny vocab.txt (id == line number, 0-indexed) and loads a
// WordPiece over it. The vocab is hand-built to cover the test anchors so unit
// tests need no 30 MB model download.
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

func TestTokenize_PunctSplit(t *testing.T) {
	wp := mkWordPiece(t, testVocab())
	if got := wp.Tokenize("notes."); !idsEq(got, []int{7, 8}) { // notes + .
		t.Errorf("Tokenize(notes.) = %v, want [7 8]", got)
	}
}

func TestTokenize_NoSpecialTokens(t *testing.T) {
	// model2vec tokenizes with add_special_tokens=False; we must never wrap [CLS]/[SEP].
	wp := mkWordPiece(t, testVocab())
	got := wp.Tokenize("tannin")
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("Tokenize(tannin) = %v, want [2] (no specials)", got)
	}
}
