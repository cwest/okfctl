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

// OKF §5.1: a "/"-absolute link resolves against the BUNDLE ROOT (the loaded
// bundle dir), not the linking node's directory. The real knowledge-base corpus
// writes every concept cross-link in this form (e.g. [x](/research/foo.md)), so
// the shared edge-builder MUST resolve it or graph/lint/analyze all see zero
// links and mis-report connectivity.
func TestLoad_RootAbsoluteLinkResolves(t *testing.T) {
	b := mkGraphBundle(t, map[string]string{
		"index.md":      "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](research/a.md)\n- [B](method/b.md)\n",
		"research/a.md": gnode("Concept", "A", "See [B](/method/b.md)."),
		"method/b.md":   gnode("Concept", "B", "Body of B."),
	})
	outs := b.OutboundLinks("research/a.md")
	found := false
	for _, o := range outs {
		if o == "method/b.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("root-absolute link /method/b.md should resolve to method/b.md; got outbound %v", outs)
	}
}

// A "/"-absolute link to a nonexistent target does NOT resolve (stays a
// dangling reference), same as the other link forms.
func TestLoad_RootAbsoluteLinkUnresolvedWhenMissing(t *testing.T) {
	b := mkGraphBundle(t, map[string]string{
		"index.md":      "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](research/a.md)\n",
		"research/a.md": gnode("Concept", "A", "See [gone](/method/gone.md)."),
	})
	if outs := b.OutboundLinks("research/a.md"); len(outs) != 0 {
		t.Fatalf("root-absolute link to missing target should not resolve; got %v", outs)
	}
}
