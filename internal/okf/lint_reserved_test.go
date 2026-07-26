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

import "testing"

// A knowledge base commonly keeps a per-neighborhood index.md and log.md
// (security/auth/index.md, wine/log.md, ...). These are reserved files at ANY
// depth, not concept nodes: they must never load as concept Nodes, never be
// reported as orphans, and never be candidate targets for missing-xref. Only
// the bundle-root index.md/log.md were recognized before, so nested ones leaked
// in as concept nodes titled "index"/"log" and produced 25 orphan findings plus
// 129 nonsense missing-xref findings ("mentions \"index\" but does not link to
// security/authz/index.md") against the real corpus.

func TestLoad_NestedReservedNotConceptNodes(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":               "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		"log.md":                 "---\ntype: Log\ntitle: Change Log\n---\n\n# Change Log\n",
		"wine/index.md":          "---\ntype: Index\ntitle: Wine\n---\n\n# Wine\n\n- [A](a.md)\n",
		"wine/log.md":            "---\ntype: Log\ntitle: Wine Log\n---\n\n# Wine Log\n",
		"wine/a.md":              lintDoc("Concept", "A", "Body."),
		"security/auth/index.md": "---\ntype: Index\ntitle: Auth\n---\n\n# Auth\n",
		"security/auth/log.md":   "---\ntype: Log\ntitle: Auth Log\n---\n\n# Auth Log\n",
	})
	for _, nested := range []string{"wine/index.md", "wine/log.md", "security/auth/index.md", "security/auth/log.md"} {
		if _, ok := b.Nodes[nested]; ok {
			t.Errorf("nested reserved file %s must not be a concept node", nested)
		}
		if _, ok := b.Reserved[nested]; !ok {
			t.Errorf("nested reserved file %s must be in b.Reserved", nested)
		}
	}
	if _, ok := b.Nodes["wine/a.md"]; !ok {
		t.Errorf("concept node wine/a.md must still load as a node")
	}
}

func TestLint_NestedReservedNeverOrphan(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":      "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		"wine/index.md": "---\ntype: Index\ntitle: Wine\n---\n\n# Wine\n",
		"wine/log.md":   "---\ntype: Log\ntitle: Wine Log\n---\n\n# Wine Log\n",
		"wine/a.md":     lintDoc("Concept", "A", "Body."),
	})
	for _, o := range findingsFor(Lint(b, LintOptions{}), "orphan") {
		if o.Path == "wine/index.md" || o.Path == "wine/log.md" {
			t.Fatalf("nested reserved file reported as orphan: %+v", o)
		}
	}
}

func TestLint_NestedReservedNeverMissingXrefTarget(t *testing.T) {
	// A concept node whose body contains the bare words "index" and "log" must
	// not produce a missing-xref finding pointing at a reserved index.md/log.md.
	b := mkLintBundle(t, map[string]string{
		"index.md":      "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		"wine/index.md": "---\ntype: Index\ntitle: Wine\n---\n\n# Wine\n\n- [A](a.md)\n",
		"wine/log.md":   "---\ntype: Log\ntitle: Wine Log\n---\n\n# Wine Log\n",
		"wine/a.md":     lintDoc("Concept", "A", "We keep an index and a log of every change."),
	})
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		t.Fatalf("bare word index/log must not trigger a missing-xref against a reserved file: %+v", f)
	}
}
