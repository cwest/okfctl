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

package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExec creates a file with the given mode in dir and returns its path.
func writeExec(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestDiscover_FindsExecutablesNamedOkfctlPrefix(t *testing.T) {
	dir := t.TempDir()
	writeExec(t, dir, "okfctl-alpha", 0o755)
	writeExec(t, dir, "okfctl-beta", 0o755)
	writeExec(t, dir, "okfctl-nonexec", 0o644) // not executable -> excluded
	writeExec(t, dir, "notaplugin", 0o755)     // wrong prefix -> excluded
	writeExec(t, dir, "other-okfctl-x", 0o755) // prefix not at start -> excluded

	got := Discover(dir)
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	want := []string{"alpha", "beta"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Discover names = %v, want %v", names, want)
	}
	// abs paths, correctly stripped
	for _, p := range got {
		if !filepath.IsAbs(p.Path) {
			t.Errorf("plugin %s path not absolute: %s", p.Name, p.Path)
		}
		if filepath.Base(p.Path) != "okfctl-"+p.Name {
			t.Errorf("plugin %s path base mismatch: %s", p.Name, p.Path)
		}
	}
}

func TestDiscover_DedupesFirstOnPathWins(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	winner := writeExec(t, first, "okfctl-alpha", 0o755)
	writeExec(t, second, "okfctl-alpha", 0o755)

	got := Discover(first + string(os.PathListSeparator) + second)
	if len(got) != 1 {
		t.Fatalf("want 1 deduped plugin, got %d: %+v", len(got), got)
	}
	if got[0].Path != winner {
		t.Errorf("first-on-PATH should win: got %s, want %s", got[0].Path, winner)
	}
}

func TestLookup_ResolvesAndMisses(t *testing.T) {
	dir := t.TempDir()
	want := writeExec(t, dir, "okfctl-alpha", 0o755)

	if p, ok := Lookup("alpha", dir); !ok || p != want {
		t.Errorf("Lookup(alpha) = %q,%v; want %q,true", p, ok, want)
	}
	if p, ok := Lookup("ghost", dir); ok || p != "" {
		t.Errorf("Lookup(ghost) = %q,%v; want \"\",false", p, ok)
	}
}
