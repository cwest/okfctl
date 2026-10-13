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

// analyze adopts the shared tag fold, so three spellings of one tag over three
// nodes form a single cluster that reaches the default --cluster-min 3 — where
// today's case-only fold leaves them as three 1-node tags, all invisible.
func TestAnalyzeClusters_FoldedVariantsFormOneCluster(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"runbook"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"runbooks"}, "Body."),
		"c.md":     lintDocTags("Concept", "C", []string{"run-book"}, "Body."),
	})
	clusters := analyzeClusters(b, DefaultAnalyzeOptions())
	var runbook *ClusterFinding
	for i := range clusters {
		if canonTag(clusters[i].Tag) == canonTag("runbook") {
			runbook = &clusters[i]
			break
		}
	}
	if runbook == nil {
		t.Fatalf("expected a folded runbook cluster at cluster-min 3, got clusters=%+v", clusters)
	}
	if len(runbook.Nodes) != 3 {
		t.Fatalf("folded runbook cluster should span all 3 nodes, got %d: %+v", len(runbook.Nodes), runbook.Nodes)
	}
}

// Case folding already merged Wine/wine before this change; that behavior must
// survive the fold swap (regression guard for the analyze path).
func TestAnalyzeClusters_CaseVariantsStillMerge(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"Wine"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"wine"}, "Body."),
		"c.md":     lintDocTags("Concept", "C", []string{"WINE"}, "Body."),
	})
	clusters := analyzeClusters(b, DefaultAnalyzeOptions())
	found := false
	for _, c := range clusters {
		if canonTag(c.Tag) == canonTag("wine") && len(c.Nodes) == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("Wine/wine/WINE must still merge into one 3-node cluster, got %+v", clusters)
	}
}

// Distinct tags must not be merged by the fold (negative control for analyze).
func TestAnalyzeClusters_DistinctTagsNotMerged(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDocTags("Concept", "A", []string{"oauth1"}, "Body."),
		"b.md":     lintDocTags("Concept", "B", []string{"oauth2"}, "Body."),
		"c.md":     lintDocTags("Concept", "C", []string{"incident"}, "Body."),
	})
	clusters := analyzeClusters(b, DefaultAnalyzeOptions())
	for _, c := range clusters {
		if len(c.Nodes) >= 3 {
			t.Fatalf("distinct tags must not be folded into a cluster, got %+v", c)
		}
	}
}
