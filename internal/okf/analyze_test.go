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
	"strings"
	"testing"
	"time"
)

// tsDoc builds a node with type/title, an optional tags list, created/modified
// timestamps, and a body — enough to exercise the analyze dimensions.
func tsDoc(typ, title string, tags []string, created, modified, body string) string {
	fm := "---\ntype: " + typ + "\ntitle: " + title + "\n"
	if len(tags) > 0 {
		fm += "tags: ["
		for i, t := range tags {
			if i > 0 {
				fm += ", "
			}
			fm += t
		}
		fm += "]\n"
	}
	if created != "" {
		fm += "created: " + created + "\n"
	}
	if modified != "" {
		fm += "modified: " + modified + "\n"
	}
	fm += "---\n\n# " + title + "\n\n" + body + "\n"
	return fm
}

// pinClock pins nowUTC to a fixed instant for the duration of a test.
func pinClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := nowUTC
	nowUTC = func() time.Time { return at }
	t.Cleanup(func() { nowUTC = prev })
}

var fixedNow = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

// --- Coverage / gaps -------------------------------------------------------

func TestAnalyze_Coverage_DanglingForwardLink(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "See [the missing one](missing.md) for more, and this long body line one.\nline two here.\nline three here.\nline four.\nline five.\nline six.\nline seven.\nline eight.\nline nine.\nline ten.\nline eleven.\nline twelve.\nline thirteen.\nline fourteen.\nline fifteen.\nline sixteen."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Coverage.DanglingLinks) != 1 {
		t.Fatalf("want 1 dangling link, got %d: %+v", len(rep.Coverage.DanglingLinks), rep.Coverage.DanglingLinks)
	}
	d := rep.Coverage.DanglingLinks[0]
	if d.From != "a.md" || d.Target != "missing.md" {
		t.Fatalf("want a.md -> missing.md, got %s -> %s", d.From, d.Target)
	}
}

func TestAnalyze_Coverage_ResolvedLinkNotDangling(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "See [B](b.md)."),
		"b.md":     lintDoc("Concept", "B", "Body."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Coverage.DanglingLinks) != 0 {
		t.Fatalf("want no dangling links, got %+v", rep.Coverage.DanglingLinks)
	}
}

// A "/"-absolute link that resolves (OKF §5.1, bundle-root relative) is NOT
// dangling. The real corpus writes every concept cross-link this way, so a
// resolved "/x/y.md" must never be reported as a coverage gap.
func TestAnalyze_Coverage_RootAbsoluteLinkNotDangling(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md":      "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](research/a.md)\n- [B](method/b.md)\n",
		"research/a.md": lintDoc("Concept", "A", "See [B](/method/b.md)."),
		"method/b.md":   lintDoc("Concept", "B", "Body."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	for _, d := range rep.Coverage.DanglingLinks {
		if d.Target == "/method/b.md" {
			t.Fatalf("resolved root-absolute link should not be dangling, got %+v", rep.Coverage.DanglingLinks)
		}
	}
}

func TestAnalyze_Coverage_ThinNode(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "one line only."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Coverage.ThinNodes) != 1 || rep.Coverage.ThinNodes[0].Path != "a.md" {
		t.Fatalf("want a.md thin, got %+v", rep.Coverage.ThinNodes)
	}
}

func TestAnalyze_Coverage_UncitedAndSingleCitation(t *testing.T) {
	pinClock(t, fixedNow)
	single := lintDoc("Concept", "S", "Body.\n\n# Citations\n\n[1] One source.")
	none := lintDoc("Concept", "N", "Body with no citations section at all.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [S](s.md)\n- [N](n.md)\n",
		"s.md":     single,
		"n.md":     none,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Coverage.SingleCitation) != 1 || rep.Coverage.SingleCitation[0].Path != "s.md" {
		t.Fatalf("want s.md single-citation, got %+v", rep.Coverage.SingleCitation)
	}
	if len(rep.Coverage.Uncited) != 1 || rep.Coverage.Uncited[0].Path != "n.md" {
		t.Fatalf("want n.md uncited, got %+v", rep.Coverage.Uncited)
	}
}

// --- Freshness -------------------------------------------------------------

