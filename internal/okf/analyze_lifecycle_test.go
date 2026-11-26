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

// analyze surfaces the §5.4 lifecycle: the raw status distribution (§11.2: show
// the raw value) and the concepts a current-knowledge filter excludes. It is
// observational, like the epistemic distribution: it never gates.
func TestAnalyze_Lifecycle_UnknownStatusIsNotCurrent_Section5_4(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "# Index\n",
		"a.md":     fmDoc(""),                                           // absent ⇒ stable
		"b.md":     fmDoc("status: stable\n"),                           // stable
		"c.md":     fmDoc("status: draft\n"),                            // known, not current
		"d.md":     fmDoc("status: deprecated\nsuperseded_by: /a.md\n"), // known, not current
		"e.md":     fmDoc("status: frobnicated\n"),                      // unknown ⇒ not stable
	})
	rep := Analyze(b, AnalyzeOptions{})
	dist := map[string]int{}
	for _, s := range rep.Lifecycle.Distribution {
		dist[s.Value] = s.Count
	}
	wantDist := map[string]int{"stable": 2, "draft": 1, "deprecated": 1, "frobnicated": 1}
	for k, v := range wantDist {
		if dist[k] != v {
			t.Errorf("distribution[%q] = %d, want %d (full: %v)", k, dist[k], v, dist)
		}
	}
	if len(rep.Lifecycle.NotCurrent) != 3 {
		t.Fatalf("NotCurrent = %+v, want c.md, d.md, e.md", rep.Lifecycle.NotCurrent)
	}
	byPath := map[string]LifecycleNode{}
	for _, n := range rep.Lifecycle.NotCurrent {
		byPath[n.Path] = n
	}
	if n := byPath["e.md"]; n.Status != "frobnicated" || n.Known {
		t.Errorf("e.md = %+v, want raw frobnicated, known=false", n)
	}
	if n := byPath["d.md"]; n.SupersededBy != "/a.md" || !n.Known {
		t.Errorf("d.md = %+v, want superseded_by /a.md, known=true", n)
	}
	if _, ok := byPath["a.md"]; ok {
		t.Error("a.md (absent status ⇒ stable) must not be NotCurrent")
	}
}
