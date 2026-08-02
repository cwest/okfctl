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
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// fmFromYAML parses a YAML frontmatter block into the map[string]any shape Load
// produces, so provenance accessors are tested against exactly what the reader
// sees on disk.
func fmFromYAML(t *testing.T, src string) map[string]any {
	t.Helper()
	fm := map[string]any{}
	if err := yaml.Unmarshal([]byte(src), &fm); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", src, err)
	}
	return fm
}

// --- §5.1 sources ------------------------------------------------------------

func TestSources_ParsesEntriesAndCredibilitySignals_Section5_1(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, `
type: Metric
sources:
  - id: ga4-schema
    resource: https://developers.google.com/analytics/bigquery/export-schema
    title: GA4 BigQuery Export schema
    author: team:ga4-docs
    usage_count: 5000
    last_modified: 2026-05-30
usage_window: { from: 2026-06-01, to: 2026-06-30 }
`)}
	got := n.Sources()
	if len(got) != 1 {
		t.Fatalf("Sources() len = %d, want 1", len(got))
	}
	s := got[0]
	if s.ID != "ga4-schema" {
		t.Errorf("ID = %q, want ga4-schema", s.ID)
	}
	if s.Resource != "https://developers.google.com/analytics/bigquery/export-schema" {
		t.Errorf("Resource = %q", s.Resource)
	}
	if s.Title != "GA4 BigQuery Export schema" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Author != "team:ga4-docs" {
		t.Errorf("Author = %q, want team:ga4-docs", s.Author)
	}
	if s.UsageCount == nil || *s.UsageCount != 5000 {
		t.Errorf("UsageCount = %v, want 5000", s.UsageCount)
	}
	if s.LastModified != "2026-05-30" {
		t.Errorf("LastModified = %q, want 2026-05-30", s.LastModified)
	}
	// §5.1: the shared usage_window frames every entry.
	if s.UsageWindow == nil || s.UsageWindow.From != "2026-06-01" || s.UsageWindow.To != "2026-06-30" {
		t.Errorf("UsageWindow = %+v, want {2026-06-01 2026-06-30}", s.UsageWindow)
	}
}

func TestSources_EntryUsageWindowOverridesShared_Section5_1(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, `
sources:
  - resource: /tables/customers.md
    usage_window: { from: 2026-07-01, to: 2026-07-31 }
usage_window: { from: 2026-06-01, to: 2026-06-30 }
`)}
	got := n.Sources()
	if len(got) != 1 {
		t.Fatalf("Sources() len = %d, want 1", len(got))
	}
	// §5.1: "A single entry MAY carry its own usage_window to override the
	// shared one."
	if got[0].UsageWindow == nil || got[0].UsageWindow.From != "2026-07-01" || got[0].UsageWindow.To != "2026-07-31" {
		t.Errorf("entry UsageWindow = %+v, want the entry override {2026-07-01 2026-07-31}", got[0].UsageWindow)
	}
}

func TestSources_DropsEntryMissingRequiredResource_Section5_1(t *testing.T) {
	// §5.1: resource is REQUIRED within an entry. An entry without it is not a
	// well-formed source; the reader surfaces only well-formed entries.
	n := &Node{Frontmatter: fmFromYAML(t, `
sources:
  - id: no-resource
    title: dangling
  - resource: https://example.com/ok
`)}
	got := n.Sources()
	if len(got) != 1 {
		t.Fatalf("Sources() len = %d, want 1 (the resource-less entry dropped)", len(got))
	}
	if got[0].Resource != "https://example.com/ok" {
		t.Errorf("Resource = %q, want the well-formed entry", got[0].Resource)
	}
}

func TestSources_AbsentYieldsEmpty_Section5_1(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if got := n.Sources(); len(got) != 0 {
		t.Errorf("Sources() with no key = %v, want empty", got)
	}
}

// --- §5.2 generated + §13.1 timestamp fallback --------------------------------

func TestGenerated_ReadsByAndAt_Section5_2(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, `
generated: { by: reference_agent/gemini-2.5-pro, at: 2026-06-20T22:53:05Z }
`)}
	g, ok := n.Generated()
	if !ok {
		t.Fatal("Generated() ok = false, want true")
	}
	if g.By != "reference_agent/gemini-2.5-pro" {
		t.Errorf("By = %q", g.By)
	}
	want := time.Date(2026, 6, 20, 22, 53, 5, 0, time.UTC)
	if !g.At.Equal(want) {
		t.Errorf("At = %v, want %v", g.At, want)
	}
}

