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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFile is a small test helper.
func readFileStr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Creating a node appends a log.md entry naming the created path — a node is
// never created without a log entry (closing the audit gap).
func TestNodeNew_AppendsLogEntry(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Concept", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, "wine/tannin.md") {
		t.Fatalf("log.md should record the created node; got:\n%s", log)
	}
}

// A successful node edit touches modified and appends a log entry. A fake
// editor (OKFCTL_EDITOR) simulates the $EDITOR session.
func TestNodeEdit_TouchesModifiedAndLogs(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	// Create a node with an old modified date so the touch is observable.
	nodePath := filepath.Join(dir, "wine", "tannin.md")
	if err := os.MkdirAll(filepath.Dir(nodePath), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-01-01T00:00:00Z\n---\n\n# Tannin\n"
	if err := os.WriteFile(nodePath, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	// A no-op editor: exits 0 without changing the file. okfctl still owns the
	// write-back that refreshes modified + logs the edit.
	t.Setenv("OKFCTL_EDITOR", "true")
	if _, err := runOKF(t, "node", "edit", "wine/tannin.md", "--bundle", dir); err != nil {
		t.Fatalf("node edit: %v", err)
	}

	edited := readFileStr(t, nodePath)
	if strings.Contains(edited, "modified: 2026-01-01T00:00:00Z") {
		t.Fatalf("edit must refresh modified; still stale:\n%s", edited)
	}
	if !strings.Contains(edited, "created: 2026-01-01T00:00:00Z") {
		t.Fatalf("edit must NOT rewrite created; got:\n%s", edited)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, "wine/tannin.md") {
		t.Fatalf("log.md should record the edit; got:\n%s", log)
	}
}
