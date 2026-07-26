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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// WordPiece is a pure-Go BERT WordPiece tokenizer (the bge-base-en-v1.5 stack
// potion-base-8M declares): BertNormalizer -> BertPreTokenizer -> greedy
// longest-match WordPiece. It returns CONTENT token ids only — never [CLS]/[SEP]
// — because model2vec tokenizes with add_special_tokens=False, and those ids
// must not enter the embedding mean-pool.
type WordPiece struct {
	vocab    map[string]int
	unkID    int
	prefix   string // continuing-subword prefix, "##"
	maxChars int    // max_input_chars_per_word, 100
}

// LoadWordPiece reads dir/vocab.txt (line N == token id N) into a WordPiece with
// the BERT defaults potion-base-8M's tokenizer.json declares.
func LoadWordPiece(dir string) (*WordPiece, error) {
	f, err := os.Open(filepath.Join(dir, "vocab.txt"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	id := 0
	for sc.Scan() {
		tok := strings.TrimRight(sc.Text(), "\r\n")
		if _, dup := vocab[tok]; !dup {
			vocab[tok] = id
		}
		id++
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	unk, ok := vocab["[UNK]"]
	if !ok {
		return nil, fmt.Errorf("wordpiece: vocab.txt has no [UNK] token")
	}
	return &WordPiece{vocab: vocab, unkID: unk, prefix: "##", maxChars: 100}, nil
}

// Tokenize turns text into content token ids: normalize, pre-tokenize into
// words, then greedy longest-match each word into subwords.
func (w *WordPiece) Tokenize(text string) []int {
	var ids []int
	for _, word := range preTokenize(normalize(text)) {
		ids = append(ids, w.wordPiece(word)...)
	}
	return ids
}

// wordPiece greedily splits one word into the longest vocab-matching subwords.
// The first piece matches bare; continuations need the "##" prefix. A word that
// cannot be fully covered (or exceeds maxChars) becomes a single [UNK].
func (w *WordPiece) wordPiece(word string) []int {
	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) > w.maxChars {
		return []int{w.unkID}
	}
	var out []int
	start := 0
	for start < len(runes) {
		end := len(runes)
		matched := -1
		for end > start {
			sub := string(runes[start:end])
			if start > 0 {
				sub = w.prefix + sub
			}
			if id, ok := w.vocab[sub]; ok {
				matched = id
				break
			}
			end--
		}
		if matched < 0 {
			return []int{w.unkID} // no split covers the word
		}
		out = append(out, matched)
		start = end
	}
	return out
}

// normalize applies BertNormalizer {clean_text, handle_chinese_chars, lowercase}:
// drop control chars, space-pad CJK codepoints, collapse whitespace, lowercase.
func normalize(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == 0 || r == 0xFFFD:
			// clean_text: drop NUL / replacement char
		case isControlRune(r):
			// clean_text: control chars become nothing
		case unicode.IsSpace(r):
			b.WriteRune(' ') // clean_text: whitespace normalized to plain space
		case isCJK(r):
			b.WriteRune(' ') // handle_chinese_chars: each CJK char is its own token
			b.WriteRune(r)
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// preTokenize applies BertPreTokenizer: split on whitespace, then split every
// punctuation rune into its own token.
func preTokenize(text string) []string {
	var words []string
	for _, field := range strings.Fields(text) {
		var cur strings.Builder
		for _, r := range field {
			if isPunct(r) {
				if cur.Len() > 0 {
					words = append(words, cur.String())
					cur.Reset()
				}
				words = append(words, string(r))
				continue
			}
			cur.WriteRune(r)
		}
		if cur.Len() > 0 {
			words = append(words, cur.String())
		}
	}
	return words
}

func isControlRune(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false // treated as whitespace, not dropped
	}
	return unicode.IsControl(r)
}

// isCJK reports the CJK ranges BERT's handle_chinese_chars covers.
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F)
}

// isPunct matches BERT's punctuation rule: Unicode punctuation/symbol plus the
// ASCII punct ranges BERT treats as punctuation.
func isPunct(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) ||
		(r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}
