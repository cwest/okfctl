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
	"regexp"
	"strings"
	"testing"
)

// A `[k]` that opens a line (up to three spaces in) and is followed by `:`
// would become `[^k]:`, which GFM reads as a footnote DEFINITION, not a
// reference (§4.2), inside a block quote or list item too. The node would get
// a second, empty definition for k. Such a reference is held back as
// citation-ref-unclear and left as written.
func TestConformance_MigrateCitationRefs_Section4_2_LineInitialColonIsNotRewritten(t *testing.T) {
	for _, body := range []string{
		"Count the commits on the default branch\n[1]:\n\n```\ngit log\n```",
		"Count the commits on the default branch\n   [1]:\n\n```\ngit log\n```",
		"[2]:\n\n    indented code",
		"> Quoted claim\n> [1]:\n\nAfter.",
		"- A claim\n- [1]:\n\nAfter.",
	} {
		out, unclear := planRefs(t, body)
		if len(unclear) != 1 {
			t.Errorf("%q: want one citation-ref-unclear item, got %q", body, unclear)
		}
		if regexp.MustCompile(`(?m)^ {0,3}\[\^[12]\]:[ \t]*$`).MatchString(out) {
			t.Errorf("%q: migrate wrote an empty footnote definition:\n%s", body, out)
		}
		if !strings.Contains(out, body) {
			t.Errorf("%q: the reference must stay as written:\n%s", body, out)
		}
	}
}

// Negative controls: a `[k]:` in the middle of a line, or a line-initial `[k]`
// not followed by a colon, is an ordinary reference and is rewritten.
func TestConformance_MigrateCitationRefs_Section4_2_ColonMidLineStillRewritten(t *testing.T) {
	cases := map[string]string{
		"Count the commits [1]:\n\n```\ngit log\n```": "Count the commits [^1]:",
		"Measured locally\n[1] and confirmed.":        "Measured locally\n[^1] and confirmed.",
		"Measured locally\n[1][2]: both.":             "Measured locally\n[^1][^2]: both.",
		"`x` [1]: here.":                              "`x` [^1]: here.",
	}
	for body, want := range cases {
		out, unclear := planRefs(t, body)
		if len(unclear) != 0 || !strings.Contains(out, want) {
			t.Errorf("%q: want %q, unclear = %q\n%s", body, want, unclear, out)
		}
	}
}
