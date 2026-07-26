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

package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TouchModifiedFile refreshes modified in place, preserving frontmatter key
// order, created, and the body verbatim.
func TestTouchModifiedFilePreservesOrderAndBody(t *testing.T) {
	root := t.TempDir()
	rel := "wine/tannin.md"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-01-01T00:00:00Z\ntags: [a, b]\n---\n\n# Tannin\n\nBody with a [link](acid.md).\n"
	if err := os.WriteFile(abs, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	if err := TouchModifiedFile(abs, at); err != nil {
		t.Fatalf("TouchModifiedFile: %v", err)
	}
	got := readAll(t, abs)

	if !strings.Contains(got, "modified: 2026-07-26T10:00:00Z") {
		t.Fatalf("modified not refreshed:\n%s", got)
	}
	if strings.Contains(got, "modified: 2026-01-01") {
		t.Fatalf("old modified survived:\n%s", got)
	}
	if !strings.Contains(got, "created: 2026-01-01T00:00:00Z") {
		t.Fatalf("created must be untouched:\n%s", got)
	}
	// Frontmatter key order preserved (type, title, created, modified, tags).
	order := []string{"type:", "title:", "created:", "modified:", "tags:"}
	last := -1
	for _, k := range order {
		i := strings.Index(got, k)
		if i < 0 {
			t.Fatalf("key %q missing:\n%s", k, got)
		}
		if i < last {
			t.Fatalf("key order not preserved at %q:\n%s", k, got)
		}
		last = i
	}
	// Body preserved verbatim.
	if !strings.Contains(got, "Body with a [link](acid.md).") {
		t.Fatalf("body not preserved:\n%s", got)
	}
	// Still parses and passes the floor.
	fm, _, err := ParseFrontmatter([]byte(got))
	if err != nil {
		t.Fatalf("result no longer parses: %v", err)
	}
	if fm["type"] != "Concept" {
		t.Fatalf("type lost: %v", fm["type"])
	}
}

// A node lacking modified gains one (a $EDITOR-authored node okfctl now writes),
// without inventing created.
func TestTouchModifiedFileAddsMissingModified(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "n.md")
	if err := os.WriteFile(abs, []byte("---\ntype: Concept\n---\n\n# N\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	if err := TouchModifiedFile(abs, at); err != nil {
		t.Fatalf("TouchModifiedFile: %v", err)
	}
	got := readAll(t, abs)
	if !strings.Contains(got, "modified: 2026-07-26T00:00:00Z") {
		t.Fatalf("modified not added:\n%s", got)
	}
	if strings.Contains(got, "created:") {
		t.Fatalf("created must not be invented:\n%s", got)
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
