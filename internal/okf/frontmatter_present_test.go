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
	"testing"
)

// A present-but-empty frontmatter block (`---\n---`) parses to an empty map, the
// same shape as "no block at all". Load must record which one it saw so validate
// can tell the §8 "no frontmatter" (conformant) case apart from a present empty
// block (a §8 violation on a non-root index; a §12 violation on the root index).
func TestLoad_RecordsFrontmatterBlockPresence(t *testing.T) {
	dir := t.TempDir()
	// A non-root index with an EMPTY frontmatter block.
	writeFile(t, dir, "sub/index.md", "---\n---\n\n# Sub\n")
	// A non-root index with NO frontmatter block.
	writeFile(t, dir, "other/index.md", "# Other\n")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	empty := b.Reserved["sub/index.md"]
	if empty == nil {
		t.Fatal("sub/index.md not loaded as reserved")
	}
	if !empty.HasFrontmatterBlock {
		t.Errorf("sub/index.md has a present (empty) frontmatter block; HasFrontmatterBlock = false, want true")
	}
	if len(empty.Frontmatter) != 0 {
		t.Errorf("sub/index.md empty block should parse to an empty map; got %v", empty.Frontmatter)
	}

	none := b.Reserved["other/index.md"]
	if none == nil {
		t.Fatal("other/index.md not loaded as reserved")
	}
	if none.HasFrontmatterBlock {
		t.Errorf("other/index.md has no frontmatter block; HasFrontmatterBlock = true, want false")
	}
}
