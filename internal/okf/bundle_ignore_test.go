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
	"sort"
	"testing"
)

// vendoredBundle writes a 2-node bundle plus a fake vendored tree
// (tool/.venv/.../site-packages) and a derived output dir (dist). It returns the
// bundle root.
func vendoredBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	writeFile(t, dir, "wine/tannin.md", "---\ntype: Reference\ntitle: Tannin\n---\n# Tannin\n\nSee [acidity](acidity.md).\n")
	writeFile(t, dir, "wine/acidity.md", "---\ntype: Reference\ntitle: Acidity\n---\n# Acidity\n")
	// Fake Python virtualenv sitting next to the bundle.
	writeFile(t, dir, "tool/.venv/lib/python3.12/site-packages/somepkg/README.md", "# somepkg\nvendored doc, nobody authored this\n")
	writeFile(t, dir, "tool/.venv/lib/python3.12/site-packages/somepkg/CHANGES.md", "# Changelog\n")
	// A derived build-output directory.
	writeFile(t, dir, "dist/generated.md", "# generated\nderived output\n")
	return dir
}

// TestLoad_SkipsVendoredDirsByDefault is the core fix: Load must not walk into a
// default skip-listed directory (a virtualenv, a build-output dir), so nothing
// under it becomes a concept node.
func TestLoad_SkipsVendoredDirsByDefault(t *testing.T) {
	dir := vendoredBundle(t)
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Only the two real wine nodes; nothing from .venv/site-packages/dist.
	if got := len(b.Nodes); got != 2 {
		t.Fatalf("concept nodes = %d, want 2 (%v)", got, keys(b.Nodes))
	}
	for p := range b.Nodes {
		if p != "wine/tannin.md" && p != "wine/acidity.md" {
			t.Errorf("vendored/derived path leaked into nodes: %s", p)
		}
	}
}

// TestLoad_RecordsSkippedDirs is the guardrail half: the skip must never be
// silent. Load records which directories it skipped so the CLI can announce them
// on stderr.
func TestLoad_RecordsSkippedDirs(t *testing.T) {
	dir := vendoredBundle(t)
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := append([]string(nil), b.SkippedDirs...)
	sort.Strings(got)
	want := []string{"dist", "tool/.venv"}
	if len(got) != len(want) {
		t.Fatalf("SkippedDirs = %v, want %v", b.SkippedDirs, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SkippedDirs = %v, want %v", b.SkippedDirs, want)
		}
	}
}

// TestLoad_NoIgnoreRestoresFullWalk is the escape hatch (positive control): a
// user who deliberately authored content whose directory name matches the skip
// list can recover it with WithNoIgnore, and the full pre-fix finding set is
// restored.
func TestLoad_NoIgnoreRestoresFullWalk(t *testing.T) {
	dir := vendoredBundle(t)
	b, err := Load(dir, WithNoIgnore())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Full walk: the two wine nodes + the two vendored README/CHANGES +
	// dist/generated = 5 concept nodes. This is the byte-identical pre-fix set.
	if got := len(b.Nodes); got != 5 {
		t.Fatalf("concept nodes with --no-ignore = %d, want 5 (%v)", got, keys(b.Nodes))
	}
	if _, ok := b.Nodes["tool/.venv/lib/python3.12/site-packages/somepkg/README.md"]; !ok {
		t.Errorf("--no-ignore must recover vendored README; have %v", keys(b.Nodes))
	}
	// With --no-ignore nothing was skipped, so there is nothing to announce.
	if len(b.SkippedDirs) != 0 {
		t.Errorf("SkippedDirs with --no-ignore = %v, want empty", b.SkippedDirs)
	}
}

// TestLoad_NoIgnoreNegativeControl proves the escape hatch is a true superset:
// every node the default walk finds is also found by --no-ignore.
func TestLoad_NoIgnoreNegativeControl(t *testing.T) {
	dir := vendoredBundle(t)
	def, err := Load(dir)
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	full, err := Load(dir, WithNoIgnore())
	if err != nil {
		t.Fatalf("Load no-ignore: %v", err)
	}
	for p := range def.Nodes {
		if _, ok := full.Nodes[p]; !ok {
			t.Errorf("node %s found by default walk but missing under --no-ignore", p)
		}
	}
	if len(full.Nodes) <= len(def.Nodes) {
		t.Errorf("--no-ignore (%d) must be a strict superset of default (%d)",
			len(full.Nodes), len(def.Nodes))
	}
}

// TestLoad_NoVendoredDirsIsUnchanged is the second control: a clean bundle with
// no vendored/derived directories loads identically and skips nothing.
func TestLoad_NoVendoredDirsIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
	writeFile(t, dir, "wine/tannin.md", "---\ntype: Reference\n---\n# Tannin\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(b.Nodes); got != 1 {
		t.Fatalf("concept nodes = %d, want 1", got)
	}
	if len(b.SkippedDirs) != 0 {
		t.Errorf("SkippedDirs on clean bundle = %v, want empty", b.SkippedDirs)
	}
}
