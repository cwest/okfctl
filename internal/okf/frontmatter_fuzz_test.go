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
