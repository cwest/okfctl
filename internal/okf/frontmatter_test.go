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

import "testing"

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
	if want := "# Body"; !contains(body, want) {
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

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
