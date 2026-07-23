// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
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
	"strconv"
	"strings"
	"testing"
)

// writeRaw writes an arbitrary node body under dir (test helper for link tests).
func writeRaw(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func nodeDoc(title string) string { return "---\ntype: Concept\ntitle: " + title + "\n---\n" }

func TestNodeMv_MovesAndRewrites(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"See [x](wine/foo.md).\n")
	writeRaw(t, dir, "wine/foo.md", nodeDoc("Foo"))

	if _, err := runOKF(t, "node", "mv", "wine/foo.md", "cellar/foo.md", "--bundle", dir); err != nil {
		t.Fatalf("node mv: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cellar", "foo.md")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if !strings.Contains(string(body), "[x](cellar/foo.md)") {
		t.Fatalf("inbound link not rewritten: %q", body)
	}
}

func TestNodeMv_DryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"See [x](wine/foo.md).\n")
	writeRaw(t, dir, "wine/foo.md", nodeDoc("Foo"))
	before, _ := os.ReadFile(filepath.Join(dir, "a.md"))

	out, err := runOKF(t, "node", "mv", "wine/foo.md", "cellar/foo.md", "--bundle", dir, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wine", "foo.md")); err != nil {
		t.Fatal("dry-run moved the file")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if string(before) != string(after) {
		t.Fatal("dry-run mutated a body")
	}
	if !strings.Contains(out, "cellar/foo.md") {
		t.Fatalf("dry-run did not print the plan: %q", out)
	}
}

func TestNodeMv_ErrNewExists(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A"))
	writeRaw(t, dir, "b.md", nodeDoc("B"))
	if _, err := runOKF(t, "node", "mv", "a.md", "b.md", "--bundle", dir); err == nil {
		t.Fatal("expected error: destination exists")
	}
}

func TestNodeMv_ErrReserved(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A"))
	if _, err := runOKF(t, "node", "mv", "a.md", "index.md", "--bundle", dir); err == nil {
		t.Fatal("expected error: reserved destination")
	}
}

func TestNodeRm_RemovesAndReportsOrphans(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"[x](b.md)\n")
	writeRaw(t, dir, "b.md", nodeDoc("B"))

	out, err := runOKF(t, "node", "rm", "a.md", "--bundle", dir)
	if err != nil {
		t.Fatalf("node rm: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); !os.IsNotExist(err) {
		t.Fatal("file not removed")
	}
	if !strings.Contains(out, "b.md") {
		t.Fatalf("orphan not reported: %q", out)
	}
}

func TestNodeRm_DryRun(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"[x](b.md)\n")
	writeRaw(t, dir, "b.md", nodeDoc("B"))

	if _, err := runOKF(t, "node", "rm", "a.md", "--bundle", dir, "--dry-run"); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal("dry-run removed the file")
	}
}

// fakeEditor writes a shell script to dir that, when run as the editor, appends
// the given text to its file argument (and touches a marker so tests can prove
// it ran), then exits with the given code. Returns the script path.
func fakeEditor(t *testing.T, dir, appendText string, exitCode int, marker string) string {
	t.Helper()
	script := filepath.Join(dir, "fake-editor.sh")
	body := "#!/bin/sh\n"
	if marker != "" {
		body += "touch " + marker + "\n"
	}
	if appendText != "" {
		// $1 is the file path passed by `node edit`.
		body += "printf '%s' " + shQuote(appendText) + " >> \"$1\"\n"
	}
	body += "exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}
	return script
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func TestNodeEdit_RunsEditorThenValidates(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"body\n")
	marker := filepath.Join(dir, "ran.marker")
	ed := fakeEditor(t, dir, "\nappended line\n", 0, marker)
	t.Setenv("OKFCTL_EDITOR", ed)

	if _, err := runOKF(t, "node", "edit", "a.md", "--bundle", dir); err != nil {
		t.Fatalf("node edit: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("editor did not run")
	}
	body, _ := os.ReadFile(filepath.Join(dir, "a.md"))
	if !strings.Contains(string(body), "appended line") {
		t.Fatalf("edit not saved: %q", body)
	}
}

func TestNodeEdit_ReportsValidationFailure(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"body\n")
	// Editor overwrites the type with an empty value (spec-floor violation).
	ed := fakeEditor(t, dir, "", 0, "")
	// Rewrite the script to CLOBBER the file with an invalid node.
	bad := "#!/bin/sh\nprintf '%s' '---\\ntype:\\ntitle: A\\n---\\nbody\\n' > \"$1\"\nexit 0\n"
	if err := os.WriteFile(ed, []byte(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OKFCTL_EDITOR", ed)

	_, err := runOKF(t, "node", "edit", "a.md", "--bundle", dir)
	if err == nil {
		t.Fatal("expected non-zero exit for spec-floor violation")
	}
}

func TestNodeEdit_EditorNonZeroAborts(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A")+"body\n")
	ed := fakeEditor(t, dir, "SHOULD NOT PERSIST", 1, "")
	t.Setenv("OKFCTL_EDITOR", ed)

	if _, err := runOKF(t, "node", "edit", "a.md", "--bundle", dir); err == nil {
		t.Fatal("expected error: editor exited non-zero")
	}
}

func TestNodeEdit_ErrReservedOrMissing(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "a.md", nodeDoc("A"))
	ed := fakeEditor(t, dir, "", 0, "")
	t.Setenv("OKFCTL_EDITOR", ed)
	if _, err := runOKF(t, "node", "edit", "index.md", "--bundle", dir); err == nil {
		t.Fatal("expected error editing reserved file")
	}
	if _, err := runOKF(t, "node", "edit", "nope.md", "--bundle", dir); err == nil {
		t.Fatal("expected error editing missing node")
	}
}
