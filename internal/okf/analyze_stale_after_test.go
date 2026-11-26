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
	"reflect"
	"testing"
)

// saDoc builds a concept node with an explicit `modified` and a raw
// `stale_after` frontmatter line (omitted when saLine is empty), so a test can
// pit the age heuristic against the §5.5 lifecycle field.
func saDoc(title, modified, saLine string) string {
	fm := "---\ntype: Concept\ntitle: " + title + "\n"
	if modified != "" {
		fm += "modified: " + modified + "\n"
	}
	if saLine != "" {
		fm += saLine + "\n"
	}
	return fm + "---\n\n# " + title + "\n\nBody.\n"
}

func staleAfterPaths(rep AnalyzeReport) []string {
	out := []string{}
	for _, s := range rep.Freshness.StaleAfter {
		out = append(out, s.Path)
	}
	return out
}

func agingPaths(rep AnalyzeReport) []string {
	out := []string{}
	for _, a := range rep.Freshness.Aging {
		out = append(out, a.Path)
	}
	return out
}

func analyzeOne(t *testing.T, doc string) AnalyzeReport {
	t.Helper()
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [N](n.md)\n",
		"n.md":     doc,
	})
	return Analyze(b, DefaultAnalyzeOptions())
}

// Positive control (§5.5): a concept whose stale_after has passed is stale,
// even though its `modified` is recent and the age heuristic stays silent.
func TestAnalyze_Stale_StaleAfterPassed_Section5_5(t *testing.T) {
	pinClock(t, fixedNow) // 2026-07-26
	rep := analyzeOne(t, saDoc("N", "2026-07-20", "stale_after: 2026-07-01"))
	if got := staleAfterPaths(rep); !reflect.DeepEqual(got, []string{"n.md"}) {
		t.Fatalf("want n.md stale via stale_after, got %+v", rep.Freshness.StaleAfter)
	}
	s := rep.Freshness.StaleAfter[0]
	if s.StaleAfter != "2026-07-01" || s.Malformed {
		t.Fatalf("want stale_after 2026-07-01, not malformed; got %+v", s)
	}
	if got := agingPaths(rep); len(got) != 0 {
		t.Fatalf("recently modified node must not be aging, got %v", got)
	}
}

// §5.5: "stale when now >= stale_after"—the exact instant is stale.
func TestAnalyze_Stale_ExactInstantIsStale_Section5_5(t *testing.T) {
	pinClock(t, fixedNow) // 2026-07-26T12:00:00Z
	rep := analyzeOne(t, saDoc("N", "2026-07-20", `stale_after: "2026-07-26T12:00:00Z"`))
	if got := staleAfterPaths(rep); !reflect.DeepEqual(got, []string{"n.md"}) {
		t.Fatalf("want n.md stale at now == stale_after, got %+v", rep.Freshness.StaleAfter)
	}
}

// Negative control (§11.2 "SHOULD calculate staleness only from stale_after"):
// a node whose basis date is years old but whose stale_after is in the future
// is NOT stale. The age heuristic still lists it as aging (an advisory).
func TestAnalyze_Stale_OldNodeWithFutureStaleAfterIsNotStale_Section11_2(t *testing.T) {
	pinClock(t, fixedNow)
	rep := analyzeOne(t, saDoc("N", "2024-01-01", "stale_after: 2027-01-01"))
	if got := staleAfterPaths(rep); len(got) != 0 {
		t.Fatalf("future stale_after must not be stale regardless of age, got %+v", rep.Freshness.StaleAfter)
	}
	if got := agingPaths(rep); !reflect.DeepEqual(got, []string{"n.md"}) {
		t.Fatalf("want n.md in the aging advisory, got %v", got)
	}
}

// Negative control (§11.2): with no stale_after, age alone never makes a node
// stale—it only surfaces in the aging advisory.
func TestAnalyze_Stale_AgeAloneIsNotStale_Section11_2(t *testing.T) {
	pinClock(t, fixedNow)
	rep := analyzeOne(t, saDoc("N", "2024-01-01", ""))
	if got := staleAfterPaths(rep); len(got) != 0 {
		t.Fatalf("no stale_after => never stale, got %+v", rep.Freshness.StaleAfter)
	}
	if got := agingPaths(rep); !reflect.DeepEqual(got, []string{"n.md"}) {
		t.Fatalf("want n.md in the aging advisory, got %v", got)
	}
}

// §5.5: "A consumer MUST NOT treat a stale_after value that it cannot parse as
// fresh. It SHOULD show the value as malformed." Unparseable values (prose, a
// no-offset date-time, which §5 forbids, an impossible date) land in the stale
// list flagged malformed, with the raw value preserved for the curator.
func TestAnalyze_Stale_MalformedIsNeverFresh_Section5_5(t *testing.T) {
	pinClock(t, fixedNow)
	for _, c := range []struct{ line, raw string }{
		{"stale_after: next quarter", "next quarter"},
		{`stale_after: "2099-01-01T00:00:00"`, "2099-01-01T00:00:00"},
		{`stale_after: "2099-13-45"`, "2099-13-45"},
	} {
		rep := analyzeOne(t, saDoc("N", "2026-07-20", c.line))
		if got := staleAfterPaths(rep); !reflect.DeepEqual(got, []string{"n.md"}) {
			t.Fatalf("%s: malformed stale_after must not read as fresh, got %+v", c.line, rep.Freshness.StaleAfter)
		}
		s := rep.Freshness.StaleAfter[0]
		if !s.Malformed || s.StaleAfter != c.raw {
			t.Fatalf("%s: want malformed=true raw=%q, got %+v", c.line, c.raw, s)
		}
	}
}

// Negative control (§5.5, §11.2 rule 1): an empty or null stale_after is the
// same as an absent key—not malformed, not stale.
func TestAnalyze_Stale_EmptyOrNullIsAbsent_Section11_2(t *testing.T) {
	pinClock(t, fixedNow)
	for _, line := range []string{`stale_after: ""`, "stale_after: null", "stale_after:", "stale_after: []"} {
		rep := analyzeOne(t, saDoc("N", "2026-07-20", line))
		if got := staleAfterPaths(rep); len(got) != 0 {
			t.Fatalf("%q: empty/null stale_after is absent, want no stale finding, got %+v", line, rep.Freshness.StaleAfter)
		}
	}
}

// The old `freshness.stale` and `summary.stale_threshold_days` JSON keys keep
// their prior (age-heuristic) meaning for one release as deprecated aliases of
// `freshness.aging` / `summary.aging_threshold_days`, so an existing --json
// consumer does not silently start reading a different list.
func TestAnalyze_DeprecatedStaleAliasMirrorsAging(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [O](o.md)\n- [S](s.md)\n",
		"o.md":     saDoc("O", "2024-01-01", ""),
		"s.md":     saDoc("S", "2026-07-20", "stale_after: 2026-07-01"),
	})
	rep := Analyze(b, AnalyzeOptions{AgingDays: 90})
	if !reflect.DeepEqual(rep.Freshness.Stale, rep.Freshness.Aging) {
		t.Fatalf("deprecated freshness.stale must mirror aging:\n stale=%+v\n aging=%+v", rep.Freshness.Stale, rep.Freshness.Aging)
	}
	if got := agingPaths(rep); !reflect.DeepEqual(got, []string{"o.md"}) {
		t.Fatalf("want only o.md aging, got %v", got)
	}
	if rep.Summary.AgingThresholdDays != 90 || rep.Summary.StaleThresholdDays != 90 {
		t.Fatalf("want aging/stale threshold 90/90, got %+v", rep.Summary)
	}
}
