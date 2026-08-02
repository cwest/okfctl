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
)

// Spec-conformance suite for the v0.2 provenance/trust/lifecycle reader (§5,
// §7, §11, §12, §13). The load-bearing property this closes: okfctl is the
// consumer of a spec it does not own, so it must READ what a v0.2 producer
// writes — not merely tolerate it. A green validate against a v0.2 bundle is
// CORRECT floor behavior (§11 forbids rejecting unknown keys) but is NOT
// evidence the families are understood. These tests read the families back and
// assert their meaning, and prove the two §13.1 fallbacks and the v0.1 negative
// control (v0.2 support must change nothing for a v0.1 bundle).

// v02Bundle writes a minimal v0.2 bundle exercising every §5 provenance family
// and returns its root. The bundle declares okf_version 0.2 via the .okf
// sidecar (§12).
func v02Bundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.2\n")
	writeFile(t, dir, "index.md", "# Knowledge Base\n\nSee [revenue](/finance/revenue.md).\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "finance/revenue.md", `---
type: Metric
title: Revenue
description: Headline revenue for a fiscal year.
sources:
  - id: ga4-schema
    resource: https://developers.google.com/analytics/bigquery/export-schema
    title: GA4 BigQuery Export schema
    author: team:ga4-docs
    usage_count: 5000
    last_modified: 2026-05-30
usage_window: { from: 2026-06-01, to: 2026-06-30 }
generated: { by: reference_agent/gemini-2.5-pro, at: 2026-06-20T22:53:05Z }
verified:
  - { by: human:ahormati, at: 2026-06-25T09:00:00Z }
  - { by: process:finance-nightly, at: 2026-06-26T02:00:00Z }
status: stable
stale_after: 2026-09-23
---

# Definition

Revenue is total income for the fiscal year.
`)
	return dir
}

func TestConformance_V02BundleValidatesAndLintsClean_Section11(t *testing.T) {
	// §11: a v0.2 bundle carrying the full provenance families must pass the
	// spec floor with zero findings — unknown-to-v0.1 keys are permitted, never
	// rejected. This is the "passing is not understanding" control: it proves
	// permissiveness, which the reader tests below turn into actual support.
	b, err := Load(v02Bundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.OkfVersion != "0.2" { // §12: behavior is driven by the declared version.
		t.Errorf("OkfVersion = %q, want 0.2 (from .okf)", b.OkfVersion)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("v0.2 bundle must validate clean (§11); got findings: %v", f)
	}
	if f := Lint(b, LintOptions{}); len(f) != 0 {
		t.Fatalf("v0.2 bundle must lint clean; got findings: %v", f)
	}
}

func TestConformance_V02ReaderUnderstandsEveryFamily_Section5(t *testing.T) {
	// The families are READ, not merely tolerated (§5). This is what makes the
	// clean validate above meaningful support rather than accidental silence.
	b, err := Load(v02Bundle(t))
	if err != nil {
		t.Fatal(err)
	}
	n := b.Nodes["finance/revenue.md"]
	if n == nil {
		t.Fatal("finance/revenue.md not loaded as a concept node")
	}

	// §5.1 sources + credibility signals + usage_window.
	src := n.Sources()
	if len(src) != 1 || src[0].ID != "ga4-schema" || src[0].Author != "team:ga4-docs" {
		t.Fatalf("Sources() = %+v, want one ga4-schema entry", src)
	}
	if src[0].UsageCount == nil || *src[0].UsageCount != 5000 {
		t.Errorf("usage_count = %v, want 5000", src[0].UsageCount)
	}
	if src[0].UsageWindow == nil || src[0].UsageWindow.From != "2026-06-01" {
		t.Errorf("usage_window = %+v, want the shared window framed on the entry", src[0].UsageWindow)
	}

	// §5.2 generated.
	g, ok := n.Generated()
	if !ok || g.By != "reference_agent/gemini-2.5-pro" {
		t.Errorf("Generated() = %+v ok=%v", g, ok)
	}

	// §5.2 verified + §5.3 derived trust tier (a human verifier is present).
	if v := n.Verified(); len(v) != 2 {
		t.Errorf("Verified() len = %d, want 2", len(v))
	}
	if got := n.TrustTier(); got != TrustHumanReviewed {
		t.Errorf("TrustTier() = %q, want %q", got, TrustHumanReviewed)
	}

	// §5.4 status.
	if got := n.Status(); got != "stable" {
		t.Errorf("Status() = %q, want stable", got)
	}

	// §5.5 stale_after.
	if !n.IsStale(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)) {
		t.Error("IsStale(2026-09-23) = false, want true (today >= stale_after)")
	}
	if n.IsStale(time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)) {
		t.Error("IsStale(2026-09-22) = true, want false (before stale_after)")
	}
}

func TestConformance_V01NegativeControl_UnchangedAndFallbacks_Section13_1(t *testing.T) {
	// Negative control: v0.2 support must change NOTHING for a v0.1 bundle. A
	// v0.1 concept (legacy `timestamp`, body `# Citations`, no v0.2 families)
	// validates and lints clean exactly as before, and the reader honors the
	// two §13.1 fallbacks.
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.1\n")
	writeFile(t, dir, "index.md", "# Knowledge Base\n\nSee [income](/finance/income.md).\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "finance/income.md", `---
type: Metric
title: Income statement
description: Headline income-statement figures for a fiscal year.
timestamp: '2026-05-28T22:53:05+00:00'
---

# Definition

The income statement reports revenue and gross profit.

# Citations

[1] GA4 BigQuery Export schema
[2] Finance ledger export
`)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.OkfVersion != "0.1" {
		t.Errorf("OkfVersion = %q, want 0.1", b.OkfVersion)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("v0.1 bundle must validate clean (unchanged); got: %v", f)
	}
	if f := Lint(b, LintOptions{}); len(f) != 0 {
		t.Fatalf("v0.1 bundle must lint clean (unchanged); got: %v", f)
	}

	n := b.Nodes["finance/income.md"]
	if n == nil {
		t.Fatal("finance/income.md not loaded")
	}
	// No v0.2 families present — the reader reports their absence, not an error.
	if len(n.Sources()) != 0 {
		t.Errorf("v0.1 node Sources() = %v, want empty", n.Sources())
	}
	if len(n.Verified()) != 0 {
		t.Errorf("v0.1 node Verified() = %v, want empty", n.Verified())
	}
	if got := n.TrustTier(); got != TrustUnverified {
		t.Errorf("v0.1 node TrustTier() = %q, want unverified", got)
	}
	if got := n.Status(); got != "stable" {
		t.Errorf("v0.1 node Status() = %q, want stable (default)", got)
	}
	if _, ok := n.StaleAfter(); ok {
		t.Error("v0.1 node StaleAfter() ok = true, want false")
	}

	// §13.1 fallback: generated.at falls back to legacy timestamp.
	g, ok := n.Generated()
	if !ok {
		t.Fatal("Generated() fallback ok = false, want true (legacy timestamp)")
	}
	if g.By != "" {
		t.Errorf("Generated().By = %q, want empty (v0.1 carries no actor)", g.By)
	}
	want := time.Date(2026, 5, 28, 22, 53, 5, 0, time.UTC)
	if !g.At.Equal(want) {
		t.Errorf("Generated().At = %v, want %v (from timestamp)", g.At, want)
	}

	// §13.1 fallback: provenance count falls back to the body # Citations list.
	if got := n.SourceCitations(); got != 2 {
		t.Errorf("SourceCitations() = %d, want 2 (legacy body fallback)", got)
	}
}

// TestConformance_UnrecognizedVersionIsBestEffort_Section12 proves §12: a bundle
// declaring a version okfctl does not understand is consumed best-effort, never
// refused. The reader honors the v0.2 families regardless of the declared
// version string.
func TestConformance_UnrecognizedVersionIsBestEffort_Section12(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 9.9\n") // a version from the future.
	writeFile(t, dir, "index.md", "# KB\n\nSee [x](/x.md).\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "x.md", `---
type: Reference
verified: { by: human:reviewer, at: 2026-06-25T09:00:00Z }
---

# Definition

Best-effort.
`)
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// §12: unrecognized version is best-effort, never a hard failure.
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("§12: unrecognized-version bundle must not be refused; got: %v", f)
	}
	// The §11 bare-mapping MUST still applies regardless of declared version.
	n := b.Nodes["x.md"]
	if got := n.Verified(); len(got) != 1 || got[0].By != "human:reviewer" {
		t.Errorf("bare verified under unrecognized version = %+v, want one human:reviewer", got)
	}
	if got := n.TrustTier(); got != TrustHumanReviewed {
		t.Errorf("TrustTier() = %q, want human-reviewed", got)
	}
}
