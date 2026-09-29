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

// findingMessagesFor returns the messages of all findings whose Path == path.
func findingMessagesFor(fs []Finding, path string) []string {
	var out []string
	for _, f := range fs {
		if f.Path == path {
			out = append(out, f.Message)
		}
	}
	return out
}

// hasFindingMessage reports whether any finding for path carries exactly msg.
func hasFindingMessage(fs []Finding, path, msg string) bool {
	for _, m := range findingMessagesFor(fs, path) {
		if m == msg {
			return true
		}
	}
	return false
}

// validateBundleAt loads dir and returns its findings.
func validateBundleAt(t *testing.T, dir string) []Finding {
	t.Helper()
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%s): %v", dir, err)
	}
	return Validate(b)
}

// A minimal valid concept + index so a log-focused bundle has no unrelated
// findings. The root index carries the §12 okf_version carve-out.
func writeValidBundleShell(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n* [Note](notes/note.md) - a concept.\n")
	writeFile(t, dir, "notes/note.md", "---\ntype: Concept\ntitle: Note\n---\n\n# Note\n")
	writeFile(t, dir, ".okf", "okf_version: 0.2\n")
}

// --- AC-V1: date headings must be valid YYYY-MM-DD -------------------------

func TestValidate_LogDateHeadingMustBeISO(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\n## 2025-13-45\n* **Update**: Bad month and day.\n\n## last Tuesday\n* **Update**: Not a date.\n")

	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", `date heading "2025-13-45" is not a valid YYYY-MM-DD date (§9)`) {
		t.Errorf("V1: expected finding for 2025-13-45; got %v", findingMessagesFor(f, "log.md"))
	}
	if !hasFindingMessage(f, "log.md", `date heading "last Tuesday" is not a valid YYYY-MM-DD date (§9)`) {
		t.Errorf("V1: expected finding for `last Tuesday`; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogValidDateHeadingSilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\n## 2026-05-22\n* **Update**: A real entry.\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V1 control: a valid date heading must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

// --- AC-V2: consecutive date headings must be non-increasing ----------------

func TestValidate_LogAscendingDatesFail(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\n## 2024-01-01\n* older\n\n## 2026-01-01\n* newer, listed last.\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", "date headings are not newest first (§9): 2024-01-01 before 2026-01-01") {
		t.Errorf("V2: expected newest-first finding; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogEqualAdjacentDatesSilent(t *testing.T) {
	// The design/infra corpus duplicate-heading case: equal adjacent dates are
	// non-increasing and must stay silent.
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\n## 2026-05-22\n* one\n\n## 2026-05-22\n* two\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V2 control: equal adjacent dates must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogDescendingDatesSilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\n## 2026-05-22\n* a\n\n## 2026-05-15\n* b\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V2 control: descending dates must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

// --- AC-V3: a log with entries but no date headings -------------------------

func TestValidate_LogNoDateHeadingsFail(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n\nSome free-form prose, no dates.\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", "log has no ISO 8601 date headings (§9)") {
		t.Errorf("V3: expected no-headings finding; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogEmptyFileSilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V3 control: empty log must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogTitleOnlySilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Update Log\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V3 control: title-only log must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogScaffoldPlaceholderSilent(t *testing.T) {
	// The unmodified `bundle init` scaffold: header + placeholder line.
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V3 control: scaffold placeholder log must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

// --- AC-V4: log must begin with a `# ` title --------------------------------

func TestValidate_LogFrontmatterBearingFails(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "---\ntitle: Log\n---\n\n# Update Log\n\n## 2026-05-22\n* a\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", "log.md must begin with a `# ` title heading (§9); a frontmatter block is not permitted") {
		t.Errorf("V4: expected frontmatter-bearing finding; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogNoTitleFails(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "## 2026-05-22\n* a\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", "log.md must begin with a `# ` title heading (§9); a frontmatter block is not permitted") {
		t.Errorf("V4: expected missing-title finding; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogAllCorpusTitlesSilent(t *testing.T) {
	// All 7 distinct H1 titles observed in the real corpus must pass.
	titles := []string{
		"# Change Log",
		"# Directory Update Log",
		"# Log — Infra neighborhood",
		"# Log",
		"# Update Log",
		"# Casey — Change Log",
		"# Research — Change Log",
	}
	for _, title := range titles {
		dir := t.TempDir()
		writeValidBundleShell(t, dir)
		writeFile(t, dir, "log.md", title+"\n\n## 2026-05-22\n* a\n")
		if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
			t.Errorf("V4 control: title %q must be silent; got %v", title, findingMessagesFor(f, "log.md"))
		}
	}
}

// --- AC-V5: legacy dated bullets before the first date heading ---------------

func TestValidate_LogLegacyBulletsFail(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	// Mirrors the corpus root log: legacy `- YYYY-MM-DD — msg` bullets above any
	// date heading.
	writeFile(t, dir, "log.md", "# Change Log\n\n- 2026-09-23 — refreshed a.md\n- 2026-08-23 — refreshed b.md\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "log.md", "log has legacy dated bullets not grouped under date headings (§9); run `okfctl log append` to regroup") {
		t.Errorf("V5: expected legacy-bullets finding; got %v", findingMessagesFor(f, "log.md"))
	}
}

func TestValidate_LogRegroupedIsSilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "log.md", "# Change Log\n\n## 2026-09-23\n* refreshed a.md\n\n## 2026-08-23\n* refreshed b.md\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "log.md")) != 0 {
		t.Errorf("V5 control: a regrouped log must be silent; got %v", findingMessagesFor(f, "log.md"))
	}
}

// --- AC-V6: index empty-block gap -------------------------------------------

func TestValidate_NonRootIndexEmptyBlockFails(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "sub/index.md", "---\n---\n\n# Sub\n")
	f := validateBundleAt(t, dir)
	if !hasFindingMessage(f, "sub/index.md", "index files contain no frontmatter (§8); frontmatter is permitted only on the bundle-root index and only for okf_version (§12)") {
		t.Errorf("V6: non-root empty block must FAIL §8; got %v", findingMessagesFor(f, "sub/index.md"))
	}
}

func TestValidate_NonRootIndexNoBlockSilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir)
	writeFile(t, dir, "sub/index.md", "# Sub\n")
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "sub/index.md")) != 0 {
		t.Errorf("V6 control: non-root index with no block must be silent; got %v", findingMessagesFor(f, "sub/index.md"))
	}
}

