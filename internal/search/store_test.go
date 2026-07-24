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
	"os"
	"path/filepath"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// writeBundle lays down a bundle dir with the given rel->content files and loads it.
func writeBundle(t *testing.T, files map[string]string) (*okf.Bundle, string) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("okf.Load: %v", err)
	}
	return b, dir
}

func node(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

func fixtureBundle(t *testing.T) (*okf.Bundle, string) {
	return writeBundle(t, map[string]string{
		"index.md":        "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/tannin.md)\n",
		"wine/tannin.md":  node("Concept", "Tannin", "Tannin gives structure and astringency to wine."),
		"wine/acidity.md": node("Concept", "Acidity", "Acidity gives freshness and lift to wine."),
		"wine/oak.md":     node("Concept", "Oak", "Oak barrels add vanilla and spice notes."),
	})
}

func TestStore_RoundTrip(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	if s.Model != e.Name() || s.Dim != e.Dim() {
		t.Errorf("store model/dim = %q/%d, want %q/%d", s.Model, s.Dim, e.Name(), e.Dim())
	}
	if len(s.Entries) != 3 { // concept nodes only (index excluded)
		t.Fatalf("want 3 entries, got %d", len(s.Entries))
	}
	for _, en := range s.Entries {
		if len(en.Vector) != e.Dim() {
			t.Errorf("entry %s vector dim = %d, want %d", en.Path, len(en.Vector), e.Dim())
		}
		if en.Hash == "" {
			t.Errorf("entry %s missing content hash", en.Path)
		}
	}
	p := filepath.Join(t.TempDir(), "index.db")
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Model != s.Model || loaded.Dim != s.Dim || len(loaded.Entries) != len(s.Entries) {
		t.Errorf("round-trip mismatch: %+v vs %+v", loaded, s)
	}
}

func TestStore_Deterministic(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	p1 := filepath.Join(t.TempDir(), "a.db")
	p2 := filepath.Join(t.TempDir(), "b.db")
	if err := BuildIndex(b, e, nil).Save(p1); err != nil {
		t.Fatal(err)
	}
	if err := BuildIndex(b, e, nil).Save(p2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(p1)
	c, _ := os.ReadFile(p2)
	if string(a) != string(c) {
		t.Error("BuildIndex not byte-deterministic for a fixed embedder")
	}
}

func TestStore_ContentHashSkip(t *testing.T) {
	b, dir := fixtureBundle(t)
	e := NewHashEmbedder()
	prev := BuildIndex(b, e, nil)
	prevHash := map[string]string{}
	for _, en := range prev.Entries {
		prevHash[en.Path] = en.Hash
	}
	// Change one node's body.
	if err := os.WriteFile(filepath.Join(dir, "wine/oak.md"),
		[]byte(node("Concept", "Oak", "CHANGED: heavy toast and smoke.")), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := okf.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	next := BuildIndex(b2, e, prev)
	for _, en := range next.Entries {
		if en.Path == "wine/oak.md" {
			if en.Hash == prevHash[en.Path] {
				t.Error("changed node should have a new content hash")
			}
		} else if en.Hash != prevHash[en.Path] {
			t.Errorf("unchanged node %s hash drifted", en.Path)
		}
	}
}

func TestStore_ModelMismatchGuard(t *testing.T) {
	b, _ := fixtureBundle(t)
	s := BuildIndex(b, NewHashEmbedder(), nil)
	s.Model = "some-other-model" // simulate a store built under a different model
	other := NewHashEmbedder()
	if _, err := Query(s, other, "tannin", 3); err != ErrModelMismatch {
		t.Errorf("Query across models = %v, want ErrModelMismatch", err)
	}
}
