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
