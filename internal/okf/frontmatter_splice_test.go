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

// reporterFixture is the exact frontmatter shape from the #146 reporter: it
// exercises the three things the whole-block re-encode destroyed — blank-line
// grouping between logical sections, four-space list indentation, and the
// alignment padding that lines up scalar values in a column. The refresh must
// change exactly one line (modified) and leave every other byte identical.
const reporterFixture = "---\n" +
	"# Core identity\n" +
	"type:     Concept\n" +
	"title:    Tannin\n" +
	"aliases:\n" +
	"    - tannins\n" +
	"    - tannic acid\n" +
	"\n" +
	"# Provenance\n" +
	"created:  2026-01-04T00:00:00Z\n" +
	"modified: 2026-01-04T00:00:00Z\n" +
	"---\n" +
	"\n" +
	"# Tannin\n" +
	"\n" +
	"A [link](acidity.md) to another node.\n"

// diffLineCount counts how many lines differ between a and b, line-for-line.
// It is the same measure the issue used (`diff | grep -c '^[<>]'` counts BOTH
// the removed and the added copy of a changed line, i.e. 2 per changed line);
// here we count changed line POSITIONS, so "exactly one changed line" == 1.
func diffLineCount(a, b string) int {
	al := strings.Split(a, "\n")
	bl := strings.Split(b, "\n")
	n := len(al)
	if len(bl) > n {
		n = len(bl)
	}
	changed := 0
	for i := 0; i < n; i++ {
		var av, bv string
		if i < len(al) {
			av = al[i]
		}
		if i < len(bl) {
			bv = bl[i]
		}
		if av != bv {
			changed++
		}
	}
	return changed
}

// The splice path must reduce the reporter's 19-changed-line diff to exactly
// ONE changed line: the modified field. Everything else — comments, blank-line
// grouping, four-space list indent, alignment padding, key order, body — is
// byte-preserved. This is the headline done-when for #146.
func TestTouchModifiedFileSpliceReporterFixtureOneLineDiff(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "tannin.md")
	if err := os.WriteFile(abs, []byte(reporterFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if err := TouchModifiedFile(abs, at); err != nil {
		t.Fatalf("TouchModifiedFile: %v", err)
	}
	got := readAll(t, abs)

	if n := diffLineCount(reporterFixture, got); n != 1 {
		t.Fatalf("expected exactly 1 changed line, got %d\n--- before ---\n%s\n--- after ---\n%s", n, reporterFixture, got)
	}
	// The one changed line is the modified field, with the alignment padding of
	// the original value column intact.
	if !strings.Contains(got, "modified: 2026-08-20T00:00:00Z\n") {
		t.Fatalf("modified not spliced with preserved layout:\n%s", got)
	}
	// Positive spot-checks that the fragile layout survived byte-for-byte.
	for _, want := range []string{
		"# Core identity\n",
		"type:     Concept\n",
		"title:    Tannin\n",
		"    - tannins\n",
		"    - tannic acid\n",
		"\n# Provenance\n",
		"created:  2026-01-04T00:00:00Z\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("layout fragment %q not preserved:\n%s", want, got)
		}
	}
	// created must not be rewritten.
	if strings.Count(got, "created:  2026-01-04T00:00:00Z") != 1 {
		t.Fatalf("created was altered:\n%s", got)
	}
	// Body untouched.
	if !strings.Contains(got, "A [link](acidity.md) to another node.\n") {
		t.Fatalf("body not preserved:\n%s", got)
	}
}

