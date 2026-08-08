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
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// StaticModel is the pure-Go half of a Model2Vec embedder: a token->vector lookup
// matrix plus the inference math (gather rows -> mean-pool -> optional L2-normalize).
// It holds token IDS -> embedding; turning TEXT into token IDs is the tokenizer,
// which lands in increment 5c-2. This mirrors the KB's Model2VecEmbedder inference
// (a model2vec StaticModel from minishlab/potion-base-8M).
type StaticModel struct {
	Rows      [][]float64 // [vocab][dim] embedding table
	Dim       int
	Normalize bool // config.normalize — L2-normalize the pooled vector
}

// model2vecConfig is the subset of config.json this loader reads.
type model2vecConfig struct {
	HiddenDim int  `json:"hidden_dim"`
	Normalize bool `json:"normalize"`
}

// LoadStaticModel reads dir/config.json (hidden_dim, normalize) and
// dir/model.safetensors (the F32 embedding matrix), cross-checking that the matrix
// dimension matches the config's hidden_dim.
func LoadStaticModel(dir string) (*StaticModel, error) {
	// dir is the user-supplied --model-path; reading config.json from the model
	// directory the user named is this loader's intended function.
	cfgRaw, err := os.ReadFile(filepath.Join(dir, "config.json")) //nolint:gosec // G304: user-supplied model dir is the intended input
	if err != nil {
		return nil, err
	}
	var cfg model2vecConfig
	if err := json.Unmarshal(cfgRaw, &cfg); err != nil {
		return nil, fmt.Errorf("model2vec: bad config.json: %w", err)
	}
	rows, dim, err := ReadSafetensorsMatrix(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		return nil, err
	}
	if cfg.HiddenDim != 0 && cfg.HiddenDim != dim {
		return nil, fmt.Errorf("model2vec: config hidden_dim %d != matrix dim %d", cfg.HiddenDim, dim)
	}
	return &StaticModel{Rows: rows, Dim: dim, Normalize: cfg.Normalize}, nil
}

// EncodeIDs turns a token-id sequence into an embedding: it gathers each id's row,
// mean-pools them, and L2-normalizes the result iff Normalize. Out-of-range ids are
// skipped defensively. An empty (or fully-skipped) sequence yields a zero vector of
// length Dim. This is the exact Model2Vec inference math; the only missing piece for
// a full Embedder is text->ids (increment 5c-2).
func (m *StaticModel) EncodeIDs(ids []int) []float64 {
	acc := make([]float64, m.Dim)
	n := 0
	for _, id := range ids {
		if id < 0 || id >= len(m.Rows) {
			continue // skip out-of-range ids rather than panic
		}
		for j, x := range m.Rows[id] {
			acc[j] += x
		}
		n++
	}
	if n == 0 {
		return acc // zero vector
	}
	for j := range acc {
		acc[j] /= float64(n)
	}
	if !m.Normalize {
		return acc
	}
	var norm float64
	for _, x := range acc {
		norm += x * x
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for j := range acc {
			acc[j] /= norm
		}
	}
	return acc
}
