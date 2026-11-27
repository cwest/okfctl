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
	"strings"
	"testing"
)

func TestParseFrontmatter_ExtractsTypeAndBody(t *testing.T) {
	src := []byte("---\ntype: Reference\ntitle: Widgets\n---\n\n# Body\n\nText here.\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("ParseFrontmatter error: %v", err)
	}
	if fm["type"] != "Reference" {
		t.Errorf("type = %v, want Reference", fm["type"])
	}
	if fm["title"] != "Widgets" {
		t.Errorf("title = %v, want Widgets", fm["title"])
	}
	if want := "# Body"; !strings.Contains(body, want) {
		t.Errorf("body missing %q; got %q", want, body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	fm, _, err := ParseFrontmatter([]byte("# Just markdown\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
}

func TestParseFrontmatter_Malformed(t *testing.T) {
	_, _, err := ParseFrontmatter([]byte("---\ntype: [unterminated\n---\n"))
	if err == nil {
		t.Fatal("expected error on malformed YAML frontmatter, got nil")
	}
}

// A frontmatter block whose YAML parses to null — a bare `!` tag, a literal
// `null`, or an empty block — must yield an EMPTY, NON-NIL map, not a nil map
// with a nil error. yaml.Unmarshal of a null document into a map leaves the map
// nil and returns no error, so without a guard ParseFrontmatter would report
// success while handing back a nil map. Load stores that map directly, and a nil
// Frontmatter is the sentinel validate uses for "unparseable frontmatter"
// (validate.go) — so a null block would masquerade as a parse failure AND its
// body would be silently dropped. The success contract is a usable (non-nil) map
// and a preserved body; a node with no frontmatter fields is legal (§7 requires
// `type`, but that is a validate concern, not a parse-time crash).
func TestParseFrontmatter_NullBlockYieldsEmptyMap(t *testing.T) {
	cases := []string{
		"---\n!\n---\n",
		"---\nnull\n---\n",
		"---\n\n---\n\n# Body kept\n",
	}
	for _, src := range cases {
		fm, _, err := ParseFrontmatter([]byte(src))
		if err != nil {
			t.Errorf("ParseFrontmatter(%q) error = %v, want nil", src, err)
			continue
		}
		if fm == nil {
			t.Errorf("ParseFrontmatter(%q) returned a nil map; success must yield a non-nil map", src)
		}
		if len(fm) != 0 {
			t.Errorf("ParseFrontmatter(%q) map = %v, want empty", src, fm)
		}
	}
	// The body after a null block must survive, not be dropped.
	_, body, err := ParseFrontmatter([]byte("---\nnull\n---\n\n# Body kept\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "# Body kept") {
		t.Errorf("body dropped after a null frontmatter block; got %q", body)
	}
}

func TestParseFrontmatter_BodyLineStartingWithDashes(t *testing.T) {
	// A body line that starts with --- must NOT be treated as the closing fence.
	src := []byte("---\ntype: Reference\n---\n\nText.\n\n---\n\nMore text after a horizontal rule.\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("ParseFrontmatter error: %v", err)
	}
	if fm["type"] != "Reference" {
		t.Errorf("type = %v, want Reference", fm["type"])
	}
	if !strings.Contains(body, "More text after a horizontal rule.") {
		t.Errorf("body lost content after an in-body --- rule; got %q", body)
	}
}

// TestParseFrontmatter_BareCRIsALineBreak pins YAML 1.2 §5.4: CR, LF and CRLF
// are each one line break. `0\r\r\n 0` is two breaks (CR, then CRLF), so the
// plain scalar spans an empty line and folds to "0\n0" (§6.5). The splitter
// must hand YAML the same breaks the source has, so the parsed value agrees with
// a parse of the verbatim block (the splice writer's view of the same file).
func TestParseFrontmatter_BareCRIsALineBreak(t *testing.T) {
	cases := []struct {
		name, src string
		want      any
	}{
		{"CR before CRLF", "---\n0: 0\r\r\n 0\n---\n", "0\n0"},
		{"CRLF blank line", "---\na: x\r\n\r\n y\n---\n", "x\ny"},
		{"bare CR after LF", "---\na: x\n\r y\n---\n", "x\ny"},
		{"CRLF only folds", "---\r\na: x\r\n y\r\n---\r\n", "x y"},
		{"bare CR between keys", "---\na: x\rb: 2\n---\n", "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, _, err := ParseFrontmatter([]byte(tc.src))
			if err != nil {
				t.Fatalf("ParseFrontmatter: %v", err)
			}
			key := "a"
			if _, ok := fm["0"]; ok {
				key = "0"
			}
			if fm[key] != tc.want {
				t.Fatalf("%s = %q, want %q", key, fm[key], tc.want)
			}
			block, _, ok := splitFrontmatterRaw([]byte(tc.src))
			if !ok {
				t.Fatal("splitFrontmatterRaw: no block")
			}
			fm2, _, err := ParseFrontmatter(append(append([]byte("---\n"), block...), "---\n"...))
			if err != nil || fm2[key] != tc.want {
				t.Fatalf("splitFrontmatterRaw block: %s = %q (err %v), want %q", key, fm2[key], err, tc.want)
			}
		})
	}
}
