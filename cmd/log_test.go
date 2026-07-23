// Copyright 2026 Casey West
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

package cmd

import (
	"strings"
	"testing"
)

func TestLogAppendThenShow(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "log", "append", dir, "--message", "added tannin node"); err != nil {
		t.Fatalf("log append: %v", err)
	}
	out, err := runOKF(t, "log", "show", dir)
	if err != nil {
		t.Fatalf("log show: %v", err)
	}
	if !strings.Contains(out, "added tannin node") {
		t.Errorf("log show missing the appended entry; got:\n%s", out)
	}
	// The scaffold seeds log.md with an empty-log placeholder; the first real
	// append must consume it, not pin it below every entry.
	if strings.Contains(out, "No entries yet") {
		t.Errorf("log show still shows the scaffold placeholder after an append; got:\n%s", out)
	}
}

func TestLogAppendTwice_NewestFirstNoPlaceholder(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "log", "append", dir, "--message", "first entry"); err != nil {
		t.Fatalf("log append 1: %v", err)
	}
	if _, err := runOKF(t, "log", "append", dir, "--message", "second entry"); err != nil {
		t.Fatalf("log append 2: %v", err)
	}
	out, err := runOKF(t, "log", "show", dir)
	if err != nil {
		t.Fatalf("log show: %v", err)
	}
	if strings.Contains(out, "No entries yet") {
		t.Errorf("placeholder survived two appends; got:\n%s", out)
	}
	first := strings.Index(out, "first entry")
	second := strings.Index(out, "second entry")
	if first < 0 || second < 0 {
		t.Fatalf("both entries must be present; got:\n%s", out)
	}
	if second > first {
		t.Errorf("entries not newest-first (second should precede first); got:\n%s", out)
	}
}

func TestLogAppend_RequiresMessage(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	if _, err := runOKF(t, "log", "append", dir); err == nil {
		t.Fatal("log append without --message must error")
	}
}