func TestValidate_RootIndexOkfVersionOnlySilent(t *testing.T) {
	dir := t.TempDir()
	writeValidBundleShell(t, dir) // root index has okf_version-only block
	if f := validateBundleAt(t, dir); len(findingMessagesFor(f, "index.md")) != 0 {
		t.Errorf("V6 control: root index with okf_version only must be silent; got %v", findingMessagesFor(f, "index.md"))
	}
}

func TestValidate_RootIndexEmptyBlockFails(t *testing.T) {
	// A present-but-empty block on the root index: block present, okf_version
	// absent — a §12 violation per the existing root rule.
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\n---\n\n# Index\n")
	writeFile(t, dir, "notes/note.md", "---\ntype: Concept\ntitle: Note\n---\n\n# Note\n")
	f := validateBundleAt(t, dir)
	if len(findingMessagesFor(f, "index.md")) == 0 {
		t.Errorf("V6: root index with an empty block must FAIL (block present, okf_version absent); got no finding")
	}
}

// --- AC-V7: the exact #176 repro bundle -------------------------------------

func TestValidate_Issue176ReproBundle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n* [Note](notes/note.md) - a concept.\n")
	writeFile(t, dir, "notes/note.md", "---\ntype: Concept\ntitle: Note\n---\n\n# Note\n")
	writeFile(t, dir, "log.md", "# Update Log\n\nSome free-form prose, no dates.\n")
	writeFile(t, dir, "notes/log.md", "# Update Log\n\n## 2025-13-45\n* **Update**: Bad month and day.\n\n## last Tuesday\n* **Update**: Not a date.\n\n## 2024-01-01\n* **Creation**: Older.\n\n## 2026-01-01\n* **Update**: Newer, but listed last.\n")

	f := validateBundleAt(t, dir)
	want := []struct{ path, msg string }{
		{"log.md", "log has no ISO 8601 date headings (§9)"},
		{"notes/log.md", `date heading "2025-13-45" is not a valid YYYY-MM-DD date (§9)`},
		{"notes/log.md", `date heading "last Tuesday" is not a valid YYYY-MM-DD date (§9)`},
		{"notes/log.md", "date headings are not newest first (§9): 2024-01-01 before 2026-01-01"},
	}
	for _, w := range want {
		if !hasFindingMessage(f, w.path, w.msg) {
			t.Errorf("V7: missing finding %s: %q; got all: %v", w.path, w.msg, f)
		}
	}
	// Exit non-zero == at least one finding.
	if len(f) == 0 {
		t.Fatalf("V7: repro bundle must produce findings")
	}
	// Sanity: the messages we assert really are the log ones (not swallowed by
	// unrelated findings). At minimum the four §9 findings must be present.
	joined := ""
	for _, ff := range f {
		joined += ff.Path + ": " + ff.Message + "\n"
	}
	if !strings.Contains(joined, "§9") {
		t.Errorf("V7: expected §9 findings in output:\n%s", joined)
	}
}
