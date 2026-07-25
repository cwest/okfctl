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
	"math"
	"os"
	"testing"
)

// potionDir returns a real potion-base-8M model dir from OKFCTL_TEST_MODEL_DIR,
// or skips: the fidelity tests need the actual model, which is not vendored.
func potionDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("OKFCTL_TEST_MODEL_DIR")
	if dir == "" {
		t.Skip("set OKFCTL_TEST_MODEL_DIR to a potion-base-8M dir to run model fidelity tests")
	}
	return dir
}

func TestModel2VecEmbedder_Interface(t *testing.T) {
	var _ Embedder = (*Model2VecEmbedder)(nil) // compile-time: satisfies the 5b seam
}

// TestModel2VecEmbedder_TokenizerFidelity proves the Go WordPiece produces the
// SAME content ids as the HuggingFace tokenizer on the real 29528-token vocab.
// Anchors captured from the live tokenizer (add_special_tokens=False).
func TestModel2VecEmbedder_TokenizerFidelity(t *testing.T) {
	wp, err := LoadWordPiece(potionDir(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]int{
		"tannin structure":   {8098, 10489, 2258},
		"Wine":               {3517},
		"oaky vanilla notes": {5122, 1106, 20167, 2970},
		"":                   nil,
	}
	for in, want := range cases {
		got := wp.Tokenize(in)
		if !idsEq(got, want) {
			t.Errorf("Tokenize(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestModel2VecEmbedder_EmbeddingFidelity is the END-TO-END proof: Go
// (tokenize -> gather -> mean-pool -> normalize) reproduces model2vec's own
// StaticModel.encode output. Anchor captured from
// model2vec.StaticModel.from_pretrained("minishlab/potion-base-8M").encode.
func TestModel2VecEmbedder_EmbeddingFidelity(t *testing.T) {
	e, err := LoadModel2VecEmbedder(potionDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if e.Dim() != 256 {
		t.Fatalf("Dim() = %d, want 256", e.Dim())
	}
	v := e.Encode([]string{"tannin structure"})[0]
	if len(v) != 256 {
		t.Fatalf("vector len = %d, want 256", len(v))
	}
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if math.Abs(math.Sqrt(norm)-1.0) > 1e-6 {
		t.Errorf("norm = %v, want 1.0", math.Sqrt(norm))
	}
	want := []float64{0.236271, -0.08241, -0.142059, -0.152239}
	for i, w := range want {
		if math.Abs(v[i]-w) > 1e-5 {
			t.Errorf("v[%d] = %v, want %v (model2vec parity)", i, v[i], w)
		}
	}
}

func TestModel2VecEmbedder_EmptyText(t *testing.T) {
	e, err := LoadModel2VecEmbedder(potionDir(t))
	if err != nil {
		t.Fatal(err)
	}
	v := e.Encode([]string{""})[0]
	for _, x := range v {
		if x != 0 {
			t.Errorf("empty text should give the zero vector, got %v...", v[:4])
			break
		}
	}
}
