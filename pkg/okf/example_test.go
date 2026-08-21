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

package okf_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cwest/okfctl/pkg/okf"
)

// writeExampleBundle writes a tiny two-node bundle to a temp dir and returns its
// root. The examples build a bundle rather than assume one on disk so their
// Output blocks are deterministic and pkg.go.dev can run them.
func writeExampleBundle() (root string, cleanup func()) {
	dir, err := os.MkdirTemp("", "okf-example-")
	if err != nil {
		log.Fatal(err)
	}
	files := map[string]string{
		"index.md":        "# Index\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\ntags: [wine]\n---\n\n# Tannin\n\nTannins bind proteins. See [Acidity](acidity.md).\n",
		"wine/acidity.md": "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\nMouthfeel and pH.\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			log.Fatal(err)
		}
	}
	return dir, func() { os.RemoveAll(dir) }
}

// Example shows the canonical read flow: load a bundle, then validate it against
// the OKF spec floor. An empty finding slice means the bundle conforms.
func Example() {
	dir, cleanup := writeExampleBundle()
	defer cleanup()

	bundle, err := okf.Load(dir)
	if err != nil {
		log.Fatal(err)
	}
	findings := okf.Validate(bundle)
	fmt.Printf("valid: %v\n", len(findings) == 0)
	// Output:
	// valid: true
}

// ExampleSearch runs a case-insensitive lexical query across all match surfaces.
func ExampleSearch() {
	dir, cleanup := writeExampleBundle()
	defer cleanup()

	bundle, err := okf.Load(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range okf.Search(bundle, "tannin", okf.FieldAny) {
		fmt.Println(r.Path)
	}
	// Output:
	// wine/tannin.md
}

// ExampleBuildGraph derives the serializable link graph and prints its shape.
func ExampleBuildGraph() {
	dir, cleanup := writeExampleBundle()
	defer cleanup()

	bundle, err := okf.Load(dir)
	if err != nil {
		log.Fatal(err)
	}
	g := okf.BuildGraph(bundle)
	fmt.Printf("nodes=%d edges=%d\n", len(g.Nodes), len(g.Edges))
	// Output:
	// nodes=2 edges=1
}

// ExampleLint runs the deterministic structural curation checks. Findings are
// guidance, not spec-floor failures.
func ExampleLint() {
	dir, cleanup := writeExampleBundle()
	defer cleanup()

	bundle, err := okf.Load(dir)
	if err != nil {
		log.Fatal(err)
	}
	findings := okf.Lint(bundle, okf.LintOptions{})
	fmt.Printf("lint findings: %d\n", len(findings))
	// Output:
	// lint findings: 0
}
