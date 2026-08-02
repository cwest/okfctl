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
	"regexp"
	"strings"
)

// headingRe matches an ATX markdown heading line: one to six '#' at the start of
// a line followed by whitespace. This is a deliberate line-scan, not a second
// markdown parser (cf. mdLinkRe in internal/okf/bundle.go) — okfctl's house
// style is a single regex over lines, and passage chunking does not need block
// structure beyond heading boundaries.
var headingRe = regexp.MustCompile(`^#{1,6}\s`)

// chunk is one heading-delimited section of a node body: the heading text (empty
// for preamble text before the first heading) and the raw section text including
// the heading line itself.
type chunk struct {
	Heading string // heading text with leading '#'s and surrounding whitespace stripped ("" for preamble)
	Text    string // the section's raw markdown, heading line included
}

// chunkByHeadings splits a node body into heading-delimited sections. Text before
// the first heading becomes a leading preamble chunk (Heading == ""); each ATX
// heading starts a new chunk carrying its heading line plus the body up to the
// next heading. A body with no headings yields a single preamble chunk. Chunks
// whose text is only whitespace are dropped so empty sections add no index noise.
func chunkByHeadings(body string) []chunk {
	lines := strings.Split(body, "\n")
	var chunks []chunk
	var cur chunk
	started := false // whether cur has accumulated any lines yet

	flush := func() {
		if started && strings.TrimSpace(cur.Text) != "" {
			cur.Text = strings.TrimRight(cur.Text, "\n")
			chunks = append(chunks, cur)
		}
	}

	for _, line := range lines {
		if headingRe.MatchString(line) {
			flush()
			cur = chunk{Heading: headingText(line), Text: line + "\n"}
			started = true
			continue
		}
		cur.Text += line + "\n"
		started = true
	}
	flush()
	return chunks
}

// headingText strips the leading '#'s and surrounding whitespace from an ATX
// heading line, returning just the heading's text.
func headingText(line string) string {
	return strings.TrimSpace(strings.TrimLeft(line, "#"))
}
