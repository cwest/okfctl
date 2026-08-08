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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

// LoadWordPiece builds a WordPiece tokenizer from a model2vec directory. It
// prefers dir/vocab.txt (line N == token id N — the simplest, fastest form) and
// falls back to dir/tokenizer.json (the standard model2vec / Hugging Face layout
// the docs promise, in which the vocab lives inside the tokenizer JSON). The
// BERT-default normalizer/WordPiece params (## prefix, 100 max chars) are
// hard-defaulted and cross-checked against tokenizer.json when it supplies them.
func LoadWordPiece(dir string) (*WordPiece, error) {
	wp, err := loadWordPieceFromVocabTxt(dir)
	if err == nil {
		return wp, nil
	}
	// Only fall through to tokenizer.json when vocab.txt is simply absent; a
	// vocab.txt that exists but is malformed is a real error, not a miss.
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	tokPath := filepath.Join(dir, "tokenizer.json")
	if wp, tokErr := loadWordPieceFromTokenizerJSON(tokPath); tokErr == nil {
		return wp, nil
	} else if !errors.Is(tokErr, fs.ErrNotExist) {
		return nil, tokErr
	}

	// Neither file exists: name BOTH so the message does not send the user
	// looking for a file the documentation never told them to create.
	return nil, fmt.Errorf("wordpiece: no vocab found in %s: need vocab.txt or tokenizer.json", dir)
}

// loadWordPieceFromVocabTxt reads dir/vocab.txt (line N == token id N) with the
// BERT defaults potion-base-8M's tokenizer.json declares.
func loadWordPieceFromVocabTxt(dir string) (*WordPiece, error) {
	f, err := os.Open(filepath.Join(dir, "vocab.txt")) //nolint:gosec // G304: reading vocab.txt from the user-supplied model dir is intended
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

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

// tokenizerJSON is the subset of a Hugging Face tokenizer.json this loader reads:
// the model section, which for a WordPiece tokenizer carries the vocab (a
// token->id map, so ids come from the values, NOT line order) plus the
// normalizer/WordPiece params. Same stdlib encoding/json approach the config
// reader in model2vec.go uses — no new dependency.
type tokenizerJSON struct {
	Model struct {
		Type                    string         `json:"type"`
		UnkToken                string         `json:"unk_token"`
		ContinuingSubwordPrefix *string        `json:"continuing_subword_prefix"`
		MaxInputCharsPerWord    *int           `json:"max_input_chars_per_word"`
		Vocab                   map[string]int `json:"vocab"`
	} `json:"model"`
}

// loadWordPieceFromTokenizerJSON decodes a WordPiece tokenizer.json into a
// WordPiece. A non-WordPiece model.type (e.g. BPE, Unigram) is rejected with a
// named error rather than attempting a WordPiece parse on an incompatible vocab
// shape. An absent file returns fs.ErrNotExist so the caller can distinguish
// "not present" from "present but broken".
func loadWordPieceFromTokenizerJSON(path string) (*WordPiece, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: reading the user-supplied tokenizer.json path is intended
	if err != nil {
		return nil, err // includes fs.ErrNotExist when absent
	}
	var tj tokenizerJSON
	if err := json.Unmarshal(raw, &tj); err != nil {
		return nil, fmt.Errorf("wordpiece: bad tokenizer.json: %w", err)
	}
	m := tj.Model
	if m.Type != "" && m.Type != "WordPiece" {
		return nil, fmt.Errorf("wordpiece: tokenizer type %q not supported (only WordPiece)", m.Type)
	}
	if len(m.Vocab) == 0 {
		return nil, fmt.Errorf("wordpiece: tokenizer.json has an empty or missing model.vocab")
	}

	unkTok := m.UnkToken
	if unkTok == "" {
		unkTok = "[UNK]"
	}
	unk, ok := m.Vocab[unkTok]
	if !ok {
		return nil, fmt.Errorf("wordpiece: tokenizer.json vocab has no %s token", unkTok)
	}

	prefix := "##"
	if m.ContinuingSubwordPrefix != nil {
		prefix = *m.ContinuingSubwordPrefix
	}
	maxChars := 100
	if m.MaxInputCharsPerWord != nil {
		maxChars = *m.MaxInputCharsPerWord
	}
	return &WordPiece{vocab: m.Vocab, unkID: unk, prefix: prefix, maxChars: maxChars}, nil
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