func TestGenerated_FallsBackToLegacyTimestamp_Section13_1(t *testing.T) {
	// §13.1: "Consumers MAY fall back to a legacy timestamp when generated is
	// absent." v0.1 recorded no author, so By is empty.
	n := &Node{Frontmatter: fmFromYAML(t, `
type: Metric
timestamp: '2026-05-28T22:53:05+00:00'
`)}
	g, ok := n.Generated()
	if !ok {
		t.Fatal("Generated() with legacy timestamp fallback ok = false, want true")
	}
	if g.By != "" {
		t.Errorf("By = %q, want empty (v0.1 timestamp carries no actor)", g.By)
	}
	want := time.Date(2026, 5, 28, 22, 53, 5, 0, time.UTC)
	if !g.At.Equal(want) {
		t.Errorf("At = %v, want %v (from legacy timestamp)", g.At, want)
	}
}

func TestGenerated_PrefersGeneratedOverTimestamp_Section13_1(t *testing.T) {
	// When both are present, generated wins — the fallback is only for its
	// absence.
	n := &Node{Frontmatter: fmFromYAML(t, `
generated: { by: agent/x, at: 2026-06-20T00:00:00Z }
timestamp: '2020-01-01T00:00:00Z'
`)}
	g, ok := n.Generated()
	if !ok || g.By != "agent/x" {
		t.Fatalf("Generated() = %+v, ok=%v; want generated block preferred", g, ok)
	}
}

func TestGenerated_AbsentYieldsNotOK_Section5_2(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if _, ok := n.Generated(); ok {
		t.Error("Generated() with neither generated nor timestamp ok = true, want false")
	}
}

// --- §5.2 verified + §11 bare-mapping MUST ------------------------------------

func TestVerified_ReadsList_Section5_2(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, `
verified:
  - { by: human:ahormati, at: 2026-06-25T09:00:00Z }
  - { by: process:finance-nightly, at: 2026-06-26T02:00:00Z }
`)}
	got := n.Verified()
	if len(got) != 2 {
		t.Fatalf("Verified() len = %d, want 2", len(got))
	}
	if got[0].By != "human:ahormati" || got[1].By != "process:finance-nightly" {
		t.Errorf("Verified() actors = %q, %q", got[0].By, got[1].By)
	}
	want := time.Date(2026, 6, 26, 2, 0, 0, 0, time.UTC)
	if !got[1].At.Equal(want) {
		t.Errorf("second At = %v, want %v", got[1].At, want)
	}
}

func TestVerified_BareMappingNormalizesToOneElementList_Section11(t *testing.T) {
	// §5.2/§11 MUST: "Consumers MUST treat a bare mapping as a one-element list."
	n := &Node{Frontmatter: fmFromYAML(t, `
verified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }
`)}
	got := n.Verified()
	if len(got) != 1 {
		t.Fatalf("bare-mapping Verified() len = %d, want 1 (§11 MUST)", len(got))
	}
	if got[0].By != "human:ahormati" {
		t.Errorf("By = %q, want human:ahormati", got[0].By)
	}
}

func TestVerified_AbsentYieldsEmpty_Section5_2(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if got := n.Verified(); len(got) != 0 {
		t.Errorf("Verified() with no key = %v, want empty", got)
	}
}

// --- §5.3 trust tiers (derived, never stored) --------------------------------

func TestTrustTier_NoVerifiedIsUnverified_Section5_3(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if got := n.TrustTier(); got != TrustUnverified {
		t.Errorf("TrustTier() = %q, want %q", got, TrustUnverified)
	}
}

func TestTrustTier_NonHumanOnlyIsMachineConfirmed_Section5_3(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, `
verified:
  - { by: process:finance-nightly, at: 2026-06-26T02:00:00Z }
  - { by: reference_agent/gemini-2.5-pro, at: 2026-06-26T03:00:00Z }
`)}
	if got := n.TrustTier(); got != TrustMachineConfirmed {
		t.Errorf("TrustTier() = %q, want %q", got, TrustMachineConfirmed)
	}
}

