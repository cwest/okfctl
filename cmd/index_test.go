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
	"testing"
)

func TestIndexBuildThenCheck_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass right after build: %v", err)
	}
}

// After node new, the index is auto-maintained: index check passes with no
// manual build. (This is the increment's contract — a build step a human must
// remember is a build step that drifts, so okfctl maintains it automatically.)
func TestNodeNew_KeepsIndexInSync(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "b.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// No manual `index build` — auto-maintenance must have kept it current.
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass after node new without a manual build: %v", err)
	}
}

// index check still detects a HAND-corrupted index: auto-maintenance keeps
// okfctl-mediated mutations in sync, but a direct edit of index.md is caught.
func TestIndexCheck_HandCorruptedExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// Corrupt index.md by hand, bypassing okfctl.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntype: Index\n---\n\n# Stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Fatal("index check must exit nonzero on a hand-corrupted index")
	}
}
