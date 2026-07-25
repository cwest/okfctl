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
	"fmt"
	"math"
	"os"
)

// safetensorsEntry is one tensor's header record.
type safetensorsEntry struct {
	Dtype       string `json:"dtype"`
	Shape       []int  `json:"shape"`
	DataOffsets []int  `json:"data_offsets"`
}

// ReadSafetensorsMatrix reads the 2-D F32 "embeddings" tensor from a safetensors
// file and returns it as [V][D]float64 plus D. The format is [u64 little-endian
// header-length][JSON header][raw tensor data]; row r's D floats live at data
// offset r*D*4 as little-endian float32. stdlib-only — no safetensors library,
// no CGO. F32 is the only dtype this loader supports (potion-base-8M is F32); any
// other dtype errors rather than silently misreading.
func ReadSafetensorsMatrix(path string) ([][]float64, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(raw) < 8 {
		return nil, 0, fmt.Errorf("safetensors: file too short for header length")
	}
	hlen := binary.LittleEndian.Uint64(raw[:8])
	if uint64(len(raw)) < 8+hlen {
		return nil, 0, fmt.Errorf("safetensors: header length %d exceeds file size", hlen)
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:8+hlen], &header); err != nil {
		return nil, 0, fmt.Errorf("safetensors: bad JSON header: %w", err)
	}
	rawEntry, ok := header["embeddings"]
	if !ok {
		return nil, 0, fmt.Errorf("safetensors: no \"embeddings\" tensor in header")
	}
	var e safetensorsEntry
	if err := json.Unmarshal(rawEntry, &e); err != nil {
		return nil, 0, fmt.Errorf("safetensors: bad embeddings entry: %w", err)
	}
	if e.Dtype != "F32" {
		return nil, 0, fmt.Errorf("safetensors: embeddings dtype %q unsupported (only F32)", e.Dtype)
	}
	if len(e.Shape) != 2 {
		return nil, 0, fmt.Errorf("safetensors: embeddings shape %v is not 2-D", e.Shape)
	}
	if len(e.DataOffsets) != 2 {
		return nil, 0, fmt.Errorf("safetensors: embeddings data_offsets malformed: %v", e.DataOffsets)
	}
	v, d := e.Shape[0], e.Shape[1]
	start, end := e.DataOffsets[0], e.DataOffsets[1]
	if end-start != v*d*4 {
		return nil, 0, fmt.Errorf("safetensors: data span %d != %d (V*D*4)", end-start, v*d*4)
	}
	dataStart := int(8 + hlen)
	if dataStart+end > len(raw) {
		return nil, 0, fmt.Errorf("safetensors: data section truncated (need %d bytes, have %d)",
			end, len(raw)-dataStart)
	}
	data := raw[dataStart+start : dataStart+end]

	rows := make([][]float64, v)
	off := 0
	for r := 0; r < v; r++ {
		row := make([]float64, d)
		for c := 0; c < d; c++ {
			row[c] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off:])))
			off += 4
		}
		rows[r] = row
	}
	return rows, d, nil
}