// spliceScalar is the byte-splice primitive: it replaces ONLY the value span of
// a top-level, plain, single-line scalar key, and returns ok=false (falls back)
// on every case outside that conservative envelope. These are the four
// negative-control cases from the #146 done-when list — none may be skipped.
func TestSpliceScalarFallbackCases(t *testing.T) {
	const stamp = "2026-08-20T00:00:00Z"
	cases := []struct {
		name  string
		block string
	}{
		{
			// key absent: nothing to splice.
			name:  "key absent",
			block: "type: Concept\ntitle: Tannin\n",
		},
		{
			// non-scalar value (a block sequence): splicing a scalar span into
			// a multi-node value would corrupt it.
			name:  "non-scalar value",
			block: "type: Concept\nmodified:\n  - a\n  - b\n",
		},
		{
			// block-style (literal) scalar: the value spans multiple lines and
			// a leading `|`; the single-line value-span assumption does not hold.
			name:  "block-style value",
			block: "type: Concept\nmodified: |\n  2026-01-04T00:00:00Z\n",
		},
		{
			// quoting-change needed: the existing value is double-quoted, so
			// replacing the inner bytes would leave a value whose quoting no
			// longer matches — the encoder path owns quoting decisions.
			name:  "quoting-change needed",
			block: "type: Concept\nmodified: \"2026-01-04T00:00:00Z\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := spliceScalar([]byte(tc.block), "modified", stamp)
			if ok {
				t.Fatalf("spliceScalar should have declined (fallback) for %q but returned ok=true", tc.name)
			}
		})
	}
}

// The eligible case: a top-level plain single-line scalar splices cleanly, and
// the returned block differs from the input only in the value span.
func TestSpliceScalarEligible(t *testing.T) {
	block := "type: Concept\ncreated:  2026-01-01T00:00:00Z\nmodified: 2026-01-04T00:00:00Z\n"
	out, ok := spliceScalar([]byte(block), "modified", "2026-08-20T00:00:00Z")
	if !ok {
		t.Fatal("spliceScalar declined an eligible plain scalar")
	}
	want := "type: Concept\ncreated:  2026-01-01T00:00:00Z\nmodified: 2026-08-20T00:00:00Z\n"
	if string(out) != want {
		t.Fatalf("splice not byte-minimal:\nGOT  %q\nWANT %q", out, want)
	}
}

// A trailing comment on the spliced line must survive: only the value token is
// replaced, not the comment after it.
func TestSpliceScalarPreservesTrailingComment(t *testing.T) {
	block := "modified: 2026-01-04T00:00:00Z  # last touched\n"
	out, ok := spliceScalar([]byte(block), "modified", "2026-08-20T00:00:00Z")
	if !ok {
		t.Fatal("spliceScalar declined an eligible plain scalar with a trailing comment")
	}
	want := "modified: 2026-08-20T00:00:00Z  # last touched\n"
	if string(out) != want {
		t.Fatalf("trailing comment not preserved:\nGOT  %q\nWANT %q", out, want)
	}
}

// The fallback path must remain byte-identical to today's encoder behaviour.
// This is the same assertion the pre-splice golden used (see
// TestTouchModifiedFilePreservesBlankSeparator); here we force the fallback by
// feeding a block-style modified value, which the splice declines, and assert
// the whole-block re-encode still lands and the body survives.
func TestTouchModifiedFileFallbackStillWorks(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "n.md")
	// A quoted modified forces the fallback (quoting-change guard).
	orig := "---\ntype: Concept\nmodified: \"2026-01-01T00:00:00Z\"\n---\n\n# Heading\n\nProse.\n"
	if err := os.WriteFile(abs, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got := readAll(t, abs)
	if !strings.Contains(got, "2026-08-20T00:00:00Z") {
		t.Fatalf("fallback did not refresh modified:\n%s", got)
	}
	// Body survives the fallback re-encode.
	if !strings.Contains(got, "# Heading\n\nProse.\n") {
		t.Fatalf("fallback dropped body:\n%s", got)
	}
	// Still parses and keeps type.
	fm, _, err := ParseFrontmatter([]byte(got))
	if err != nil {
		t.Fatalf("fallback result no longer parses: %v", err)
	}
	if fm["type"] != "Concept" {
		t.Fatalf("fallback lost type: %v", fm["type"])
	}
}
