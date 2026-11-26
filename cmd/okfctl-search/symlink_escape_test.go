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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Positive and negative control (§3.2 rule 5, §11.2): the search index build
// never embeds a node read through a symlink that escapes the bundle root, and
// still indexes one that stays inside it.
func TestPlugin_Section11_2_IndexBuildSkipsEscapingSymlink(t *testing.T) {
	dir := writeSearchBundle(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("---\ntype: Concept\ntitle: Secret\n---\n\nOUTSIDE-SECRET-BODY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "leak.md")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	if err := os.Symlink(filepath.Join("wine", "tannin.md"), filepath.Join(dir, "alias.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".okfctl", "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	idx := string(data)
	if strings.Contains(idx, "leak.md") || strings.Contains(idx, "OUTSIDE-SECRET-BODY") {
		t.Error("index embedded a node read through an escaping symlink")
	}
	if !strings.Contains(idx, "alias.md") {
		t.Error("index dropped an in-root symlinked node")
	}
}
