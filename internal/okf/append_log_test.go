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
	"testing"
	"time"
)

// withLogClock pins AppendLog's clock to a fixed UTC day for deterministic
// exact-output assertions, restoring the real clock on cleanup.
func withLogClock(t *testing.T, day string) {
	t.Helper()
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatalf("bad test day %q: %v", day, err)
	}
	prev := nowUTC
	nowUTC = func() time.Time { return d }
	t.Cleanup(func() { nowUTC = prev })
}

func readLogFile(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// AC-A1: AppendLog keeps everything before the first `## ` heading verbatim,
// including an H1 that is not `# Change Log`.
func TestAppendLog_PreservesNonChangeLogTitle(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := "# Directory Update Log\n\n## 2026-05-22\n* **Update**: Added a table reference.\n\n## 2026-05-15\n* **Initialization**: Created foundational directory structure.\n"
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Added a playbook"); err != nil {
		t.Fatal(err)
	}
	got := readLogFile(t, dir)
	if want := "# Directory Update Log\n"; got[:len(want)] != want {
		t.Errorf("AC-A1: title not preserved; got head:\n%q", got[:min(len(got), 60)])
	}
	// The original title must appear exactly once (no stacked `# Change Log`).
	if n := countOccurrences(got, "# Directory Update Log"); n != 1 {
		t.Errorf("AC-A1: expected original title once, got %d in:\n%s", n, got)
	}
	if countOccurrences(got, "# Change Log") != 0 {
		t.Errorf("AC-A1: must not stack a `# Change Log` header in:\n%s", got)
	}
}

// AC-A2 / AC-A3: the #177 step-2 repro produces exactly the expected output.
func TestAppendLog_Issue177Step2ExactOutput(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	seed := "# Directory Update Log\n\n## 2026-05-22\n* **Update**: Added a table reference.\n\n## 2026-05-15\n* **Initialization**: Created foundational directory structure.\n"
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Added a playbook"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	want := "# Directory Update Log\n\n## 2026-09-29\n* Fixed a link\n* Added a playbook\n\n## 2026-05-22\n* **Update**: Added a table reference.\n\n## 2026-05-15\n* **Initialization**: Created foundational directory structure.\n"
	if got := readLogFile(t, dir); got != want {
		t.Errorf("AC-A3 step-2 exact mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// AC-A3 step-3: fresh bundle init + two appends yields `# Change Log` + one dated
// group, newest entry first, no placeholder left behind.
func TestAppendLog_Issue177Step3FreshInit(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Added a playbook"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "Fixed a link"); err != nil {
		t.Fatal(err)
	}
	want := "# Change Log\n\n## 2026-09-29\n* Fixed a link\n* Added a playbook\n"
	if got := readLogFile(t, dir); got != want {
		t.Errorf("AC-A3 step-3 exact mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// AC-A2: when the newest heading is NOT today, a new `## today` group is inserted
// above the first heading. Covered by step-2. When it IS today, entries stack in
// that group.
func TestAppendLog_SameDayStacksInGroup(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "one"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "two"); err != nil {
		t.Fatal(err)
	}
	want := "# Change Log\n\n## 2026-09-29\n* two\n* one\n"
	if got := readLogFile(t, dir); got != want {
		t.Errorf("AC-A2 same-day mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// AppendLog creates a §9-shaped log when the file is absent.
func TestAppendLog_CreatesSection9Log(t *testing.T) {
	withLogClock(t, "2026-09-29")
	dir := t.TempDir()
	if err := AppendLog(dir, "first"); err != nil {
		t.Fatal(err)
	}
	want := "# Change Log\n\n## 2026-09-29\n* first\n"
	if got := readLogFile(t, dir); got != want {
		t.Errorf("create mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
