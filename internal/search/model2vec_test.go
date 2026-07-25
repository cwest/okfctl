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
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// mkModelDir writes a synthetic Model2Vec dir: config.json + model.safetensors for
// the given matrix, with the given hidden_dim + normalize in config.
func mkModelDir(t *testing.T, m [][]float32, hiddenDim int, normalize bool) string {
	t.Helper()
	dir := t.TempDir()
	cfg := map[string]any{"hidden_dim": hiddenDim, "normalize": normalize}
	cjson, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), cjson, 0o644); err != nil {
		t.Fatal(err)
	}
	// Build safetensors directly into dir/model.safetensors.
	v, d := len(m), 0
	if v > 0 {
		d = len(m[0])
	}
	hdr, _ := json.Marshal(map[string]any{
		"embeddings": map[string]any{"dtype": "F32", "shape": []int{v, d}, "data_offsets": []int{0, v * d * 4}},
	})
	var buf []byte
	lenb := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenb, uint64(len(hdr)))
	buf = append(append(buf, lenb...), hdr...)
	data := make([]byte, v*d*4)
	off := 0
	for _, row := range m {
		for _, f := range row {
			binary.LittleEndian.PutUint32(data[off:], math.Float32bits(f))
			off += 4
		}
	}
	buf = append(buf, data...)
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func l2(v []float64) []float64 {
	var n float64
	for _, x := range v {
		n += x * x
	}
	if n == 0 {
		return v
	}
	n = math.Sqrt(n)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / n
	}
	return out
}

func approxEq(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len %d != %d", len(got), len(want))
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("[%d]=%v want %v", i, got[i], want[i])
		}
	}
}

func TestLoadStaticModel_ReadsConfigAndMatrix(t *testing.T) {
	dir := mkModelDir(t, [][]float32{{1, 0, 0, 0}, {0, 2, 0, 0}, {0, 0, 3, 0}}, 4, true)
	m, err := LoadStaticModel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dim != 4 || !m.Normalize || len(m.Rows) != 3 {
		t.Fatalf("Dim=%d Normalize=%v rows=%d", m.Dim, m.Normalize, len(m.Rows))
	}
}

func TestLoadStaticModel_DimMismatchErrors(t *testing.T) {
	dir := mkModelDir(t, [][]float32{{1, 2, 3, 4}}, 5, true) // config says 5, matrix is 4
	if _, err := LoadStaticModel(dir); err == nil {
		t.Fatal("hidden_dim/matrix mismatch should error")
	}
}

func TestEncodeIDs_SingleRowNormalized(t *testing.T) {
	m := &StaticModel{Rows: [][]float64{{3, 4, 0, 0}, {1, 1, 1, 1}}, Dim: 4, Normalize: true}
	approxEq(t, m.EncodeIDs([]int{0}), l2([]float64{3, 4, 0, 0}))
}

func TestEncodeIDs_MeanPool(t *testing.T) {
	m := &StaticModel{Rows: [][]float64{{2, 0, 0, 0}, {0, 0, 0, 0}, {0, 4, 0, 0}}, Dim: 4, Normalize: true}
	mean := []float64{1, 2, 0, 0} // mean(row0, row2)
	approxEq(t, m.EncodeIDs([]int{0, 2}), l2(mean))
}

func TestEncodeIDs_Empty(t *testing.T) {
	m := &StaticModel{Rows: [][]float64{{1, 2, 3, 4}}, Dim: 4, Normalize: true}
	approxEq(t, m.EncodeIDs(nil), make([]float64, 4))
}

func TestEncodeIDs_OutOfRangeSkipped(t *testing.T) {
	m := &StaticModel{Rows: [][]float64{{3, 4, 0, 0}}, Dim: 4, Normalize: true}
	approxEq(t, m.EncodeIDs([]int{0, 999}), m.EncodeIDs([]int{0})) // 999 skipped, no panic
}

func TestEncodeIDs_NoNormalize(t *testing.T) {
	m := &StaticModel{Rows: [][]float64{{2, 0, 0, 0}, {0, 4, 0, 0}}, Dim: 4, Normalize: false}
	approxEq(t, m.EncodeIDs([]int{0, 1}), []float64{1, 2, 0, 0}) // raw mean, unnormalized
}
