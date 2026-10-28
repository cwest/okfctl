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

// toCRLF rewrites every index.md under dir to CRLF, as a Windows checkout with
// core.autocrlf=true does (see issue #185). It is the test-side equivalent of
// the issue's `find kb -name index.md -exec perl -pi -e 's/\r?\n/\r\n/'`.
func toCRLF(t *testing.T, dir string) {
	t.Helper()
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(p) != "index.md" {
			return nil
		}
		b, rerr := os.ReadFile(p) //nolint:gosec // G304: test fixture under t.TempDir()
		if rerr != nil {
			return rerr
		}
		// Normalize to LF first, then to CRLF, so the result is pure CRLF
		// regardless of the input's existing endings.
		lf := strings.ReplaceAll(string(b), "\r\n", "\n")
		crlf := strings.ReplaceAll(lf, "\n", "\r\n")
		return os.WriteFile(p, []byte(crlf), 0o644) //nolint:gosec // G306: test fixture
	})
	if err != nil {
		t.Fatal(err)
	}
}

// readFile is a small test helper that returns a file's bytes as a string.
func readFileStr(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p) //nolint:gosec // G304: test fixture under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestUsesCRLF_Table pins the dominant-ending detector: a file is CRLF only when
// EVERY line break is CRLF. A mixed-ending file, an all-LF file, and a file with
// no line break at all are NOT CRLF (new/mixed indexes stay LF on write).
func TestUsesCRLF_Table(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"pure LF", "a\nb\nc\n", false},
		{"pure CRLF", "a\r\nb\r\nc\r\n", true},
		{"mixed mostly CRLF one LF", "a\r\nb\nc\r\n", false},
		{"mixed mostly LF one CRLF", "a\nb\r\nc\n", false},
		{"no newline", "abc", false},
		{"empty", "", false},
		{"single CRLF line", "a\r\n", true},
		{"single LF line", "a\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := usesCRLF(c.in); got != c.want {
				t.Errorf("usesCRLF(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestNormalizeEOL folds CRLF to LF and leaves LF-only content untouched.
func TestNormalizeEOL(t *testing.T) {
	if got := normalizeEOL("a\r\nb\r\n"); got != "a\nb\n" {
		t.Errorf("normalizeEOL CRLF: got %q", got)
	}
	if got := normalizeEOL("a\nb\n"); got != "a\nb\n" {
		t.Errorf("normalizeEOL LF: got %q", got)
	}
	// A lone CR not followed by LF is left alone (we only fold CRLF).
	if got := normalizeEOL("a\rb"); got != "a\rb" {
		t.Errorf("normalizeEOL lone CR: got %q", got)
	}
}

// TestIndexInSync_CRLFIndexIsFresh is seam (b): a CRLF index whose normalized
// content matches the render reports IN SYNC. Issue #185: `index check` compares
// raw bytes to the LF render, so a line-ending-only difference counted as drift
// and Windows teams could not use it as a gate.
func TestIndexInSync_CRLFIndexIsFresh(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	// Build the canonical LF tree, then convert to CRLF (the Windows checkout).
	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	toCRLF(t, dir)

	b2, _ := Load(dir)
	if ok, diff := IndexInSync(b2); !ok {
		t.Errorf("a CRLF index matching the render must be IN SYNC (content-only compare); diff:\n%s", diff)
	}
}

// TestIndexInSync_CRLFGenuineDriftStillCaught is the NEGATIVE control for seam
// (b): normalizing line endings must not mask REAL content drift. A CRLF index
// that is genuinely stale (a node added after the build) must still report out
// of sync.
func TestIndexInSync_CRLFGenuineDriftStillCaught(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	toCRLF(t, dir)

	// Real content change AFTER the index was written: the CRLF index is now
	// genuinely stale, not merely a different line ending.
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")

	b2, _ := Load(dir)
	if ok, _ := IndexInSync(b2); ok {
		t.Error("a CRLF index with genuinely stale content must still be OUT OF SYNC")
	}
}

// TestWriteIndex_PreservesCRLF is seam (c), positive control: `index build` on
// an up-to-date CRLF checkout leaves every index.md BYTE-IDENTICAL (still CRLF),
// so git with autocrlf=false (or CRLF committed) sees no churn.
func TestWriteIndex_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	toCRLF(t, dir)

	rootP := filepath.Join(dir, "index.md")
	wineP := filepath.Join(dir, "wine", "index.md")
	beforeRoot := readFileStr(t, rootP)
	beforeWine := readFileStr(t, wineP)
	if !strings.Contains(beforeRoot, "\r\n") {
		t.Fatal("fixture precondition: root index should be CRLF after toCRLF")
	}

	// Rebuild from the CRLF checkout: content is unchanged, so the files must
	// stay byte-identical and CRLF.
	b2, _ := Load(dir)
	if err := WriteIndex(b2); err != nil {
		t.Fatal(err)
	}
	if got := readFileStr(t, rootP); got != beforeRoot {
		t.Errorf("root index.md must be byte-identical after rebuild on a CRLF checkout\nbefore: %q\nafter:  %q", beforeRoot, got)
	}
	if got := readFileStr(t, wineP); got != beforeWine {
		t.Errorf("wine/index.md must be byte-identical after rebuild on a CRLF checkout\nbefore: %q\nafter:  %q", beforeWine, got)
	}
}

// TestWriteIndex_PreservesLF is seam (c), the LF arm: an up-to-date LF index
// stays byte-identical and LF after a rebuild (no CRLF leaks in).
func TestWriteIndex_PreservesLF(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	rootP := filepath.Join(dir, "index.md")
	before := readFileStr(t, rootP)
	if strings.Contains(before, "\r\n") {
		t.Fatal("fixture precondition: a freshly built index should be LF")
	}

	b2, _ := Load(dir)
	if err := WriteIndex(b2); err != nil {
		t.Fatal(err)
	}
	after := readFileStr(t, rootP)
	if after != before {
		t.Errorf("LF index.md must be byte-identical after rebuild\nbefore: %q\nafter:  %q", before, after)
	}
	if strings.Contains(after, "\r\n") {
		t.Errorf("an LF index must not gain CRLF endings on rebuild; got %q", after)
	}
}

// TestWriteIndex_NewIndexIsLF: a directory that gains its first index.md on a
// build writes LF (new files stay LF; only an EXISTING CRLF file is preserved).
func TestWriteIndex_NewIndexIsLF(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	wineP := filepath.Join(dir, "wine", "index.md")
	got := readFileStr(t, wineP)
	if got == "" {
		t.Fatal("expected a nested wine/index.md to be written")
	}
	if strings.Contains(got, "\r\n") {
		t.Errorf("a newly written index.md must be LF; got %q", got)
	}
}

// TestWriteIndex_CuratorSuffixIdempotentOnCRLF is seam (a), the data-damage bug:
// a curator subdirectory description in a CRLF index must round-trip through
// `index build` with the tool-owned shape suffix appearing EXACTLY ONCE. Before
// the fix, the `$`-anchored suffix regex fails on a trailing \r, the stale
// suffix is captured into the preserved description, and build appends a fresh
// one: `... - Audit trail rules. (1 concept · reference) (1 concept · reference)`.
func TestWriteIndex_CuratorSuffixIdempotentOnCRLF(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "audit/rule.md", "reference", "Rule")

	// Build the canonical index, then give the audit subdirectory entry a
	// curator description (the §8 curator-owned text before the shape suffix).
	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	rootP := filepath.Join(dir, "index.md")
	rendered := readFileStr(t, rootP)

	// Insert a curator description into the audit bullet: turn
	//   * [Audit](audit/) (N concept · ...)
	// into
	//   * [Audit](audit/) - Audit trail rules. (N concept · ...)
	const linkMarker = "](audit/)"
	i := strings.Index(rendered, linkMarker)
	if i < 0 {
		t.Fatalf("expected an audit subdirectory bullet in the root index; got:\n%s", rendered)
	}
	insertAt := i + len(linkMarker)
	curated := rendered[:insertAt] + " - Audit trail rules." + rendered[insertAt:]
	if err := os.WriteFile(rootP, []byte(curated), 0o644); err != nil { //nolint:gosec // G306: test fixture
		t.Fatal(err)
	}

	// A build on the LF curated index is idempotent and the suffix appears once.
	bLF, _ := Load(dir)
	if err := WriteIndex(bLF); err != nil {
		t.Fatal(err)
	}
	lfResult := readFileStr(t, rootP)
	if n := strings.Count(lfResult, "(1 concept"); n != 1 {
		t.Fatalf("LF precondition: shape suffix should appear once, got %d:\n%s", n, lfResult)
	}
	if !strings.Contains(lfResult, "- Audit trail rules.") {
		t.Fatalf("LF precondition: curator description should be preserved:\n%s", lfResult)
	}

	// Now convert to CRLF (the Windows checkout) and rebuild. The suffix must
	// STILL appear exactly once and the curator description survive verbatim.
	toCRLF(t, dir)
	bCRLF, _ := Load(dir)
	if err := WriteIndex(bCRLF); err != nil {
		t.Fatal(err)
	}
	crlfResult := readFileStr(t, rootP)
	if n := strings.Count(crlfResult, "(1 concept"); n != 1 {
		t.Errorf("shape suffix must appear EXACTLY ONCE after a CRLF rebuild, got %d:\n%q", n, crlfResult)
	}
	if !strings.Contains(crlfResult, "- Audit trail rules.") {
		t.Errorf("curator description must survive a CRLF rebuild:\n%q", crlfResult)
	}
	if !strings.Contains(crlfResult, "\r\n") {
		t.Errorf("an up-to-date CRLF index must stay CRLF after rebuild:\n%q", crlfResult)
	}
}
