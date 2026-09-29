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

// preambleBetweenTitleAndFirstHeading returns the non-blank lines that sit
// between the `# ` title and the first `## ` date heading. For a §9-conformant
// log this must be empty after a regroup: a legacy bullet's continuation block
// (indented tables, multi-line bodies) must travel WITH the bullet into its date
// group, not be stranded above the first heading.
func preambleBetweenTitleAndFirstHeading(body string) []string {
	var orphans []string
	seenTitle := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if strings.HasPrefix(line, "# ") {
			seenTitle = true
			continue
		}
		if !seenTitle {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		orphans = append(orphans, line)
	}
	return orphans
}

// A legacy `- YYYY-MM-DD — msg` bullet whose body continues over several indented
// lines (a ledger table under a write-time-sweep entry, mirroring the real root
// log.md) must be regrouped WITH its continuation block: the whole entry lands
// under that date's heading, nothing is stranded above the first heading. This is
// the continuation-orphaning regression.
func TestAppendLog_RegroupCarriesContinuationBlock(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := strings.Join([]string{
		"# Change Log",
		"",
		"- 2026-08-22 — Write-time curation sweep. Generative-pass ledger:",
		"  | node | commission? | split? | reason |",
		"  | ---- | ----------- | ------ | ------ |",
		"  | alpha | no | no | already covered |",
		"  | beta | yes | no | adjacent gap |",
		"- 2026-08-04 — legacy four (single line)",
		"",
		"## 2026-07-01",
		"* old",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "new entry"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)

	// Nothing is stranded between the title and the first date heading.
	if orphans := preambleBetweenTitleAndFirstHeading(got); len(orphans) != 0 {
		t.Errorf("continuation orphaning: %d non-blank line(s) stranded above the first heading:\n%q\nfull:\n%s",
			len(orphans), orphans, got)
	}

	// The table body lands under the 2026-08-22 group, attached to its bullet.
	sweepGroup := sectionFor(got, "2026-08-22")
	for _, want := range []string{
		"Write-time curation sweep",
		"| node | commission? | split? | reason |",
		"| alpha | no | no | already covered |",
		"| beta | yes | no | adjacent gap |",
	} {
		if !strings.Contains(sweepGroup, want) {
			t.Errorf("continuation block not under 2026-08-22 group; missing %q in:\n%s", want, sweepGroup)
		}
	}

	// The whole log is valid §9 and newest-first.
	assertBodyValidSection9(t, got)
	assertHeadingOrder(t, got, []string{"2026-09-29", "2026-08-22", "2026-08-04", "2026-07-01"})
}

// Control: a log whose legacy bullets are ALL single-line regroups byte-identically
// to the pre-fix output — carrying continuation blocks must not perturb the common
// case. We assert the single-line legacy bullet renders as a plain `* msg` entry
// with no injected blank/continuation lines.
func TestAppendLog_RegroupSingleLineBulletsUnchanged(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := strings.Join([]string{
		"# Change Log",
		"",
		"- 2026-08-08 — a",
		"- 2026-08-04 — b",
		"",
		"## 2026-07-01",
		"* old",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "new entry"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)

	want := strings.Join([]string{
		"# Change Log",
		"",
		"## 2026-09-29",
		"* new entry",
		"",
		"## 2026-08-08",
		"* a",
		"",
		"## 2026-08-04",
		"* b",
		"",
		"## 2026-07-01",
		"* old",
		"",
	}, "\n")
	if got != want {
		t.Errorf("single-line regroup not byte-identical to expected:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
