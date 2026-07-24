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
	"testing"
)

func TestHashEmbedder_NameDim(t *testing.T) {
	e := NewHashEmbedder()
	if e.Name() != "hash-test-embedder" {
		t.Errorf("Name = %q, want hash-test-embedder", e.Name())
	}
	if e.Dim() != 64 {
		t.Errorf("Dim = %d, want 64", e.Dim())
	}
}

func TestHashEmbedder_Deterministic(t *testing.T) {
	e := NewHashEmbedder()
	a := e.Encode([]string{"tannin structure wine"})[0]
	b := e.Encode([]string{"tannin structure wine"})[0]
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("nondeterministic at %d: %v != %v", i, a[i], b[i])
		}
	}
}

func TestHashEmbedder_L2Normalized(t *testing.T) {
	e := NewHashEmbedder()
	v := e.Encode([]string{"tannin structure wine"})[0]
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if math.Abs(math.Sqrt(norm)-1.0) > 1e-9 {
		t.Errorf("norm = %v, want ~1.0", math.Sqrt(norm))
	}
	empty := e.Encode([]string{""})[0]
	for _, x := range empty {
		if x != 0 {
			t.Errorf("empty text should give zero vector, got %v", empty)
			break
		}
	}
}

// TestHashEmbedder_MatchesKBProtocol asserts byte-for-byte fidelity with the KB's
// Python HashEmbedder (cwest/knowledge-base tools/okf/embed.py). Captured anchor:
// "tannin structure wine" -> 3 nonzero buckets {41,48,50}, each 1/sqrt(3).
// This is the shared-protocol guarantee (PRD 8.4): a Go-embedded vector equals the
// Python one, so index/query are cross-verifiable across the two implementations.
func TestHashEmbedder_MatchesKBProtocol(t *testing.T) {
	v := NewHashEmbedder().Encode([]string{"tannin structure wine"})[0]
	want := map[int]float64{41: 0.5773502692, 48: 0.5773502692, 50: 0.5773502692}
	for i, x := range v {
		exp := want[i] // 0 for buckets not in the map
		if math.Abs(x-exp) > 1e-9 {
			t.Errorf("bucket %d = %v, want %v (KB protocol fidelity)", i, x, exp)
		}
	}
}

func TestCosine(t *testing.T) {
	if got := cosine([]float64{1, 0}, []float64{0, 1}); got != 0 {
		t.Errorf("orthogonal cosine = %v, want 0", got)
	}
	u := []float64{0.6, 0.8}
	if got := cosine(u, u); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("identical-unit cosine = %v, want 1", got)
	}
	if got := cosine([]float64{0, 0}, []float64{1, 1}); got != 0.0 {
		t.Errorf("zero-norm cosine = %v, want 0.0", got)
	}
}