func TestTrustTier_AnyHumanIsHumanReviewed_Section5_3(t *testing.T) {
	// §5.3: any human:<id> verifier ⇒ human-reviewed, even mixed with machines.
	n := &Node{Frontmatter: fmFromYAML(t, `
verified:
  - { by: process:finance-nightly, at: 2026-06-26T02:00:00Z }
  - { by: human:ahormati, at: 2026-06-25T09:00:00Z }
`)}
	if got := n.TrustTier(); got != TrustHumanReviewed {
		t.Errorf("TrustTier() = %q, want %q", got, TrustHumanReviewed)
	}
}

func TestTrustTier_NotStoredInFrontmatter_Section5_3(t *testing.T) {
	// §5.3: trust tiers are DERIVED, never stored. Reading the tier must not
	// write a key back into the frontmatter map.
	fm := fmFromYAML(t, `
verified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }
`)
	n := &Node{Frontmatter: fm}
	_ = n.TrustTier()
	for _, k := range []string{"trust", "trust_tier", "tier"} {
		if _, present := fm[k]; present {
			t.Errorf("TrustTier() must not store %q back into frontmatter", k)
		}
	}
}

// --- §5.4 status --------------------------------------------------------------

func TestStatus_ReadsExplicit_Section5_4(t *testing.T) {
	for _, want := range []string{"draft", "stable", "deprecated"} {
		n := &Node{Frontmatter: fmFromYAML(t, "status: "+want+"\n")}
		if got := n.Status(); got != want {
			t.Errorf("Status() = %q, want %q", got, want)
		}
	}
}

func TestStatus_AbsentDefaultsToStable_Section5_4(t *testing.T) {
	// §5.4: "Absent status ⇒ stable."
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if got := n.Status(); got != "stable" {
		t.Errorf("Status() with no key = %q, want stable", got)
	}
}

// --- §5.5 stale_after ---------------------------------------------------------

func TestStaleAfter_StaleOnOrAfterDate_Section5_5(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "stale_after: 2026-09-23\n")}
	sa, ok := n.StaleAfter()
	if !ok {
		t.Fatal("StaleAfter() ok = false, want true")
	}
	if sa != (time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("StaleAfter() = %v, want 2026-09-23", sa)
	}
	// §5.5: stale when today >= stale_after.
	cases := []struct {
		day  time.Time
		want bool
	}{
		{time.Date(2026, 9, 22, 23, 0, 0, 0, time.UTC), false}, // day before
		{time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), true},   // exact day (>=)
		{time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), true},   // after
	}
	for _, c := range cases {
		if got := n.IsStale(c.day); got != c.want {
			t.Errorf("IsStale(%s) = %v, want %v", c.day.Format("2006-01-02"), got, c.want)
		}
	}
}

func TestStaleAfter_AbsentIsNeverStale_Section5_5(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n")}
	if _, ok := n.StaleAfter(); ok {
		t.Error("StaleAfter() with no key ok = true, want false")
	}
	if n.IsStale(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("IsStale() with no stale_after = true, want false")
	}
}

// --- §13.1 sources / # Citations fallback ------------------------------------

func TestSourceCitations_UsesFrontmatterSources_Section13_1(t *testing.T) {
	n := &Node{
		Frontmatter: fmFromYAML(t, `
sources:
  - resource: https://example.com/a
  - resource: https://example.com/b
`),
		Body: "# Definition\n\n# Citations\n\n[1] legacy body cite\n",
	}
	// §13.1: read frontmatter sources; when present, that is provenance.
	if got := n.SourceCitations(); got != 2 {
		t.Errorf("SourceCitations() = %d, want 2 (from frontmatter sources)", got)
	}
}

func TestSourceCitations_FallsBackToBodyCitationsForV01_Section13_1(t *testing.T) {
	// §13.1: "MAY still parse a legacy # Citations body list for v0.1 documents"
	// — no frontmatter sources, so fall back to the body list.
	n := &Node{
		Frontmatter: fmFromYAML(t, "type: Metric\n"),
		Body:        "# Definition\n\nBody.\n\n# Citations\n\n[1] one\n[2] two\n",
	}
	if got := n.SourceCitations(); got != 2 {
		t.Errorf("SourceCitations() = %d, want 2 (legacy body fallback)", got)
	}
}

func TestSourceCitations_NoneYieldsZero_Section13_1(t *testing.T) {
	n := &Node{Frontmatter: fmFromYAML(t, "type: Reference\n"), Body: "# Definition\n\nNo cites.\n"}
	if got := n.SourceCitations(); got != 0 {
		t.Errorf("SourceCitations() = %d, want 0", got)
	}
}
