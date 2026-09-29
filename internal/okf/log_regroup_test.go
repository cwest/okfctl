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

// countHeadings returns how many `## <date>` headings for the given date appear.
func countHeadings(body, date string) int {
	return countOccurrences(body, "## "+date+"\n")
}

// AC-L1: on write, legacy `- YYYY-MM-DD — msg` bullets are regrouped into
// `* msg` under their date heading, MERGING into an existing `## date` group when
// present. Mirrors the research/log.md 2026-08-04 / 2026-08-08 collision.
func TestAppendLog_RegroupsLegacyMergingIntoExistingGroup(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	// Legacy bullets above headings; 2026-08-04 collides with an existing group.
	seed := strings.Join([]string{
		"# Research — Change Log",
		"",
		"- 2026-08-08 — legacy eight",
		"- 2026-08-04 — legacy four",
		"- 2026-08-16 — legacy sixteen",
		"",
		"## 2026-08-04",
		"* **Update**: existing four",
		"",
		"## 2026-06-22",
		"* **Init**: existing.",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "new entry"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)

	// No duplicate heading created for the colliding date.
	if n := countHeadings(got, "2026-08-04"); n != 1 {
		t.Errorf("AC-L1: 2026-08-04 heading must appear once (merge, not duplicate); got %d in:\n%s", n, got)
	}
	// The colliding group merges: existing entry, then the legacy one (order
	// preserved within the group).
	fourGroup := sectionFor(got, "2026-08-04")
	if !strings.Contains(fourGroup, "existing four") || !strings.Contains(fourGroup, "legacy four") {
		t.Errorf("AC-L1: merged 2026-08-04 group missing an entry:\n%s", fourGroup)
	}
	if strings.Index(fourGroup, "existing four") > strings.Index(fourGroup, "legacy four") {
		t.Errorf("AC-L1: within-group order not preserved (existing before legacy):\n%s", fourGroup)
	}
	// New date groups created for non-colliding legacy dates.
	if countHeadings(got, "2026-08-08") != 1 || countHeadings(got, "2026-08-16") != 1 {
		t.Errorf("AC-L1: non-colliding legacy dates must get their own group:\n%s", got)
	}
	// The whole log is newest-first and valid §9.
	assertBodyValidSection9(t, got)
	// Newest-first: 2026-09-29 (today), then 08-16, 08-08, 08-04, 06-22.
	order := []string{"2026-09-29", "2026-08-16", "2026-08-08", "2026-08-04", "2026-06-22"}
	assertHeadingOrder(t, got, order)
}

// AC-L2: regrouping is idempotent (a second append changes only the new entry)
// and lossless (entry count before == after − 1).
func TestAppendLog_RegroupIdempotentAndLossless(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := strings.Join([]string{
		"# Change Log",
		"",
		"- 2026-08-08 — a",
		"- 2026-08-04 — b",
		"- 2026-08-04 — c",
		"",
		"## 2026-07-01",
		"* old",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	// Entry count in the seed: 3 legacy + 1 existing = 4.
	seedEntries := countEntries(seed)
	if seedEntries != 4 {
		t.Fatalf("seed entry count sanity: got %d, want 4", seedEntries)
	}

	if err := AppendLog(dir, "first append"); err != nil {
		t.Fatal(err)
	}
	after1 := readLogFile(t, dir)
	if got := countEntries(after1); got != seedEntries+1 {
		t.Errorf("AC-L2 lossless: after one append want %d entries, got %d in:\n%s", seedEntries+1, got, after1)
	}
	assertBodyValidSection9(t, after1)

	if err := AppendLog(dir, "second append"); err != nil {
		t.Fatal(err)
	}
	after2 := readLogFile(t, dir)

	// Idempotent: the only difference between after1 and after2 is the new entry.
	// Removing the new `* second append` line from after2 should yield after1.
	reduced := strings.Replace(after2, "* second append\n", "", 1)
	if reduced != after1 {
		t.Errorf("AC-L2 idempotent: a second append must change ONLY the new entry\n--- after1 ---\n%s\n--- after2 (minus new entry) ---\n%s", after1, reduced)
	}
	if got := countEntries(after2); got != seedEntries+2 {
		t.Errorf("AC-L2 lossless: after two appends want %d entries, got %d", seedEntries+2, got)
	}
}

// sectionFor returns the text of the `## date` section (heading through the line
// before the next heading or EOF).
func sectionFor(body, date string) string {
	lines := strings.Split(body, "\n")
	var out []string
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			if strings.TrimSpace(strings.TrimPrefix(line, "## ")) == date {
				in = true
				out = append(out, line)
			}
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// countEntries counts `* ` and legacy `- YYYY-MM-DD — ` entry lines.
func countEntries(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "* ") || legacyDatedBulletRe.MatchString(line) {
			n++
		}
	}
	return n
}

func assertHeadingOrder(t *testing.T, body string, want []string) {
	t.Helper()
	var got []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			got = append(got, strings.TrimSpace(strings.TrimPrefix(line, "## ")))
		}
	}
	if len(got) != len(want) {
		t.Errorf("heading order: got %v, want %v", got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("heading order: got %v, want %v", got, want)
			return
		}
	}
}

// assertBodyValidSection9 loads a synthetic bundle whose root log.md is body and
// asserts the PRODUCTION validator finds no §9 issue with it.
func assertBodyValidSection9(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "log.md", body)
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fs := validateReservedLog("log.md", b.Reserved["log.md"]); len(fs) != 0 {
		t.Errorf("body must be valid §9; got findings %v for:\n%s", fs, body)
	}
}
