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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A `# Citations` section ends at the next heading of the same or a higher
// level. A deeper heading inside it (`## Tier 1`) is a group label within the
// list, so the entries under it are still citation entries (Appendix B,
// §12.1, §5.1).

const subheadedCitations = "# Def\n\nBody cites [1] and [2].\n\n" +
	"# Citations\n\n" +
	"Status key: VERIFIED = checked.\n\n" +
	"## Tier 1\n\n" +
	"- [1] VERIFIED — Alpha, https://a.example/1\n\n" +
	"## Tier 2\n\n" +
	"- [2] VERIFIED — Beta, https://b.example/2\n\n" +
	"# Next\n\n" +
	"- [3] not a citation, https://c.example/3\n"

// Positive control: entries under a deeper sub-heading are returned.
// Negative control: a following same-level heading still ends the section.
func TestConformance_CitationEntries_AppendixB_SubheadingStaysInSection(t *testing.T) {
	got := citationEntries(subheadedCitations)
	want := []string{
		"- [1] VERIFIED — Alpha, https://a.example/1",
		"- [2] VERIFIED — Beta, https://b.example/2",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("§5.1: entries = %q, want %q", got, want)
	}
	_, other := citationSection(subheadedCitations)
	for _, l := range []string{"## Tier 1", "## Tier 2"} {
		if !containsString(other, l) {
			t.Errorf("group label %q must be reported as a non-entry line, other = %q", l, other)
		}
	}
	for _, l := range other {
		if strings.Contains(l, "Next") {
			t.Errorf("same-level # Next must end the section, other = %q", other)
		}
	}
}

// The heading level is relative to the Citations heading: under
// `## Citations`, a `###` is a group label, and both `##` and `#` end it.
func TestConformance_CitationEntries_AppendixB_LevelIsRelative(t *testing.T) {
	for _, end := range []string{"## Next", "# Top"} {
		body := "## Citations\n\n### Group\n\n- [1] A https://a.example/1\n\n" +
			end + "\n\n- [2] B https://b.example/2\n"
		got := citationEntries(body)
		if len(got) != 1 || !strings.Contains(got[0], "[1]") {
			t.Errorf("ending %q: entries = %q, want only [1]", end, got)
		}
	}
}

// The removal span and the reference mask use the same end as the parser:
// they cover the sub-headed groups and stop at the same-level heading.
func TestConformance_CitationSectionSpan_AppendixB_SubheadingStaysInSection(t *testing.T) {
	s, e, ok := citationSectionSpan(subheadedCitations)
	if !ok {
		t.Fatal("no section found")
	}
	span := subheadedCitations[s:e]
	if !strings.Contains(span, "https://b.example/2") {
		t.Errorf("span stops at a sub-heading: %q", span)
	}
	if strings.Contains(span, "# Next") {
		t.Errorf("span runs past the same-level heading: %q", span)
	}
	masked := maskBody(subheadedCitations)
	if strings.Contains(masked, "b.example") {
		t.Errorf("mask leaves a sub-headed group unmasked:\n%s", masked)
	}
	if !strings.Contains(masked, "c.example") || !strings.Contains(masked, "Body cites [1]") {
		t.Errorf("mask covers text outside the section:\n%s", masked)
	}
}

// Regression: a `# Citations` heading followed only by sub-headed groups read
// as an empty, recoverable list, so removal cut the heading alone and left the
// entries orphaned under the previous section. The group label is a line
// removal would lose, so the list is kept and the heading stays (§4.2).
func TestConformance_MigrateRemoveCitations_Section4_2_HeadingNotCutFromGroups(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nText.\n\n# Related\n\n- [Other](/other.md)\n\n" +
		"# Citations\n\n## Probed\n\n- [P0] VERIFIED — proxy read, no URL.\n"
	dir, plan := migrateOnce(t, kb(node))
	if nm := planNodeMigration(plan, "a.md"); nm != nil && nm.RemoveCitations {
		t.Fatalf("§4.2: removal planned for a list whose entries sit under a group label")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# Citations\n\n## Probed") {
		t.Errorf("the # Citations heading must survive apply:\n%s", raw)
	}
}

// End to end: migrate converts the sub-headed entries to sources, and keeps
// the list (its group labels are not entries) with a citations-kept item.
func TestConformance_MigrateCitations_AppendixB_SubheadedEntriesConvert(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n" + subheadedCitations
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	res := map[string]bool{}
	for _, s := range nm.Sources {
		res[s.Resource] = true
	}
	for _, r := range []string{"https://a.example/1", "https://b.example/2"} {
		if !res[r] {
			t.Errorf("§5.1: entry for %s not converted; sources %+v", r, nm.Sources)
		}
	}
	if res["https://c.example/3"] {
		t.Errorf("an entry after the section was converted: %+v", nm.Sources)
	}
	if nm.RemoveCitations {
		t.Errorf("§4.2: a list with group labels must be kept, not removed")
	}
	if !hasJudgment(plan, "a.md", JudgmentCitationsKept) {
		t.Errorf("kept list must be reported as citations-kept: %+v", plan.Judgment)
	}
}
