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
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

// FuzzParseFrontmatter drives the YAML-frontmatter splitter with untrusted bytes.
// A .md file's leading `---` block (§7 typed frontmatter) is author-supplied and,
// for a remote bundle, attacker-influenced; a malformed block must be a returned
// error, never a panic. The seed corpus is the exact set of shapes the
// example-based tests in frontmatter_test.go already exercise, plus the byte
// edge cases (empty, bare fence, CRLF) that only a fuzzer thinks to combine.
func FuzzParseFrontmatter(f *testing.F) {
	seeds := [][]byte{
		[]byte("---\ntype: Reference\ntitle: Widgets\n---\n\n# Body\n\nText here.\n"),
		[]byte("# Just markdown\n"),
		[]byte("---\ntype: [unterminated\n---\n"),
		[]byte("---\ntype: Reference\n---\n\nText.\n\n---\n\nMore text after a rule.\n"),
		[]byte("---\r\ntype: Reference\r\n---\r\n\r\n# CRLF body\r\n"),
		[]byte("---\nokf_version: \"0.2\"\n---\n\n# Index\n"),
		[]byte(""),
		[]byte("---\n"),
		[]byte("---\n---\n"),
		[]byte("---\n\x00\x00\n---\nbody"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fm, body, err := ParseFrontmatter(data)

		if err != nil {
			// The malformed-YAML contract (frontmatter.go): on error the map is
			// nil and the body is empty — a caller must never see a half-parsed
			// document masquerading as valid.
			if fm != nil {
				t.Fatalf("ParseFrontmatter error but non-nil frontmatter map: %v", fm)
			}
			if body != "" {
				t.Fatalf("ParseFrontmatter error but non-empty body: %q", body)
			}
			return
		}

		// Success contract: the map is always non-nil (callers range over it).
		if fm == nil {
			t.Fatal("ParseFrontmatter returned nil map with nil error")
		}

		// No opening fence => there is no frontmatter, so the body is the input
		// verbatim and the map is empty. This is the branch splitFrontmatter
		// short-circuits (frontmatter.go), and it must be byte-exact: a splitter
		// that silently drops or rewrites body bytes here would corrupt every
		// frontmatter-less node.
		hasFence := bytes.HasPrefix(data, []byte("---\n")) || bytes.HasPrefix(data, []byte("---\r\n"))
		if !hasFence {
			if body != string(data) {
				t.Fatalf("no-fence input: body != input\n input: %q\n  body: %q", data, body)
			}
			if len(fm) != 0 {
				t.Fatalf("no-fence input yielded non-empty frontmatter: %v", fm)
			}
		}
	})
}

// FuzzSplitFrontmatterRaw asserts the byte-preserving variant round-trips: the
// yaml block and the verbatim rawAfter, taken together, must reconstruct exactly
// the bytes that follow the opening fence line. This is the invariant an in-place
// frontmatter rewriter relies on to reproduce a file byte-for-byte outside the
// one field it changes (frontmatter.go splitFrontmatterRaw doc), so a fuzzer that
// breaks it catches a silent corruption before a `node` mutation ever ships it.
func FuzzSplitFrontmatterRaw(f *testing.F) {
	seeds := []string{
		"---\ntype: Reference\n---\n\n# Body\n",
		"---\r\ntype: Reference\r\n---\r\nbody\r\n",
		"no frontmatter here\n",
		"---\nunclosed: block\n",
		"---\nk: v\n---\n",
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		yamlBlock, rawAfter, ok := splitFrontmatterRaw(data)
		if !ok {
			return // no frontmatter block: nothing to reconstruct.
		}
		// The opening fence is the first line; everything after it is
		// yamlBlock's source lines, the closing `---\n`, then rawAfter. We can't
		// reconstruct the pre-trim yaml bytes (CR stripping is lossy by design),
		// but rawAfter must be a suffix of the original input: it is a subslice
		// taken verbatim, never rewritten.
		if !bytes.HasSuffix(data, rawAfter) {
			t.Fatalf("rawAfter is not a verbatim suffix of the input\n input: %q\n after: %q", data, rawAfter)
		}
		// yamlBlock is always newline-terminated per line (it is rebuilt with an
		// explicit '\n' per source line); an empty block is legal.
		if len(yamlBlock) > 0 && yamlBlock[len(yamlBlock)-1] != '\n' {
			t.Fatalf("yamlBlock not newline-terminated: %q", yamlBlock)
		}
	})
}

// FuzzSpliceScalar drives the byte-splice primitive with arbitrary frontmatter
// blocks. The contract under fuzz is a safety one, not a success one: spliceScalar
// MAY decline (ok=false) any block it deems ineligible, but whenever it accepts
// (ok=true) the result MUST (1) still parse as YAML, and (2) carry the intended
// value for the target key. A splice that produces unparseable YAML, or that
// sets the wrong value, is a corruption the whole conservative eligibility gate
// exists to prevent — exactly the failure a fuzzer over frontmatter shapes finds
// that fixtures cannot. The target key ("modified") and new value are fixed; the
// block is the fuzzed input.
func FuzzSpliceScalar(f *testing.F) {
	seeds := []string{
		"type: Concept\nmodified: 2026-01-04T00:00:00Z\n",
		"modified: 2026-01-04T00:00:00Z  # comment\n",
		"a:   1\nmodified:   old\nb: 2\n",
		"modified: \"quoted\"\n",
		"modified:\n  - a\n  - b\n",
		"modified: |\n  block\n",
		"no_target: here\n",
		"",
		"modified: 2026-01-04T00:00:00Z\r\n",
		"nested:\n  modified: notatoplevelkey\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	const newVal = "2026-08-20T00:00:00Z"
	f.Fuzz(func(t *testing.T, block []byte) {
		// The invariant is a RELATIVE one: whatever a map-decode of the ORIGINAL
		// block yields, the spliced block must decode the same way with only the
		// modified value changed. An input that itself fails a map decode (e.g. a
		// duplicate top-level key, which yaml rejects into a map but tolerates
		// into a Node) is out of scope — the splice cannot be blamed for a defect
		// already present in the input. So establish the input's baseline first.
		before := map[string]any{}
		if err := yaml.Unmarshal(block, &before); err != nil {
			return // input isn't a clean mapping; not a splice concern.
		}

		out, ok := spliceScalar(block, "modified", newVal)
		if !ok {
			return // declined: fallback owns this shape, nothing to assert.
		}
		// Accepted: the spliced block must still be a parseable YAML mapping and
		// the intended value must be set.
		after := map[string]any{}
		if err := yaml.Unmarshal(out, &after); err != nil {
			t.Fatalf("spliced block no longer parses: %v\n block: %q\n   out: %q", err, block, out)
		}
		got, present := after["modified"]
		if !present {
			t.Fatalf("spliced block dropped the target key\n block: %q\n   out: %q", block, out)
		}
		// The decoded value must equal the intended new value (string-compared;
		// yaml may decode a timestamp-shaped scalar to time.Time, so compare on
		// the round-tripped string form).
		if s, isStr := got.(string); isStr && s != newVal {
			t.Fatalf("spliced value wrong: got %q want %q\n block: %q\n   out: %q", s, newVal, block, out)
		}
		// Every OTHER top-level key must be untouched by the splice.
		if len(after) != len(before) {
			t.Fatalf("splice changed the key set: before=%d after=%d\n block: %q\n   out: %q", len(before), len(after), block, out)
		}
	})
}
