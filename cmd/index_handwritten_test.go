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

func writeTestFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// (a) OKF v0.3 §8: a hand-written index.md in a directory with no nodes
// survives `index build`, which names it on stderr and exits non-zero.
func TestIndexBuild_Section8_KeepsHandWrittenOrphanAndExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	notes := "# Empty\n\nHand-written notes about this folder.\n"
	p := filepath.Join(dir, "empty", "index.md")
	writeTestFile(t, p, notes)

	stdout, stderr, err := runOKFSplit(t, "index", "build", dir)
	if err == nil {
		t.Fatalf("index build must exit non-zero when it keeps a hand-written index; stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "empty/index.md") {
		t.Fatalf("stderr must name the kept file; got %q", stderr)
	}
	if got := readFileStr(t, p); got != notes {
		t.Fatalf("hand-written orphan was changed or deleted; got %q", got)
	}
}

// (b) §8: a hand-written intro in a directory with nodes is not overwritten
// without --overwrite-hand-written; with it, the file is regenerated.
func TestIndexBuild_Section8_OverwriteHandWrittenFlag(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/t.md", "--type", "Reference", "--title", "T", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "wine", "index.md")
	intro := "# Wine\n\nHand-written intro.\n"
	writeTestFile(t, p, intro)

	if _, _, err := runOKFSplit(t, "index", "build", dir); err == nil {
		t.Fatal("index build must exit non-zero when it keeps a hand-written index")
	}
	if got := readFileStr(t, p); got != intro {
		t.Fatalf("intro overwritten without the opt-in; got %q", got)
	}
	// check agrees the tree is not current.
	if out, err := runOKF(t, "index", "check", dir); err == nil || !strings.Contains(out, "wine/index.md") || !strings.Contains(out, "hand-written") {
		t.Fatalf("index check must fail naming the hand-written file; err=%v out=%q", err, out)
	}

	if _, err := runOKF(t, "index", "build", "--overwrite-hand-written", dir); err != nil {
		t.Fatalf("index build --overwrite-hand-written: %v", err)
	}
	if got := readFileStr(t, p); strings.Contains(got, "Hand-written intro.") || !strings.Contains(got, "* [T](t.md)") {
		t.Fatalf("opt-in must regenerate the file; got %q", got)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check after overwrite: %v", err)
	}
}

// (c) NEGATIVE control, §8: generated indexes are pruned and regenerated
// silently, with exit 0 and no warning.
func TestIndexBuild_Section8_GeneratedIndexesStaySilent(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"wine/t.md", "old/x.md"} {
		if _, err := runOKF(t, "node", "new", rel, "--type", "Reference", "--title", "N", "--bundle", dir); err != nil {
			t.Fatal(err)
		}
	}
	// old/ loses its only node by hand, leaving its generated index orphaned.
	if err := os.Remove(filepath.Join(dir, "old", "x.md")); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runOKFSplit(t, "index", "build", dir)
	if err != nil {
		t.Fatalf("generated indexes must rebuild with exit 0: %v (stderr %q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("no warning expected for generated indexes; stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Wrote") {
		t.Fatalf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "old", "index.md")); !os.IsNotExist(err) {
		t.Fatalf("generated orphan old/index.md must be pruned; stat err = %v", err)
	}
	if out, err := runOKF(t, "index", "check", dir); err != nil || !strings.Contains(out, "OK: index.md is current") {
		t.Fatalf("index check: err=%v out=%q", err, out)
	}
}

// The node verbs rebuild the index too; they must keep a hand-written index
// and say so, without failing the node operation itself.
func TestNodeNew_Section8_KeepsHandWrittenIndexAndWarns(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "wine", "index.md")
	intro := "# Wine\n\nHand-written intro.\n"
	writeTestFile(t, p, intro)

	_, stderr, err := runOKFSplit(t, "node", "new", "wine/t.md", "--type", "Reference", "--title", "T", "--bundle", dir)
	if err != nil {
		t.Fatalf("node new must still succeed: %v", err)
	}
	if !strings.Contains(stderr, "wine/index.md") || !strings.Contains(stderr, "hand-written") {
		t.Fatalf("node new must warn about the kept index; stderr=%q", stderr)
	}
	if got := readFileStr(t, p); got != intro {
		t.Fatalf("node new overwrote the hand-written index; got %q", got)
	}
}
