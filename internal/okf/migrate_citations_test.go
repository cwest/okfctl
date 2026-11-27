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
)

// Spec-conformance suite for the parts of migrate that guard the Appendix B
// fallbacks (OKF v0.3 §12.1, Appendix B, §5.1): a citation entry with exactly
// one follow-able resource becomes a source; the 0.3 declaration is held back
// while an item still depends on a v0.1 fallback; and re-applying a plan to an
// already-migrated tree changes nothing.

func planFor(t *testing.T, dir, actor string, force bool) (*Bundle, MigratePlan) {
	t.Helper()
	b := loadMigrate(t, dir)
	plan, err := PlanMigrationWith(b, MigrateOptions{GeneratedBy: actor, ForceDeclare: force})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	return b, plan
}

// TestConformance_MigrateCitationEntries_Section5_1_SingleResourceBecomesSource
// is the positive control: an entry that carries exactly one follow-able
// resource, wherever it sits in the entry and however the entry wraps, becomes
// one sources entry with that resource. Its title is the work's name only
// (citationName); these entries mark none, so they get no title.
func TestConformance_MigrateCitationEntries_Section5_1_SingleResourceBecomesSource(t *testing.T) {
	body := "# Def\n\n# Citations\n\n" +
		"Status key: **VERIFIED** = checked against the source.\n" +
		"[1] VERIFIED — Ollama, license **MIT**, active (pushed\n" +
		"2026-06-26). GitHub REST API: <https://api.github.com/repos/ollama/ollama>.\n" +
		"- **[2] [VERIFIED — VENDOR-PRICING]** Anthropic API pricing\n" +
		"  (https://claude.com/pricing) — Opus-class $5/MTok input.\n" +
		"- [3] Paper, arXiv:2310.20410 — https://arxiv.org/abs/2310.20410 — and again\n" +
		"  https://arxiv.org/abs/2310.20410.\n"
	got := citationEntries(body)
	if len(got) != 3 {
		t.Fatalf("want 3 entries (legend excluded, wrapped lines joined), got %d: %q", len(got), got)
	}
	cases := []struct{ res, title string }{
		{"https://api.github.com/repos/ollama/ollama", ""},
		{"https://claude.com/pricing", ""},
		{"https://arxiv.org/abs/2310.20410", ""},
	}
	for i, c := range cases {
		s, ok := citationSource(got[i])
		if !ok {
			t.Fatalf("§5.1: entry %d has one resource but was not converted: %q", i, got[i])
		}
		if s.Resource != c.res || s.Title != c.title {
			t.Errorf("§5.1: entry %d = %+v\nwant resource %q title %q", i, s, c.res, c.title)
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_AmbiguousStayJudgment is
// the negative control: an entry with two different resources, or with none,
// is left for judgment. A sole link or URL keeps the link text (or no title),
// so a bare-URL entry does not repeat itself as its own title.
func TestConformance_MigrateCitationEntries_Section5_1_AmbiguousStayJudgment(t *testing.T) {
	for _, entry := range []string{
		"[4] VERIFIED — two sources: https://a.example/x and https://b.example/y.",
		"[5] VERIFIED — [doc](https://a.example/x) mirrors https://b.example/x",
		"[6] VERIFIED — on-box probe, no URL.",
		"- Personal observation from a tasting, 2026.",
	} {
		if s, ok := citationSource(entry); ok {
			t.Errorf("§5.1: ambiguous entry %q was converted to %+v", entry, s)
		}
	}
	for entry, want := range map[string]SourceEdit{
		"- https://wiki.acme/x":         {Resource: "https://wiki.acme/x"},
		"- <https://wiki.acme/x>":       {Resource: "https://wiki.acme/x"},
		"- [Other concept](/other.md)":  {Resource: "/other.md", Title: "Other concept"},
		"- [](/other.md)":               {Resource: "/other.md"},
		"1. [Spec](https://s.example/)": {Resource: "https://s.example/", Title: "Spec"},
	} {
		if s, ok := citationSource(entry); !ok || s != want {
			t.Errorf("entry %q = %+v ok=%v, want %+v", entry, s, ok, want)
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_ProseLinkLineIsNotAnEntry:
// a prose paragraph inside `# Citations` whose wrapped line happens to start
// with a markdown link is not a citation entry (analyze.go's citationLinkRe
// draws the same line), so its text never becomes a title. A bulleted link is
// a list item and still is an entry.
func TestConformance_MigrateCitationEntries_Section5_1_ProseLinkLineIsNotAnEntry(t *testing.T) {
	body := "# Citations\n\n" +
		"This memo inherits its facts from the\n" +
		"[landscape node](/infra/landscape.md)\n" +
		"and does not re-verify them.\n\n" +
		"[1] VERIFIED — see [/infra/landscape.md](/infra/landscape.md).\n" +
		"- [Other concept](/other.md)\n"
	got := citationEntries(body)
	want := []string{
		"[1] VERIFIED — see [/infra/landscape.md](/infra/landscape.md).",
		"- [Other concept](/other.md)",
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_YearContinuationStaysInEntry
// is the positive control for hard-wrapped entries: a wrapped line inside a
// bracket-key or bullet entry that merely begins with digits and a period (a
// year: `2025. https://...`) continues that entry. Splitting it would turn one
// Appendix B citation into an untitled §5.1 source plus a bogus judgment item.
func TestConformance_MigrateCitationEntries_Section5_1_YearContinuationStaysInEntry(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Paper\n---\n\n# Def\n\n# Citations\n\n" +
			"[1] VERIFIED — Laban et al., \"LLMs Get Lost In Multi-Turn Conversation,\" arXiv:2505.06120,\n" +
			"2025. https://arxiv.org/abs/2505.06120\n\n" +
			"- Ordóñez & Roggen, *DeepConvLSTM*, *Sensors*\n" +
			"2016. <https://doi.org/10.3390/s16010115>\n",
	})
	_, plan := planFor(t, dir, "human:casey", false)
	if len(plan.Judgment) != 0 {
		t.Fatalf("§5.1: a wrapped year line split an entry into judgment items: %+v", plan.Judgment)
	}
	nm := planNodeMigration(plan, "a.md")
	if nm == nil {
		t.Fatalf("no migration planned for a.md")
	}
	// The entry joins across its wrapped year line: the name comes from the
	// first line and the resource from the second (§5.1: title is the name).
	want := []SourceEdit{
		{Resource: "https://arxiv.org/abs/2505.06120", Title: "LLMs Get Lost In Multi-Turn Conversation", ID: "1"},
		{Resource: "https://doi.org/10.3390/s16010115", Title: "DeepConvLSTM", ID: "cite-2"},
	}
	if len(nm.Sources) != len(want) {
		t.Fatalf("§5.1: want %d sources (one per entry), got %+v", len(want), nm.Sources)
	}
	for i := range want {
		if nm.Sources[i] != want[i] {
			t.Errorf("§5.1: source %d = %+v\nwant %+v", i, nm.Sources[i], want[i])
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_NumberedListStillSplits is
// the negative control: a real ordered list, whether it opens the section or
// follows a blank line, still yields one entry per item, and sibling bullets
// stay separate entries. Text indented under an entry (a wrapped status tag)
// joins that entry rather than starting a new one.
func TestConformance_MigrateCitationEntries_Section5_1_NumberedListStillSplits(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"tight list": {
			"# Citations\n1. First source https://a\n2. Second https://b\n",
			[]string{"1. First source https://a", "2. Second https://b"},
		},
		"list after legend and blank": {
			"# Citations\n\nStatus key: [V] = verified.\n\n1. First source https://a\n   wrapped tail\n2. Second https://b\n\n10. Tenth https://c\n",
			[]string{"1. First source https://a wrapped tail", "2. Second https://b", "10. Tenth https://c"},
		},
		"bracket entry then list after blank": {
			"# Citations\n[1] VERIFIED — x https://x\n\n2. Second https://b\n",
			[]string{"[1] VERIFIED — x https://x", "2. Second https://b"},
		},
		"indented bracket text inside an ordered item": {
			"# Citations\n4. **[Yegge]** Revenge, relevant to the\n   **[NUANCED]** note. <https://s.example/r>.\n5. **[1% rule]** context, no URL.\n",
			[]string{"4. **[Yegge]** Revenge, relevant to the **[NUANCED]** note. <https://s.example/r>.", "5. **[1% rule]** context, no URL."},
		},
		"indented status tag inside a bullet entry": {
			"# Citations\n- https://a.example/doc\n  — the claim. **[PRIMARY — §1]**\n  [PEER-REVIEWED.]\n- https://b.example/doc\n",
			[]string{"- https://a.example/doc — the claim. **[PRIMARY — §1]** [PEER-REVIEWED.]", "- https://b.example/doc"},
		},
	} {
		got := citationEntries(tc.body)
		if len(got) != len(tc.want) {
			t.Errorf("%s: entries = %q, want %q", name, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s: entry %d = %q, want %q", name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_WrappedBracketKeyStaysInEntry
// is the positive control for hard-wrapped entries whose continuation line
// begins with a bracket token: an inline back-reference (`[9]: C4 provides`,
// `[4]).`), a bold status tag (`**[2026-11-25 corrected]**`), or bracketed
// prose. A `[key]` line inside an open entry is wrapped text unless its key is
// the next citation key in sequence (see the negative control). Splitting
// there turns one Appendix B citation into a fragment §5.1 title plus a bogus
// judgment item.
func TestConformance_MigrateCitationEntries_Section5_1_WrappedBracketKeyStaysInEntry(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"back-reference after a bracket entry": {
			"# Citations\n\n[10] VERIFIED — Raffel et al. 2019 (C4), same source as\n[9]: C4 provides web text. https://arxiv.org/abs/1910.10683\n",
			[]string{"[10] VERIFIED — Raffel et al. 2019 (C4), same source as [9]: C4 provides web text. https://arxiv.org/abs/1910.10683"},
		},
		"back-reference after a bullet entry": {
			"# Citations\n\n- [2] Attestation semantics, W3C WebAuthn (see\n[1]); key held by the authenticator. https://w3.org/TR/webauthn-2/\n- [3] Next https://c\n",
			[]string{"- [2] Attestation semantics, W3C WebAuthn (see [1]); key held by the authenticator. https://w3.org/TR/webauthn-2/", "- [3] Next https://c"},
		},
		"bold status tag after a bracket entry": {
			"# Citations\n\n[7] SST §18 applies to all managed models.\n**[2026-11-25 corrected]** Grounding stores queries. https://cloud.google.com/terms\n",
			[]string{"[7] SST §18 applies to all managed models. **[2026-11-25 corrected]** Grounding stores queries. https://cloud.google.com/terms"},
		},
		"back-reference to a later key": {
			"# Citations\n\n[1] Context — HIPAA BAA scope is verified in citation\n[21].\n",
			[]string{"[1] Context — HIPAA BAA scope is verified in citation [21]."},
		},
		"indented back-reference inside an ordered entry": {
			"# Citations\n\n4. Orca, OSDI '22 — quoted in the PagedAttention primary\n   [1] (which implements Orca). https://example.com/orca\n5. Next https://b\n",
			[]string{"4. Orca, OSDI '22 — quoted in the PagedAttention primary [1] (which implements Orca). https://example.com/orca", "5. Next https://b"},
		},
		"back-reference after a non-numeric key": {
			"# Citations\n\n[V] SOURCE VIDEO — claims checked (see [1]); not in\n[2]; others. https://youtu.be/x\n",
			[]string{"[V] SOURCE VIDEO — claims checked (see [1]); not in [2]; others. https://youtu.be/x"},
		},
		"bracketed prose after an ordered entry": {
			"# Citations\n\n1. Harley, \"Icon Usability,\" NN/g 2014\n[\"universal\" icons rare]. https://nngroup.com/icons\n2. Second https://b\n",
			[]string{"1. Harley, \"Icon Usability,\" NN/g 2014 [\"universal\" icons rare]. https://nngroup.com/icons", "2. Second https://b"},
		},
	} {
		got := citationEntries(tc.body)
		if len(got) != len(tc.want) {
			t.Errorf("§5.1 %s: entries = %q, want %q", name, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("§5.1 %s: entry %d = %q, want %q", name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_WrappedBracketKeyPlansOneSource
// drives the same defect through migrate: the whole entry becomes one source
// and no fragment judgment item appears. The entry marks no work name, so the
// source is untitled (§5.1, name only) and only that is reported.
func TestConformance_MigrateCitationEntries_Section5_1_WrappedBracketKeyPlansOneSource(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Paper\n---\n\n# Def\n\n# Citations\n\n" +
			"[2] VERIFIED — origin binding (see\n[1]); phishing resistance follows. https://w3.org/TR/webauthn-2/\n",
	})
	_, plan := planFor(t, dir, "human:casey", false)
	for _, j := range plan.Judgment {
		if j.Kind != JudgmentSourceTitleUnnamed {
			t.Fatalf("§5.1: a wrapped [k] line split an entry into judgment items: %+v", plan.Judgment)
		}
	}
	if len(plan.Judgment) != 1 {
		t.Fatalf("want one source-title-unnamed item for the whole entry, got %+v", plan.Judgment)
	}
	nm := planNodeMigration(plan, "a.md")
	if nm == nil {
		t.Fatalf("no migration planned for a.md")
	}
	want := SourceEdit{Resource: "https://w3.org/TR/webauthn-2/", ID: "2"}
	if len(nm.Sources) != 1 || nm.Sources[0] != want {
		t.Fatalf("§5.1: sources = %+v, want [%+v]", nm.Sources, want)
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_BracketKeyListStillSplits
// is the negative control: bracket-key entries separated by blank lines, a
// bracket entry right after a status-key or legend line, a tight bulleted
// `- [1]` / `- [2]` list, a run of markdown link reference definitions
// (`[label]: url`, each its own definition in CommonMark), a tight unbulleted
// `[1]` / `[2]` list, and lettered sub-entries (`[1]` / `[1a]` / `[1b]`) on
// consecutive lines each still yield one entry per citation.
func TestConformance_MigrateCitationEntries_Section5_1_BracketKeyListStillSplits(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"first line after the heading": {
			"# Citations\n[1] First https://a\n\n[2] Second https://b\n",
			[]string{"[1] First https://a", "[2] Second https://b"},
		},
		"blank-separated wrapped entries": {
			"# Citations\n\n[1] First source, long\nwrapped tail https://a\n\n[2] Second https://b\n\n**[3]** Third https://c\n",
			[]string{"[1] First source, long wrapped tail https://a", "[2] Second https://b", "**[3]** Third https://c"},
		},
		"after a status-key line": {
			"# Citations\n\nStatus key: **VERIFIED** = checked.\n[1] VERIFIED — x https://a\n",
			[]string{"[1] VERIFIED — x https://a"},
		},
		"after a legend line": {
			"# Citations\n\n[V] = verified\n[1] VERIFIED — x https://a\n",
			[]string{"[1] VERIFIED — x https://a"},
		},
		"tight bulleted list": {
			"# Citations\n- [1] First https://a\n- [2] Second https://b\n* **[3]** Third https://c\n",
			[]string{"- [1] First https://a", "- [2] Second https://b", "* **[3]** Third https://c"},
		},
		"consecutive link reference definitions": {
			"# Citations\n\n[gh]: https://github.com/o/r\n[docs]: <https://o.example/docs/>\n[kb]: https://kb.example/1 \"KB 1\"\n",
			[]string{"[gh]: https://github.com/o/r", "[docs]: <https://o.example/docs/>", "[kb]: https://kb.example/1 \"KB 1\""},
		},
		"tight unbulleted list": {
			"# Citations\n\n[1] First Work. https://example.com/one\n[2] Second Work. https://example.com/two\n**[3]** Third Work. https://example.com/three\n",
			[]string{"[1] First Work. https://example.com/one", "[2] Second Work. https://example.com/two", "**[3]** Third Work. https://example.com/three"},
		},
		"tight list after a status-key paragraph": {
			"# Citations\n\nStatus key: **VERIFIED** = checked.\n[1] VERIFIED — x https://a\n[2] VERIFIED — y https://b\n",
			[]string{"[1] VERIFIED — x https://a", "[2] VERIFIED — y https://b"},
		},
		"tight list after an unkeyed bullet": {
			"# Citations\n- https://wiki.acme/handbook\n[1] VERIFIED — checked on-box.\n",
			[]string{"- https://wiki.acme/handbook", "[1] VERIFIED — checked on-box."},
		},
		// The layout of domains/dns-security-dnssec-caa.md: lettered
		// sub-entries on consecutive lines under a parent key, each wrapped.
		"lettered sub-entries under a parent key": {
			"# Citations\n\n[1] VERIFIED — DNSSEC RFCs.\n[1a] RFC 4033 (introduction —\n§2, verbatim). <https://www.rfc-editor.org/rfc/rfc4033.txt>\n" +
				"[1b] RFC 4034 (records\n§5.1, verbatim). <https://www.rfc-editor.org/rfc/rfc4034.txt>\n" +
				"[1c] RFC 4035 (validation).\n<https://www.rfc-editor.org/rfc/rfc4035.txt>\nNUANCED — naming is RFC 6781.\n" +
				"[2] VERIFIED — CAA, RFC 8659.\n<https://www.rfc-editor.org/rfc/rfc8659.txt>\n",
			[]string{
				"[1] VERIFIED — DNSSEC RFCs.",
				"[1a] RFC 4033 (introduction — §2, verbatim). <https://www.rfc-editor.org/rfc/rfc4033.txt>",
				"[1b] RFC 4034 (records §5.1, verbatim). <https://www.rfc-editor.org/rfc/rfc4034.txt>",
				"[1c] RFC 4035 (validation). <https://www.rfc-editor.org/rfc/rfc4035.txt> NUANCED — naming is RFC 6781.",
				"[2] VERIFIED — CAA, RFC 8659. <https://www.rfc-editor.org/rfc/rfc8659.txt>",
			},
		},
		// The layout of domains/email-authentication-spf-dkim-dmarc.md: a
		// wrapped parent entry, then its first lettered sub-entry.
		"first lettered sub-entry after a wrapped parent": {
			"# Citations\n\n[2] VERIFIED — DKIM, RFC 6376 (Signatures,\n**Standards Track**).\n[2a] RFC 6376: abstract. https://www.rfc-editor.org/rfc/rfc6376\n",
			[]string{"[2] VERIFIED — DKIM, RFC 6376 (Signatures, **Standards Track**).", "[2a] RFC 6376: abstract. https://www.rfc-editor.org/rfc/rfc6376"},
		},
	} {
		got := citationEntries(tc.body)
		if len(got) != len(tc.want) {
			t.Errorf("§5.1 %s: entries = %q, want %q", name, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("§5.1 %s: entry %d = %q, want %q", name, i, got[i], tc.want[i])
			}
		}
	}
}

// fallbackBundle is undeclared and carries a single-URL citation, a prose
// citation, and (when withTimestamp) a legacy timestamp.
func fallbackBundle(t *testing.T, prose, withTimestamp bool) string {
	fm := "---\ntype: Metric\n"
	if withTimestamp {
		fm += "timestamp: '2026-05-28T22:53:05+00:00'\n"
	}
	cites := "# Citations\n- [1] Handbook — https://wiki.acme/fpa\n"
	if prose {
		cites += "- [2] Personal observation, no public source.\n"
	}
	return mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"a.md":     fm + "---\n\n# Def\n\n" + cites,
	})
}

// TestConformance_MigrateHoldsDeclaration_Section12_1_WhileFallbackItemsRemain
// is the positive control for the hold: while a prose-citation (Appendix B
// `# Citations`) or missing-actor (Appendix B `timestamp`) item remains,
// declaring 0.3 would switch off the §12.1 fallback that still carries it, so
// the plan holds the declaration and apply leaves the bundle undeclared. The
// deterministic edits still land.
func TestConformance_MigrateHoldsDeclaration_Section12_1_WhileFallbackItemsRemain(t *testing.T) {
	for name, dir := range map[string]string{
		"prose-citation": fallbackBundle(t, true, false),
		"missing-actor":  fallbackBundle(t, false, true),
	} {
		b, plan := planFor(t, dir, "", false)
		if plan.VersionAction != VersionHold {
			t.Fatalf("%s: §12.1: version action = %q, want %q", name, plan.VersionAction, VersionHold)
		}
		if plan.DeclaresTarget() {
			t.Fatalf("%s: a held plan must not declare the target", name)
		}
		if err := MigrateApply(dir, b, plan); err != nil {
			t.Fatalf("%s: MigrateApply: %v", name, err)
		}
		after := loadMigrate(t, dir)
		if after.DeclaredOkfVersion != "" {
			t.Fatalf("%s: §12.1: held bundle declares %q; its fallback is now off", name, after.DeclaredOkfVersion)
		}
		if strings.Contains(readRel(t, dir, "index.md"), "okf_version") {
			t.Fatalf("%s: held bundle's index.md gained okf_version", name)
		}
		if got := after.Nodes["a.md"].Sources(); len(got) != 1 || got[0].Resource != "https://wiki.acme/fpa" {
			t.Fatalf("%s: deterministic source edit did not land: %+v", name, got)
		}
	}
}

// TestConformance_MigrateDeclares_Section8_1_WhenNoFallbackItemRemains is the
// negative control: with every fallback item resolved (no prose citation; an
// actor supplied for the timestamp), the plan bumps and apply declares 0.3.
func TestConformance_MigrateDeclares_Section8_1_WhenNoFallbackItemRemains(t *testing.T) {
	dir := fallbackBundle(t, false, true)
	b, plan := planFor(t, dir, "human:casey", false)
	if plan.VersionAction != VersionBump || !plan.DeclaresTarget() {
		t.Fatalf("§8.1: want bump, got %q (declares=%v); judgment %+v", plan.VersionAction, plan.DeclaresTarget(), plan.Judgment)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if v := loadMigrate(t, dir).DeclaredOkfVersion; v != "0.3" {
		t.Fatalf("§8.1: declared = %q, want 0.3", v)
	}
}

// TestConformance_MigrateForceDeclare_Section12_1 proves the explicit override:
// ForceDeclare declares 0.3 even with prose-citation items left, and the plan
// records that it did.
func TestConformance_MigrateForceDeclare_Section12_1(t *testing.T) {
	dir := fallbackBundle(t, true, false)
	b, plan := planFor(t, dir, "", true)
	if plan.VersionAction != VersionBump || !plan.ForceDeclare || !plan.DeclaresTarget() {
		t.Fatalf("forced plan: action %q force %v", plan.VersionAction, plan.ForceDeclare)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if v := loadMigrate(t, dir).DeclaredOkfVersion; v != "0.3" {
		t.Fatalf("forced: declared = %q, want 0.3", v)
	}
}

// TestConformance_MigrateApplySamePlanTwice_Section12_Idempotent re-applies one
// plan file to the tree it already migrated. Every splice re-checks the
// current bytes, so the second apply changes nothing: no duplicated sources
// entry, no rewritten generated mapping, no second link rewrite.
func TestConformance_MigrateApplySamePlanTwice_Section12_Idempotent(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"y.md":     "---\ntype: Concept\n---\n\n# Y\n",
		"sub/a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n" +
			"sources:\n  - resource: https://kept.example/\n---\n\n# Def\n\nSee [y](y.md).\n\n" +
			"# Citations\n- [1] Handbook — https://wiki.acme/fpa\n- [Y](y.md)\n",
	})
	b, plan := planFor(t, dir, "human:casey", false)
	if nm := findNodeMigration(t, plan, "sub/a.md"); len(nm.Sources) != 2 || nm.Generated == nil || len(nm.Paths) == 0 {
		t.Fatalf("precondition: want 2 sources, a rename, and a path edit; got %+v", nm)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, loadMigrate(t, dir), plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the same plan changed the tree:\n%s", readRel(t, dir, "sub/a.md"))
	}
	got := readRel(t, dir, "sub/a.md")
	if n := strings.Count(got, `resource: "https://wiki.acme/fpa"`); n != 1 {
		t.Fatalf("sources entry appears %d times, want 1:\n%s", n, got)
	}
}
