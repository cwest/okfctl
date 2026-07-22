// Copyright 2026 Casey West
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

package okf

import (
	"os"
	"strings"
	"testing"
)

func TestNewNode_WritesConformantFile(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	path, err := NewNode(dir, "wine/tannin.md", "Reference", "Tannin")
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "type: Reference") {
		t.Errorf("node missing type; got:\n%s", data)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("authored node must validate clean, got %v", f)
	}
}

func TestNewNode_RejectsEmptyType(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if _, err := NewNode(dir, "x.md", "", "X"); err == nil {
		t.Fatal("NewNode with empty type must error")
	}
}

func TestNewNode_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if _, err := NewNode(dir, "a.md", "Reference", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewNode(dir, "a.md", "Reference", "A2"); err == nil {
		t.Fatal("NewNode must refuse to overwrite an existing node")
	}
}
