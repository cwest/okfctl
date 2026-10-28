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

// writeLogCRLF seeds a log.md whose every line break is CRLF, as a Windows
// checkout with core.autocrlf=true produces (issue #185, now applied to log.md:
// the review of PR #205 found log.md read paths still compared raw bytes). The
// input is normalized to LF first, then to CRLF, so the result is pure CRLF
// regardless of the literal's endings.
func writeLogCRLF(t *testing.T, dir, body string) {
	t.Helper()
	lf := strings.ReplaceAll(body, "\r\n", "\n")
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(crlf), 0o644); err != nil { //nolint:gosec // G306: test fixture under t.TempDir()
		t.Fatal(err)
	}
}

// TestAppendLog_CRLFPreservedNoCorruption is the log analogue of PR #205's seam
// (c), positive control: appending to an up-to-date CRLF log.md writes the whole
// file back in CRLF — no mixed-ending corruption. Before the fix, parseLog split
// on "\n" so every preserved line kept a trailing \r while the newly rendered
// group/entry was pure LF, yielding a mangled mixed-ending file
// ("# Change Log\r\n\n## today\n* new\n...\r\n") and whole-file git churn on
// every append.
func TestAppendLog_CRLFPreservedNoCorruption(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	writeLogCRLF(t, dir, "# Change Log\n\n## 2026-05-22\n* Added a table reference.\n")

	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)

	// Every line break must be CRLF — no stray LF left where the render inserted
	// new structure, and no stray \r left mid-content.
	if !usesCRLF(got) {
		t.Errorf("appending to a CRLF log must write the whole file back CRLF (no mixed endings); got %q", got)
	}
	// Normalized, the content must be exactly the §9 LF render — the new entry on
	// top of today's group, the old group preserved, nothing shredded.
	want := "# Change Log\n\n## 2026-09-29\n* Fixed a link\n\n## 2026-05-22\n* Added a table reference.\n"
	if gotLF := normalizeEOL(got); gotLF != want {
		t.Errorf("CRLF log content mismatch after append\n--- got (normalized) ---\n%q\n--- want ---\n%q", gotLF, want)
	}
}

// TestAppendLog_LFStaysLF is the LF arm (seam c): appending to an LF log leaves
// it LF — the CRLF-preservation path must not leak CRLF into an LF file.
func TestAppendLog_LFStaysLF(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := "# Change Log\n\n## 2026-05-22\n* Added a table reference.\n"
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)
	if strings.Contains(got, "\r\n") {
		t.Errorf("an LF log must not gain CRLF endings on append; got %q", got)
	}
	want := "# Change Log\n\n## 2026-09-29\n* Fixed a link\n\n## 2026-05-22\n* Added a table reference.\n"
	if got != want {
		t.Errorf("LF log mismatch after append\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestAppendLog_NewLogIsLF: a brand-new log.md (no file on disk) is written LF —
// only an EXISTING CRLF file is preserved; a fresh file stays LF regardless of
// platform (deterministic from the one file).
func TestAppendLog_NewLogIsLF(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	if err := AppendLog(dir, "first"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)
	if strings.Contains(got, "\r\n") {
		t.Errorf("a newly created log.md must be LF; got %q", got)
	}
}

// TestAppendLog_CRLFLegacyFoldStillWorks is the NEGATIVE control / genuine-drift
// arm: normalizing line endings on read must not mask the real work AppendLog
// does. A CRLF log carrying a legacy `- YYYY-MM-DD — msg` bullet (the pre-§9
// inline-dated form) must still regroup it into its date group AND stay CRLF —
// proving the read normalization feeds the parser clean lines (the legacy regex
// is `$`-anchored and would miss a trailing \r) without suppressing the fold.
func TestAppendLog_CRLFLegacyFoldStillWorks(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	writeLogCRLF(t, dir, "# Change Log\n\n- 2026-05-22 — Added a table reference.\n")

	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)
	norm := normalizeEOL(got)

	// The legacy bullet must be folded under a `## 2026-05-22` heading as a `* `
	// entry — if the trailing \r had leaked into the parser, the legacy regex
	// would not match and the bullet would be stranded in the preamble.
	if !strings.Contains(norm, "## 2026-05-22\n* Added a table reference.") {
		t.Errorf("legacy bullet must be regrouped under its date heading (read normalization must feed the parser clean lines); got:\n%s", norm)
	}
	// The new entry lands under today's group, newest-first.
	if !strings.Contains(norm, "## 2026-09-29\n* Fixed a link") {
		t.Errorf("new entry must land under today's group; got:\n%s", norm)
	}
	// Full normalized content must be EXACTLY the clean §9 render — no \r leaked
	// into the folded message. This is the dedicated normalize-on-read control:
	// without the read fold, parseLog captures "Added a table reference.\r" into
	// the entry (the legacy regex's `.` matches the trailing \r), and this exact
	// comparison fails even though the substring checks above still pass.
	want := "# Change Log\n\n## 2026-09-29\n* Fixed a link\n\n## 2026-05-22\n* Added a table reference.\n"
	if norm != want {
		t.Errorf("legacy-fold content must be the clean §9 render (no \\r in the folded message)\n--- got (normalized) ---\n%q\n--- want ---\n%q", norm, want)
	}
	// And the file stays CRLF (preserve on write).
	if !usesCRLF(got) {
		t.Errorf("a CRLF log must stay CRLF after a legacy-fold append; got %q", got)
	}
}

// TestAppendLog_LoneCRNotMangled is a boundary negative control: a lone CR that
// is NOT part of a CRLF pair (e.g. an old-Mac classic-CR file, or a CR inside a
// line) is left alone by normalizeEOL — only \r\n is folded — and the file, being
// not dominantly CRLF, is written LF. This guards against an over-eager fold that
// would eat legitimate carriage returns.
func TestAppendLog_LoneCRNotMangled(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	// A log whose entry text contains a bare \r (not a CRLF line break). Written
	// with LF line breaks so the file is NOT dominantly CRLF.
	seed := "# Change Log\n\n## 2026-05-22\n* Added a\rtable reference.\n"
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)
	// The lone CR inside the preserved entry survives (we only fold CRLF pairs).
	if !strings.Contains(got, "Added a\rtable reference.") {
		t.Errorf("a lone CR inside entry text must be preserved verbatim; got %q", got)
	}
	// The file is not dominantly CRLF, so it is written LF.
	if strings.Contains(got, "\r\n") {
		t.Errorf("a non-CRLF log must be written LF; got %q", got)
	}
}
