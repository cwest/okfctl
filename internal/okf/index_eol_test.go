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
)

// toCRLF rewrites a file with CRLF endings, as a Windows checkout with
// core.autocrlf=true would materialise it.
func toCRLF(t *testing.T, p string) {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	crlf := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n", "\r\n")
	if err := os.WriteFile(p, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func buildAndReload(t *testing.T, dir string) *Bundle {
	t.Helper()
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	b, err = Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUsesCRLF(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"no newline", false},
		{"a\nb\n", false},
		{"a\r\nb\r\n", true},
		{"a\r\nb", true},
		{"a\r\nb\n", false}, // mixed falls back to LF
		{"a\rb\r", false},   // bare CR is not a line ending we emit
	}
	for _, c := range cases {
		if got := usesCRLF([]byte(c.in)); got != c.want {
			t.Errorf("usesCRLF(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// A CRLF checkout of an LF-built index carries the same content, so index check
// must report it in sync — and must still catch real drift.
func TestIndexInSync_CRLFCheckoutIsInSync(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	_ = buildAndReload(t, dir)
	toCRLF(t, filepath.Join(dir, "index.md"))
	toCRLF(t, filepath.Join(dir, "wine", "index.md"))

	b, _ := Load(dir)
	if ok, diff := IndexInSync(b); !ok {
		t.Errorf("CRLF checkout of a current index must be in sync; got: %s", diff)
	}
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	b, _ = Load(dir)
	if ok, _ := IndexInSync(b); ok {
		t.Error("a CRLF index must still be reported stale after adding a node")
	}
}

// index build keeps a CRLF index CRLF, writes a brand-new index LF, and
// normalises a mixed-ending index to LF.
func TestWriteIndex_PreservesExistingLineEndings(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	writeNode(t, dir, "beer/hops.md", "Reference", "Hops")
	_ = buildAndReload(t, dir)

	root := filepath.Join(dir, "index.md")
	wine := filepath.Join(dir, "wine", "index.md")
	beer := filepath.Join(dir, "beer", "index.md")
	toCRLF(t, root)
	toCRLF(t, wine)
	// beer/index.md: mixed — one CRLF line, the rest LF.
	if err := os.WriteFile(beer, []byte(strings.Replace(readFile(t, beer), "\n", "\r\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity") // forces a real wine/ change
	writeNode(t, dir, "cider/apple.md", "Reference", "Apple")    // a brand-new index

	b := buildAndReload(t, dir)
	if ok, diff := IndexInSync(b); !ok {
		t.Fatalf("index must be in sync right after build; got: %s", diff)
	}
	for _, p := range []string{root, wine} {
		if got := readFile(t, p); !usesCRLF([]byte(got)) {
			t.Errorf("%s: CRLF index must stay CRLF after build; got %q", p, got)
		}
	}
	if got := readFile(t, wine); !strings.Contains(got, "acidity.md") {
		t.Errorf("wine/index.md must list the new node; got %q", got)
	}
	for _, p := range []string{beer, filepath.Join(dir, "cider", "index.md")} {
		if got := readFile(t, p); strings.Contains(got, "\r") {
			t.Errorf("%s: new or mixed-ending index must be written LF; got %q", p, got)
		}
	}
}

// A curator-authored subdirectory description in a CRLF index survives a
// rebuild intact: the trailing CR must not defeat the shape-suffix strip, which
// would capture the old suffix into the description and double it.
func TestWriteIndex_CRLFKeepsSubdirDescriptionIdempotent(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	_ = buildAndReload(t, dir)

	root := filepath.Join(dir, "index.md")
	withDesc := strings.Replace(readFile(t, root), "](wine/)", "](wine/) - Grapes and more.", 1)
	if err := os.WriteFile(root, []byte(withDesc), 0o644); err != nil {
		t.Fatal(err)
	}
	toCRLF(t, root)

	b, _ := Load(dir)
	if ok, diff := IndexInSync(b); !ok {
		t.Fatalf("CRLF index with a curator description must be in sync; got: %s", diff)
	}
	b = buildAndReload(t, dir)
	got := readFile(t, root)
	if strings.Count(got, "(1 concept") != 1 {
		t.Errorf("shape suffix must appear exactly once after rebuilding a CRLF index; got %q", got)
	}
	if !strings.Contains(got, " - Grapes and more. (1 concept") {
		t.Errorf("curator description must be preserved; got %q", got)
	}
	if ok, diff := IndexInSync(b); !ok {
		t.Errorf("index must be in sync after rebuild; got: %s", diff)
	}
}