func TestAnalyze_Freshness_StaleByModified(t *testing.T) {
	pinClock(t, fixedNow)
	// modified 2025-01-01 is > 180 days before 2026-07-26 -> stale.
	old := tsDoc("Concept", "Old", nil, "2025-01-01T00:00:00Z", "2025-01-01T00:00:00Z", "Body.")
	fresh := tsDoc("Concept", "Fresh", nil, "2026-07-01T00:00:00Z", "2026-07-20T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Old](old.md)\n- [Fresh](fresh.md)\n",
		"old.md":   old,
		"fresh.md": fresh,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "old.md" {
		t.Fatalf("want old.md stale only, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].AgeDays == nil || *rep.Freshness.Stale[0].AgeDays < 180 {
		t.Fatalf("want age >= 180, got %+v", rep.Freshness.Stale[0])
	}
}

func TestAnalyze_Freshness_ModifiedFallsBackToCreated(t *testing.T) {
	pinClock(t, fixedNow)
	// No modified; created is old -> stale via created fallback.
	n := tsDoc("Concept", "C", nil, "2024-01-01T00:00:00Z", "", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [C](c.md)\n",
		"c.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "c.md" {
		t.Fatalf("want c.md stale via created fallback, got %+v", rep.Freshness.Stale)
	}
}

func TestAnalyze_Freshness_UndatedFlaggedSoftly(t *testing.T) {
	pinClock(t, fixedNow)
	n := tsDoc("Concept", "U", nil, "", "", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [U](u.md)\n",
		"u.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "u.md" {
		t.Fatalf("want u.md flagged undated, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].AgeDays != nil {
		t.Fatalf("want nil age for undated, got %v", *rep.Freshness.Stale[0].AgeDays)
	}
}

func TestAnalyze_Freshness_TimeSensitiveAgeGated(t *testing.T) {
	pinClock(t, fixedNow)
	// marked & aged past 0.5*180=90d -> surfaces. marked & fresh -> not.
	agedMarked := tsDoc("Concept", "AM", nil, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "The latest pricing is deprecated now.")
	freshMarked := tsDoc("Concept", "FM", nil, "2026-07-20T00:00:00Z", "2026-07-20T00:00:00Z", "The latest pricing is deprecated now.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [AM](am.md)\n- [FM](fm.md)\n",
		"am.md":    agedMarked,
		"fm.md":    freshMarked,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 1 || rep.Freshness.TimeSensitive[0].Path != "am.md" {
		t.Fatalf("want only am.md time-sensitive, got %+v", rep.Freshness.TimeSensitive)
	}
	if len(rep.Freshness.TimeSensitive[0].Markers) == 0 {
		t.Fatalf("want markers recorded, got none")
	}
}

// genDoc builds a node whose freshness date lives in a spec field rather than
// the okfctl-native modified/created. `generatedAt` populates `generated.at`
// (§5.2); `timestamp` populates the legacy `timestamp` (§13.1). Either may be
// "" to omit it. modified/created are intentionally never set here so a
// resolved basis proves it came through the spec path.
func genDoc(title, generatedAt, timestamp, body string) string {
	fm := "---\ntype: Concept\ntitle: " + title + "\n"
	if generatedAt != "" {
		fm += "generated: { by: reference_agent/gemini-2.5-pro, at: " + generatedAt + " }\n"
	}
	if timestamp != "" {
		fm += "timestamp: " + timestamp + "\n"
	}
	fm += "---\n\n# " + title + "\n\n" + body + "\n"
	return fm
}

// §5.2 negative control: a node dated ONLY by the canonical generated.at
// resolves a basis where before this change it reported "(none)". This is the
// spec-conformant bundle the fix targets.
func TestAnalyze_Freshness_ResolvesGeneratedAt_Section5_2(t *testing.T) {
	pinClock(t, fixedNow)
	// generated.at 2025-01-01 is > 180 days before 2026-07-26 -> stale, dated.
	n := genDoc("G", "2025-01-01T00:00:00Z", "", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [G](g.md)\n",
		"g.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "g.md" {
		t.Fatalf("want g.md stale via generated.at, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].Basis == "(none)" {
		t.Fatalf("want a resolved basis, got (none): %+v", rep.Freshness.Stale[0])
	}
	if rep.Freshness.Stale[0].AgeDays == nil || *rep.Freshness.Stale[0].AgeDays < 180 {
		t.Fatalf("want age >= 180 from generated.at, got %+v", rep.Freshness.Stale[0])
	}
}

// §13.1 legacy fallback: a node dated ONLY by the legacy `timestamp` resolves a
// basis (this is the reporter's exact fixture — issue #39).
func TestAnalyze_Freshness_ResolvesLegacyTimestamp_Section13_1(t *testing.T) {
	pinClock(t, fixedNow)
	n := genDoc("T", "", "2025-01-01T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [T](t.md)\n",
		"t.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "t.md" {
		t.Fatalf("want t.md stale via legacy timestamp, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].Basis == "(none)" {
		t.Fatalf("want a resolved basis, got (none): %+v", rep.Freshness.Stale[0])
	}
	if rep.Freshness.Stale[0].AgeDays == nil {
		t.Fatalf("want a non-nil age from timestamp, got %+v", rep.Freshness.Stale[0])
	}
}

// §5.2 precedence: generated.at is the canonical basis and wins over the
// okfctl-native modified/created compatibility fields when both are present.
func TestAnalyze_Freshness_GeneratedAtBeatsModified_Section5_2(t *testing.T) {
	pinClock(t, fixedNow)
	// generated.at is OLD (stale); modified is FRESH. If precedence were wrong
	// (modified first) the node would read fresh and not surface.
	n := "---\ntype: Concept\ntitle: P\n" +
		"generated: { by: reference_agent/gemini-2.5-pro, at: 2025-01-01T00:00:00Z }\n" +
		"modified: 2026-07-20T00:00:00Z\n---\n\n# P\n\nBody.\n"
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [P](p.md)\n",
		"p.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "p.md" {
		t.Fatalf("want p.md stale via generated.at precedence, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].AgeDays == nil || *rep.Freshness.Stale[0].AgeDays < 180 {
		t.Fatalf("want age >= 180 from generated.at, not modified, got %+v", rep.Freshness.Stale[0])
	}
}

// Positive control (regression guard): the okfctl-native modified/created path
// is unchanged — a node dated only by modified still resolves a basis.
func TestAnalyze_Freshness_ModifiedStillResolves(t *testing.T) {
	pinClock(t, fixedNow)
	n := tsDoc("Concept", "M", nil, "", "2025-01-01T00:00:00Z", "Body.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [M](m.md)\n",
		"m.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "m.md" {
		t.Fatalf("want m.md stale via modified, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].Basis == "(none)" {
		t.Fatalf("want a resolved basis from modified, got (none): %+v", rep.Freshness.Stale[0])
	}
}

// §5.2/§13.1 zero-value guard: a `generated` mapping with no usable `at` and no
// legacy timestamp and no modified/created must NOT read an empty string or a
// zero time.Time as a valid basis — it stays undated, flagged softly.
func TestAnalyze_Freshness_EmptyGeneratedIsUndated(t *testing.T) {
	pinClock(t, fixedNow)
	// generated has only `by`; no at, no timestamp, no modified/created.
	n := "---\ntype: Concept\ntitle: E\n" +
		"generated: { by: reference_agent/gemini-2.5-pro }\n---\n\n# E\n\nBody.\n"
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [E](e.md)\n",
		"e.md":     n,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.Stale) != 1 || rep.Freshness.Stale[0].Path != "e.md" {
		t.Fatalf("want e.md flagged undated, got %+v", rep.Freshness.Stale)
	}
	if rep.Freshness.Stale[0].AgeDays != nil {
		t.Fatalf("want nil age for undated node, got %v", *rep.Freshness.Stale[0].AgeDays)
	}
	if rep.Freshness.Stale[0].Basis != "(none)" {
		t.Fatalf("want basis (none) for empty generated, got %q", rep.Freshness.Stale[0].Basis)
	}
}

// --- Connectivity ----------------------------------------------------------

func TestAnalyze_Connectivity_OrphanAndWeaklyLinked(t *testing.T) {
	pinClock(t, fixedNow)
	// hub links to a; a links back to hub (a: in1+out1=2, hub: in1+out1=2).
	// orphan: no links at all. weak: exactly one link (linked only from hub-ish).
	b := mkLintBundle(t, map[string]string{
		"index.md":  "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Hub](hub.md)\n",
		"hub.md":    lintDoc("Concept", "Hub", "See [Weak](weak.md)."),
		"weak.md":   lintDoc("Concept", "Weak", "Body only, no outbound."),
		"orphan.md": lintDoc("Concept", "Orphan", "No links in or out."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	// orphan.md is linked from index? No — index only links hub. So orphan.md
	// has 0 in, 0 out -> orphan. weak.md has 1 in (from hub), 0 out -> weak.
	// hub.md has 1 in (from index), 1 out -> not weak, not orphan.
	if !pathIn(rep.Connectivity.Orphans, "orphan.md") {
		t.Fatalf("want orphan.md in orphans, got %+v", rep.Connectivity.Orphans)
	}
	if !weakPathIn(rep.Connectivity.WeaklyLinked, "weak.md") {
		t.Fatalf("want weak.md weakly-linked, got %+v", rep.Connectivity.WeaklyLinked)
	}
	if pathIn(rep.Connectivity.Orphans, "hub.md") || weakPathIn(rep.Connectivity.WeaklyLinked, "hub.md") {
		t.Fatalf("hub.md should be neither orphan nor weak, got orphans=%+v weak=%+v", rep.Connectivity.Orphans, rep.Connectivity.WeaklyLinked)
	}
}

func TestAnalyze_Clusters_TagCoOccurrence(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [D](d.md)\n",
		"a.md":     tsDoc("Concept", "A", []string{"wine"}, "", "", "b"),
		"b.md":     tsDoc("Concept", "B", []string{"wine"}, "", "", "b"),
		"c.md":     tsDoc("Concept", "C", []string{"wine"}, "", "", "b"),
		"d.md":     tsDoc("Concept", "D", []string{"tech"}, "", "", "b"),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Clusters) != 1 {
		t.Fatalf("want 1 cluster (wine >= 3), got %+v", rep.Clusters)
	}
	if rep.Clusters[0].Tag != "wine" || len(rep.Clusters[0].Nodes) != 3 {
		t.Fatalf("want wine/3, got %+v", rep.Clusters[0])
	}
}

// A numeric tag (YAML parses bare `403` as an int) is still a valid tag and
// must cluster. The real corpus tags several security nodes with `403`; the
// reference stringifies every tag element, so okfctl must too.
func TestAnalyze_Clusters_NumericTag(t *testing.T) {
	pinClock(t, fixedNow)
	// tags: [403] parses 403 as an int in YAML.
	numTag := func(title string) string {
		return "---\ntype: Concept\ntitle: " + title + "\ntags: [403]\n---\n\n# " + title + "\n\nb\n"
	}
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     numTag("A"),
		"b.md":     numTag("B"),
		"c.md":     numTag("C"),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	found := false
	for _, c := range rep.Clusters {
		if c.Tag == "403" && len(c.Nodes) == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("want numeric tag 403 clustered across 3 nodes, got %+v", rep.Clusters)
	}
}

// --- Structure -------------------------------------------------------------

func TestAnalyze_Structure_DuplicateTitles(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md":  "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](wine/b.md)\n",
		"a.md":      lintDoc("Concept", "Tannin", "b"),
		"wine/b.md": lintDoc("Concept", "Tannin", "b"),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Structure.DuplicateTitles) != 1 {
		t.Fatalf("want 1 duplicate-title group, got %+v", rep.Structure.DuplicateTitles)
	}
	if len(rep.Structure.DuplicateTitles[0].Members) != 2 {
		t.Fatalf("want 2 members, got %+v", rep.Structure.DuplicateTitles[0])
	}
}

func TestAnalyze_Structure_NearDuplicateSlugs(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](tannin.md)\n- [B](tannins.md)\n",
		"tannin.md":  lintDoc("Concept", "Tannin", "b"),
		"tannins.md": lintDoc("Concept", "Tannins", "b"),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Structure.NearDuplicateSlugs) != 1 {
		t.Fatalf("want 1 near-duplicate-slug pair, got %+v", rep.Structure.NearDuplicateSlugs)
	}
}

// --- report-level ----------------------------------------------------------

func TestAnalyze_EmptyDimensionsAreEmptySlicesNotNil(t *testing.T) {
	pinClock(t, fixedNow)
	// A minimal clean-ish corpus: exercise that list fields never marshal to
	// JSON null (nil slice). The machine consumer (curation sweep) relies on
	// arrays being present and iterable.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     tsDoc("Concept", "A", nil, "2026-07-20T00:00:00Z", "2026-07-20T00:00:00Z", strings.Repeat("body line\n", 20)),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if rep.Freshness.Stale == nil {
		t.Fatalf("Freshness.Stale must be a non-nil slice for JSON stability")
	}
	if rep.Freshness.TimeSensitive == nil {
		t.Fatalf("Freshness.TimeSensitive must be a non-nil slice")
	}
	if rep.Connectivity.Orphans == nil || rep.Connectivity.WeaklyLinked == nil {
		t.Fatalf("Connectivity slices must be non-nil")
	}
	if rep.Coverage.DanglingLinks == nil || rep.Coverage.ThinNodes == nil ||
		rep.Coverage.Uncited == nil || rep.Coverage.SingleCitation == nil ||
		rep.Coverage.KnownGaps == nil {
		t.Fatalf("Coverage slices must be non-nil")
	}
	if rep.Clusters == nil {
		t.Fatalf("Clusters must be non-nil")
	}
	if rep.Structure.DuplicateTitles == nil || rep.Structure.NearDuplicateSlugs == nil {
		t.Fatalf("Structure slices must be non-nil")
	}
}

func TestAnalyze_SummaryCountsNodes(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "b"),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if rep.Summary.Nodes != 1 {
		t.Fatalf("want 1 node in summary, got %d", rep.Summary.Nodes)
	}
	if rep.Summary.StaleThresholdDays != 180 {
		t.Fatalf("want default stale threshold 180 in summary, got %d", rep.Summary.StaleThresholdDays)
	}
}

func pathIn(items []AnalyzeNodeRef, path string) bool {
	for _, it := range items {
		if it.Path == path {
			return true
		}
	}
	return false
}

func weakPathIn(items []WeaklyLinked, path string) bool {
	for _, it := range items {
		if it.Path == path {
			return true
		}
	}
	return false
}
