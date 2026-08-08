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
	"os"
	"path/filepath"
	"strings"
)

// Model2VecEmbedder joins the two halves built in 5c-1 and 5c-2 — the WordPiece
// tokenizer (text -> ids) and the StaticModel (ids -> vector) — into a single
// Embedder. It is the semantically meaningful counterpart to HashEmbedder, and
// it stays stdlib-only: no CGO, no runtime download, no Python. Encode is a
// faithful port of model2vec's StaticModel.encode, so a vector produced here is
// interchangeable with one produced by the KB's Python embedder.
type Model2VecEmbedder struct {
	model *StaticModel
	tok   *WordPiece
	name  string
}

// LoadModel2VecEmbedder loads a local model2vec directory (config.json,
// model.safetensors, vocab.txt or tokenizer.json). The directory must already
// exist on disk — okfctl never downloads a model at runtime.
func LoadModel2VecEmbedder(dir string) (*Model2VecEmbedder, error) {
	model, err := LoadStaticModel(dir)
	if err != nil {
		return nil, err
	}
	tok, err := LoadWordPiece(dir)
	if err != nil {
		return nil, err
	}
	return &Model2VecEmbedder{model: model, tok: tok, name: modelName(dir)}, nil
}

// modelName reports a human-readable model identity, which an index stores so a
// later query can refuse vectors built by a different model. Precedence: an
// explicit name in config.json, then the HuggingFace cache layout, then the
// directory name.
func modelName(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "config.json")) //nolint:gosec // G304: reading config.json from the user-supplied model dir is intended
	if err == nil {
		var cfg struct {
			ModelName string `json:"model_name"`
			BaseModel string `json:"base_model_name"`
		}
		if json.Unmarshal(raw, &cfg) == nil {
			if cfg.ModelName != "" {
				return cfg.ModelName
			}
			if cfg.BaseModel != "" {
				return cfg.BaseModel
			}
		}
	}
	if name := huggingFaceCacheName(dir); name != "" {
		return name
	}
	return filepath.Base(filepath.Clean(dir))
}

// huggingFaceCacheName recovers a readable model id from HuggingFace's cache
// layout, .../models--<org>--<name>/snapshots/<revision>, whose leaf directory
// is a bare commit hash. Without this, an index's provenance reads as an opaque
// SHA. The revision is preserved as a suffix because two snapshots of the same
// repo can hold different weights, and an index must not silently treat them as
// interchangeable. Returns "" when dir is not a HuggingFace cache path.
func huggingFaceCacheName(dir string) string {
	clean := filepath.Clean(dir)
	rev := filepath.Base(clean)
	parent := filepath.Dir(clean)
	if filepath.Base(parent) != "snapshots" {
		return ""
	}
	repo := filepath.Base(filepath.Dir(parent))
	if !strings.HasPrefix(repo, "models--") {
		return ""
	}
	name := strings.ReplaceAll(strings.TrimPrefix(repo, "models--"), "--", "/")
	if name == "" {
		return ""
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return name + "@" + rev
}

func (m *Model2VecEmbedder) Name() string { return m.name }
func (m *Model2VecEmbedder) Dim() int     { return m.model.Dim }

// Encode embeds each text independently: tokenize to content ids (no [CLS]/[SEP]),
// gather the rows, mean-pool, and normalize per the model's own config. Text that
// tokenizes to nothing yields the zero vector rather than an error, matching
// model2vec's behavior for empty input.
func (m *Model2VecEmbedder) Encode(texts []string) [][]float64 {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		out[i] = m.model.EncodeIDs(m.tok.Tokenize(t))
	}
	return out
}
