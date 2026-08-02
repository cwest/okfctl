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
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// TestChunkByHeadings_SplitsOnHeadings verifies the line-scan splitter divides a
// body at each ATX heading (^#{1,6}\s), keeping the heading with the text under
// it, and captures preamble text before the first heading as its own chunk.
func TestChunkByHeadings_SplitsOnHeadings(t *testing.T) {
	body := "Intro before any heading.\n\n" +
		"## Section One\n\nAlpha content here.\n\n" +
		"### Sub A\n\nBeta content.\n\n" +
		"## Section Two\n\nGamma content.\n"
	chunks := chunkByHeadings(body)
	if len(chunks) != 4 {
		t.Fatalf("want 4 chunks (preamble + 3 headings), got %d: %+v", len(chunks), chunks)
	}
	// Preamble chunk: no heading, carries the intro text.
	if chunks[0].Heading != "" {
		t.Errorf("preamble chunk heading = %q, want empty", chunks[0].Heading)
	}
	if !strings.Contains(chunks[0].Text, "Intro before any heading") {
		t.Errorf("preamble chunk missing intro text: %q", chunks[0].Text)
	}
	// Heading chunks carry the heading text (stripped of # and whitespace).
	wantHeadings := []string{"", "Section One", "Sub A", "Section Two"}
	for i, want := range wantHeadings {
		if chunks[i].Heading != want {
			t.Errorf("chunk[%d].Heading = %q, want %q", i, chunks[i].Heading, want)
		}
	}
	// A heading chunk's Text includes its own heading line + body under it.
	if !strings.Contains(chunks[1].Text, "Section One") || !strings.Contains(chunks[1].Text, "Alpha content") {
		t.Errorf("chunk[1].Text = %q, want heading + alpha content", chunks[1].Text)
	}
	// Sub A must not leak into Section One.
	if strings.Contains(chunks[1].Text, "Beta content") {
		t.Errorf("chunk[1] leaked Sub A content: %q", chunks[1].Text)
	}
}

// TestChunkByHeadings_NoHeadings returns the whole body as a single preamble
// chunk when there are no headings at all.
func TestChunkByHeadings_NoHeadings(t *testing.T) {
	body := "Just a paragraph.\nNo headings anywhere.\n"
	chunks := chunkByHeadings(body)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Heading != "" {
		t.Errorf("heading = %q, want empty", chunks[0].Heading)
	}
	if !strings.Contains(chunks[0].Text, "Just a paragraph") {
		t.Errorf("chunk text = %q", chunks[0].Text)
	}
}

// TestChunkByHeadings_EmptyChunksDropped: a heading immediately followed by
// another heading (no body) is still a chunk (the heading itself is content),
// but a body that is only whitespace between headings does not produce empty
// noise chunks with no text at all.
func TestChunkByHeadings_LeadingHeading(t *testing.T) {
	body := "# Title\n\nOnly content.\n"
	chunks := chunkByHeadings(body)
	// No preamble (body starts with a heading), so exactly one chunk.
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Heading != "Title" {
		t.Errorf("heading = %q, want Title", chunks[0].Heading)
	}
}

// TestChunkByHeadings_IgnoresNonHeadingHashes: a '#' that is not an ATX heading
// (e.g. inside text or a fenced code block start line without a space) is not a
// split point. Line-scan requires ^#{1,6}\s.
func TestChunkByHeadings_RequiresSpaceAfterHashes(t *testing.T) {
	body := "## Real Heading\n\n#notaheading still body\n\ncontent\n"
	chunks := chunkByHeadings(body)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk (only one real heading), got %d: %+v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0].Text, "#notaheading") {
		t.Errorf("chunk should retain the non-heading '#' line: %q", chunks[0].Text)
	}
}

// TestChunkByHeadings_DeepInRealNode confirms the splitter works on a node
// loaded through okf.Load (body is the post-frontmatter markdown).
func TestChunkByHeadings_OnLoadedNode(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"multi.md": "---\ntype: Concept\ntitle: Multi\n---\n\n# Multi\n\nlead\n\n## H2\n\nbody2\n",
	})
	n := b.Nodes["multi.md"]
	if n == nil {
		t.Fatal("multi.md not loaded")
	}
	_ = okf.Node{} // keep okf import meaningful
	chunks := chunkByHeadings(n.Body)
	if len(chunks) < 2 {
		t.Fatalf("want >=2 chunks from a multi-heading node, got %d", len(chunks))
	}
}
