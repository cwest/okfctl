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

// statusDoc builds a concept node with an explicit status frontmatter value.
func statusDoc(title, status string) string {
	return "---\ntype: Concept\ntitle: " + title + "\nstatus: " + status +
		"\n---\n\n# " + title + "\n\nBody.\n"
}

// --- status lifecycle enum (§5.4) ------------------------------------------

// TestLint_Status_FlagsNonEnumValue is the POSITIVE control: a status value
// outside the §5.4 lifecycle enum (draft|stable|deprecated) — here the old
// conflated grade "verified" left in status instead of moved to epistemic — is a
// status-lifecycle finding. This is the exact defect the migration's status/
// epistemic split guards against, which the v0.1 tool could not see (§11 let the
// value through as arbitrary text).
func TestLint_Status_FlagsNonEnumValue(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     statusDoc("A", "verified"),
	})
	fs := findingsFor(Lint(b, LintOptions{}), "status-lifecycle")
	if len(fs) != 1 || fs[0].Path != "a.md" {
		t.Fatalf("want one status-lifecycle finding on a.md, got %+v", fs)
	}
}

// TestLint_Status_EnumValuesSilent is the NEGATIVE control: each legitimate §5.4
// enum value (draft, stable, deprecated) is silent. This is the load-bearing
// control — a check that fires on a legit lifecycle value would be worse than no
// check at all.
func TestLint_Status_EnumValuesSilent(t *testing.T) {
	for _, v := range []string{"draft", "stable", "deprecated"} {
		b := mkLintBundle(t, map[string]string{
			"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
			"a.md":     statusDoc("A", v),
		})
		if fs := findingsFor(Lint(b, LintOptions{}), "status-lifecycle"); len(fs) != 0 {
			t.Errorf("status=%q is a valid §5.4 enum value; want silent, got %+v", v, fs)
		}
	}
}

// TestLint_Status_AbsentIsStableSilent is a NEGATIVE control for the §5.4
// "absent ⇒ stable" rule: a node with no status key is not a finding.
func TestLint_Status_AbsentIsStableSilent(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "No status key at all."),
	})
	if fs := findingsFor(Lint(b, LintOptions{}), "status-lifecycle"); len(fs) != 0 {
		t.Fatalf("absent status ⇒ stable (§5.4); want silent, got %+v", fs)
	}
}

// --- per-node okf_spec_version resurrection (§12) --------------------------

// TestLint_SpecVersion_FlagsResurrectedPerNode is the POSITIVE control: the
// migration deduped the per-node okf_spec_version up to the single bundle-level
// okf_version on index.md (§12). A per-node okf_spec_version that reappears
// invites drift with its own bundle, so it is flagged.
func TestLint_SpecVersion_FlagsResurrectedPerNode(t *testing.T) {
	doc := "---\ntype: Concept\ntitle: A\nokf_spec_version: \"0.2\"\n---\n\n# A\n\nBody.\n"
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc,
	})
	fs := findingsFor(Lint(b, LintOptions{}), "spec-version")
	if len(fs) != 1 || fs[0].Path != "a.md" {
		t.Fatalf("want one spec-version finding on a.md, got %+v", fs)
	}
}

// TestLint_SpecVersion_CleanCorpusSilent is the NEGATIVE control: a node with no
// per-node okf_spec_version (the post-migration shape) is silent, and the
// bundle-level okf_version on index.md is never itself flagged.
func TestLint_SpecVersion_CleanCorpusSilent(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "Body, no per-node version."),
	})
	if fs := findingsFor(Lint(b, LintOptions{}), "spec-version"); len(fs) != 0 {
		t.Fatalf("post-migration node carries no okf_spec_version; want silent, got %+v", fs)
	}
}
