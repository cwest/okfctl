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

// Package search implements okfctl's offline semantic search over a bundle's
// concept nodes: the shared Embedder contract, an embedded flat vector store,
// and cosine query/related. It is stdlib-only and imports no cobra, no net/http,
// and no CGO — the whole point of okfctl-search shipping as a single static
// plugin binary. The Embedder protocol and the HashEmbedder are ported faithfully
// from cwest/knowledge-base tools/okf/embed.py so vectors are cross-verifiable
// (PRD §8.4: port the protocol, do not duplicate it).
package search

import (
	"crypto/sha1" //nolint:gosec // G505: SHA1 is a non-cryptographic feature-hash bucket, not a security primitive
	"encoding/binary"
	"math"
	"strings"
)

// Embedder is the minimal interface every OKF search consumer depends on,
// mirroring the KB's Embedder Protocol (name, dim, encode).
type Embedder interface {
	Name() string
	Dim() int
	Encode(texts []string) [][]float64
}

// HashEmbedder is a deterministic, dependency-free embedder: it maps text to a
// fixed-dimension L2-normalized vector via hashed token bucketing. It is NOT
// semantically meaningful — it exists so okfctl-search works fully offline with
// zero model download, and so index/query are exercisable end-to-end. Ported
// byte-for-byte from the KB HashEmbedder (sha1 bucket + sign, L2-normalize) so
// the same text yields the same vector in Go and Python.
type HashEmbedder struct {
	dim  int
	name string
}

// NewHashEmbedder returns the default offline embedder (dim 64, matching the KB).
func NewHashEmbedder() *HashEmbedder {
	return &HashEmbedder{dim: 64, name: "hash-test-embedder"}
}

func (h *HashEmbedder) Name() string { return h.name }
func (h *HashEmbedder) Dim() int     { return h.dim }

func (h *HashEmbedder) Encode(texts []string) [][]float64 {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		out[i] = h.embedOne(t)
	}
	return out
}

func (h *HashEmbedder) embedOne(text string) []float64 {
	vec := make([]float64, h.dim)
	for _, tok := range strings.Fields(strings.ToLower(text)) {
		// SHA1 here is a deterministic feature-hash bucket selector, not a
		// security mechanism; it is ported byte-for-byte from the KB embedder so
		// Go and Python produce identical vectors (see package doc).
		sum := sha1.Sum([]byte(tok)) //nolint:gosec // G401: non-cryptographic feature hashing, cross-language parity required
		idx := int(binary.BigEndian.Uint32(sum[:4])) % h.dim
		sign := 1.0
		if sum[4]%2 != 0 {
			sign = -1.0
		}
		vec[idx] += sign
	}
	var norm float64
	for _, x := range vec {
		norm += x * x
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// cosine returns the cosine similarity of two equal-length vectors (0.0 if
// either is a zero vector), mirroring the KB cosine().
func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
