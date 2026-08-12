// Copyright 2026 Casey West
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package okf

import "testing"

// verDoc builds a node whose freshness-relevant provenance lives in the §5.2
// `verified` key alongside okfctl-native created/modified. A single `verifiedAt`
// populates a bare-mapping `verified` (§11 MUST: normalized to a one-element
// list); created/modified are set so a resolved basis proves WHICH axis won.
func verDoc(title, created, modified, verifiedAt, body string) string {
	fm := "---\ntype: Concept\ntitle: " + title + "\n"
	if created != "" {
		fm += "created: " + created + "\n"
	}
	if modified != "" {
		fm += "modified: " + modified + "\n"
	}
	if verifiedAt != "" {
		fm += "verified: { by: human:casey, at: " + verifiedAt + " }\n"
	}
	fm += "---\n\n# " + title + "\n\n" + body + "\n"
	return fm
}

// verDocMulti builds a node with a two-event `verified` LIST (§5.2): the older
// event first, the newer second, to prove the LATEST verification wins.
func verDocMulti(title, created, modified, olderAt, newerAt, body string) string {
	fm := "---\ntype: Concept\ntitle: " + title + "\n"
	if created != "" {
		fm += "created: " + created + "\n"
	}
	if modified != "" {
		fm += "modified: " + modified + "\n"
	}
	fm += "verified:\n"
	fm += "  - { by: machine:reference_agent, at: " + olderAt + " }\n"
	fm += "  - { by: human:casey, at: " + newerAt + " }\n"
	fm += "---\n\n# " + title + "\n\n" + body + "\n"
	return fm
}

// A recently-`modified` node whose CONTENT was last `verified` long ago is
// STALE: the §5.2 verification instant is the freshness basis, so a mechanical
// reformat that only bumped `modified` no longer masks aged claims. This is the
// core defect fix — a corpus-wide `modified:` touch must not disarm freshness.
func TestAnalyze_Freshness_VerifiedBeatsModified(t *testing.T) {
	pinClock(t, fixedNow)
	// modified 2026-07-20 is fresh (6d); verified 2025-01-01 is > 180d -> stale.
	n := verDoc("V", "2024-06-01T00:00:00Z", "2026-07-20T00:00:00Z", "2025-01-01T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [V](v.md)\n",
		"v.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "v.md" {
		t.Fatalf("want v.md stale via verified.at (not masked by fresh modified), got %+v", rep.Freshness.Stale)
	}
	if got := rep.Freshness.Stale[0].Basis; got != "2025-01-01" {
		t.Fatalf("want basis 2025-01-01 (the verified date), got %q", got)
	}
	if rep.Freshness.Stale[0].AgeDays == nil || *rep.Freshness.Stale[0].AgeDays < 180 {
		t.Fatalf("want age >= 180 from verified.at, got %+v", rep.Freshness.Stale[0])
	}
}

// The LATEST verification event is the basis: an old event followed by a recent
// one keeps the node FRESH even though modified is old. Proves the reader takes
// the max `at` across the §5.2 event list, not the first entry.
func TestAnalyze_Freshness_LatestVerifiedWins(t *testing.T) {
	pinClock(t, fixedNow)
	// modified is old (would be stale) but the newer verified event is 6d fresh.
	n := verDocMulti("M", "2024-01-01T00:00:00Z", "2024-06-01T00:00:00Z",
		"2024-06-01T00:00:00Z", "2026-07-20T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [M](m.md)\n",
		"m.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 0 {
		t.Fatalf("want no stale (latest verified 2026-07-20 is fresh), got %+v", rep.Freshness.Stale)
	}
}

// When `verified` is absent the basis falls back to generated.at -> modified ->
// created exactly as before: a node with no verified but a fresh modified is NOT
// stale. Guards that the new precedence is additive, not a replacement.
func TestAnalyze_Freshness_NoVerifiedFallsBackToModified(t *testing.T) {
	pinClock(t, fixedNow)
	n := verDoc("F", "2024-01-01T00:00:00Z", "2026-07-20T00:00:00Z", "", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [F](f.md)\n",
		"f.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 0 {
		t.Fatalf("want no stale (fresh modified, no verified), got %+v", rep.Freshness.Stale)
	}
}

// Time-sensitive gate keys off the verified-derived age: a marker-bearing node
// that is `modified`-fresh but `verified` long ago surfaces (its claims are
// aged past the gate), while the same node verified recently does not. This is
// the second half of the defect fix — a live-pricing node re-touched but never
// re-verified must still surface for re-verification.
func TestAnalyze_Freshness_TimeSensitiveUsesVerifiedAge(t *testing.T) {
	pinClock(t, fixedNow)
	// gate = 0.5 * 180 = 90d. modified fresh either way; verified is the axis.
	agedVerified := verDoc("AV", "2024-01-01T00:00:00Z", "2026-07-20T00:00:00Z",
		"2026-01-01T00:00:00Z", "The latest pricing is deprecated now.") // verified ~206d -> surfaces
	freshVerified := verDoc("FV", "2024-01-01T00:00:00Z", "2026-07-20T00:00:00Z",
		"2026-07-20T00:00:00Z", "The latest pricing is deprecated now.") // verified 6d -> quiet
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [AV](av.md)\n- [FV](fv.md)\n",
		"av.md":    agedVerified,
		"fv.md":    freshVerified,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 1 || rep.Freshness.TimeSensitive[0].Path != "av.md" {
		t.Fatalf("want only av.md time-sensitive via verified age, got %+v", rep.Freshness.TimeSensitive)
	}
}

// verGenDoc builds a node dated by BOTH the canonical §5.2 `generated.at` and a
// `verified` event, to prove `verified` outranks `generated` in the basis order.
func verGenDoc(title, generatedAt, verifiedAt, body string) string {
	fm := "---\ntype: Concept\ntitle: " + title + "\n"
	fm += "generated: { by: reference_agent/gemini-2.5-pro, at: " + generatedAt + " }\n"
	fm += "verified: { by: human:casey, at: " + verifiedAt + " }\n"
	fm += "---\n\n# " + title + "\n\n" + body + "\n"
	return fm
}

// `verified` outranks `generated.at`: a node GENERATED long ago but VERIFIED
// recently is fresh. Locks the full basis order verified -> generated ->
// modified -> created.
func TestAnalyze_Freshness_VerifiedBeatsGenerated_Section5_2(t *testing.T) {
	pinClock(t, fixedNow)
	// generated 2024-01-01 (old); verified 2026-07-20 (6d fresh) -> not stale.
	n := verGenDoc("VG", "2024-01-01T00:00:00Z", "2026-07-20T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [VG](vg.md)\n",
		"vg.md":    n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 0 {
		t.Fatalf("want no stale (verified 2026-07-20 beats old generated.at), got %+v", rep.Freshness.Stale)
	}
}
