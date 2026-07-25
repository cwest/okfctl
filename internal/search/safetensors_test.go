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

// mkSafetensors builds a valid safetensors byte stream for the given [V][D]float32
// matrix: [u64 LE header-length][JSON header][raw LE float32 data]. dtypeOverride,
// when non-empty, replaces "F32" in the header (to exercise the reject path);
// truncate, when >0, drops that many trailing data bytes.
func mkSafetensors(t *testing.T, m [][]float32, dtypeOverride string, truncate int) string {
	t.Helper()
	v := len(m)
	d := 0
	if v > 0 {
		d = len(m[0])
	}
	dtype := "F32"
	if dtypeOverride != "" {
		dtype = dtypeOverride
	}
	nbytes := v * d * 4
	hdr := map[string]any{
		"embeddings": map[string]any{
			"dtype":        dtype,
			"shape":        []int{v, d},
			"data_offsets": []int{0, nbytes},
		},
	}
	hjson, err := json.Marshal(hdr)
	if err != nil {
		t.Fatal(err)
	}
	var buf []byte
	lenb := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenb, uint64(len(hjson)))
	buf = append(buf, lenb...)
	buf = append(buf, hjson...)
	data := make([]byte, nbytes)
	off := 0
	for _, row := range m {
		for _, f := range row {
			binary.LittleEndian.PutUint32(data[off:], math.Float32bits(f))
			off += 4
		}
	}
	if truncate > 0 && truncate <= len(data) {
		data = data[:len(data)-truncate]
	}
	buf = append(buf, data...)

	p := filepath.Join(t.TempDir(), "model.safetensors")
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadSafetensorsMatrix_RoundTrip(t *testing.T) {
	m := [][]float32{
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{-1, -2, -3, -4},
	}
	p := mkSafetensors(t, m, "", 0)
	rows, dim, err := ReadSafetensorsMatrix(p)
	if err != nil {
		t.Fatal(err)
	}
	if dim != 4 || len(rows) != 3 {
		t.Fatalf("dim=%d rows=%d, want 4/3", dim, len(rows))
	}
	for i := range m {
		for j := range m[i] {
			if math.Abs(rows[i][j]-float64(m[i][j])) > 1e-9 {
				t.Errorf("rows[%d][%d]=%v want %v", i, j, rows[i][j], m[i][j])
			}
		}
	}
}

func TestReadSafetensorsMatrix_RejectsNonF32(t *testing.T) {
	p := mkSafetensors(t, [][]float32{{1, 2}}, "F16", 0)
	if _, _, err := ReadSafetensorsMatrix(p); err == nil {
		t.Fatal("non-F32 dtype should error")
	}
}

func TestReadSafetensorsMatrix_RejectsTruncated(t *testing.T) {
	p := mkSafetensors(t, [][]float32{{1, 2, 3, 4}, {5, 6, 7, 8}}, "", 4)
	if _, _, err := ReadSafetensorsMatrix(p); err == nil {
		t.Fatal("truncated data should error")
	}
}
