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
	"testing"
)

// FuzzLoadBundle drives the whole node/Markdown parse path — frontmatter split,
// body extraction, the §5.1 inline-link scan (mdLinkRe), and edge building — over
// an untrusted node body. A bundle's .md files are author-supplied and, for a
// remote/imported bundle, attacker-influenced; a malformed body must never panic
// Load, and link resolution must never emit a dangling edge (an OutboundLinks
// target that is not a real node). One node's body is the fuzzed bytes; a second,
// fixed node exists so the link scanner has a real resolution target to find.
//
// The seed corpus is real link shapes: an in-bundle relative link, a root-
// absolute (§5.1) link, an image (must be skipped), an external link (skipped),
// a link with a CommonMark title, and a broken link — plus frontmatter edge
// cases so the two parsers are exercised together the way Load runs them.
func FuzzLoadBundle(f *testing.F) {
	seeds := []string{
		"---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nSee [acidity](wine/acidity.md) and [abs](/wine/acidity.md).\n",
		"---\ntype: Concept\n---\n\n![alt](img.png) [ext](https://example.com) [anchor](#top)\n",
		"# No frontmatter\n\n[titled](wine/acidity.md \"A Title\") [broken](nope/missing.md)\n",
		"---\nnull\n---\n\n[dir](acidity.md)\n",
		"[]() [ ]( ) [x](  )\n",
		"---\ntype: [unterminated\n---\n\nbody\n",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, body string) {
		dir := t.TempDir()
		// A fixed sibling node the link scanner can resolve against, so a fuzzed
		// link like [x](wine/acidity.md) actually becomes an edge and the
		// dangling-edge invariant is exercised, not vacuously satisfied.
		mustWrite(t, filepath.Join(dir, "wine", "acidity.md"),
			"---\ntype: Concept\ntitle: Acidity\n---\n\n# Acidity\n\nTart.\n")
		// The node under test carries the fuzzed body verbatim.
		mustWrite(t, filepath.Join(dir, "wine", "tannin.md"), body)

		b, err := Load(dir)
		if err != nil {
			// Load only errors on a filesystem walk failure, which a fixed temp
			// dir does not produce; a malformed body is preserved as a node, not
			// an error. If Load ever errors here it is a real regression.
			t.Fatalf("Load(%q) returned error on a well-formed temp dir: %v", dir, err)
		}

		// Invariant: every derived edge points at a node that actually exists.
		// A link resolver that emitted a target outside b.Nodes would corrupt the
		// graph every consumer (lint, graph, analyze) walks.
		for _, src := range []string{"wine/tannin.md", "wine/acidity.md"} {
			for _, dst := range b.OutboundLinks(src) {
				if b.Nodes[dst] == nil {
					t.Fatalf("OutboundLinks(%q) contains dangling target %q (not in Nodes)", src, dst)
				}
			}
		}
	})
}

// mustWrite writes content to path, creating parent dirs, failing the test on
// any error. It is the fuzz harness's bundle-file writer.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
