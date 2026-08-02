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
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mkPromoteCLIBundle writes files (rel->content) under a temp dir. Unlike a
// pure-model fixture it goes through the on-disk shape a real corpus has,
// including directory-concept index.md files carrying frontmatter.
func mkPromoteCLIBundle(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func cliTreeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		fmt.Fprintf(h, "%s\x00%x\x00", filepath.ToSlash(rel), data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// promoteFixtureFiles is a multi-directory corpus authored in the
// directory-as-concept shape: two non-root index.md files carry frontmatter,
// inbound links use BOTH the foo/ and foo/index.md spellings, and the bundle
// root index carries the legal §12 okf_version marker.
func promoteFixtureFiles() map[string]string {
	return map[string]string{
		".okf":         "okf_version: 0.1\n",
		"index.md":     "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n\n* [Foo](foo/)\n* [Bar](bar/)\n",
		"log.md":       "# Change Log\n\n_No entries yet. Record changes with `okfctl log append`._\n",
		"foo/index.md": "---\ntype: Concept\ntitle: Foo\ncreated: 2026-01-01\nmodified: 2026-01-01\n---\n\n# Foo\n\nFoo is a concept authored as a folder note.\n",
		"bar/index.md": "---\ntype: Concept\ntitle: Bar\ncreated: 2026-02-02\nmodified: 2026-02-02\n---\n\n# Bar\n\nBar links to [Foo](../foo/).\n",
		// inbound links in both spellings, various relative forms.
		"alpha.md": "---\ntype: Concept\ntitle: Alpha\ncreated: 2026-03-03\nmodified: 2026-03-03\n---\n\n# Alpha\n\nExplicit [Foo](foo/index.md) and dir [Bar](bar/).\n",
	}
}

// `node promote --dry-run` lists the moves and writes ZERO bytes.
func TestNodePromote_DryRunWritesNothing(t *testing.T) {
	dir := mkPromoteCLIBundle(t, promoteFixtureFiles())
	before := cliTreeHash(t, dir)

	out, err := runOKF(t, "node", "promote", dir, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run must exit 0: err=%v out=%q", err, out)
	}
	if after := cliTreeHash(t, dir); before != after {
		t.Fatalf("dry-run wrote to disk: tree hash changed")
	}
	// It lists every intended move.
	if !strings.Contains(out, "foo/index.md") || !strings.Contains(out, "foo/foo.md") {
		t.Fatalf("dry-run did not list the foo promotion:\n%s", out)
	}
	if !strings.Contains(out, "bar/index.md") || !strings.Contains(out, "bar/bar.md") {
		t.Fatalf("dry-run did not list the bar promotion:\n%s", out)
	}
}

// A real `node promote` leaves the bundle conformant (validate exits 0), rewrites
// inbound links in both spellings, regenerates frontmatter-free indexes, appends
// to log.md, leaves the root index alone, and keeps `created` immutable.
func TestNodePromote_RealRunConformant(t *testing.T) {
	dir := mkPromoteCLIBundle(t, promoteFixtureFiles())

	if out, err := runOKF(t, "node", "promote", dir); err != nil {
		t.Fatalf("promote must exit 0: err=%v out=%q", err, out)
	}

	// validate exits 0 (the directory-index frontmatter finding is gone).
	if out, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate must pass after promote; err=%v out=%q", err, out)
	}

	// Promoted concept files exist; old directory indexes are regenerated clean.
	fooConcept := readFileStr(t, filepath.Join(dir, "foo", "foo.md"))
	if !strings.Contains(fooConcept, "Foo is a concept authored as a folder note.") {
		t.Fatalf("promoted foo concept lost its body:\n%s", fooConcept)
	}
	if !strings.Contains(fooConcept, "created: 2026-01-01") {
		t.Fatalf("created not immutable in promoted foo:\n%s", fooConcept)
	}

	// Regenerated foo/index.md carries NO frontmatter.
	fooIndex := readFileStr(t, filepath.Join(dir, "foo", "index.md"))
	if strings.HasPrefix(fooIndex, "---") {
		t.Fatalf("regenerated foo/index.md must have no frontmatter:\n%s", fooIndex)
	}

	// Inbound links rewritten for BOTH spellings.
	alpha := readFileStr(t, filepath.Join(dir, "alpha.md"))
	if !strings.Contains(alpha, "foo/foo.md") {
		t.Fatalf("explicit-index inbound link not rewritten:\n%s", alpha)
	}
	if strings.Contains(alpha, "](bar/)") {
		t.Fatalf("dir-style inbound link to bar not rewritten:\n%s", alpha)
	}

	// Root index untouched (still the §12 okf_version marker).
	rootIndex := readFileStr(t, filepath.Join(dir, "index.md"))
	if !strings.Contains(rootIndex, "okf_version") {
		t.Fatalf("root index okf_version marker lost:\n%s", rootIndex)
	}

	// log.md appended for each promoted node.
	logMD := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(logMD, "promoted foo/index.md") || !strings.Contains(logMD, "promoted bar/index.md") {
		t.Fatalf("log.md missing promotion entries:\n%s", logMD)
	}
}

// The acceptance criterion that matters most: after a promote run, `lint
// --strict` reports ZERO broken-link findings — proof the bulk rewriter broke no
// links.
func TestNodePromote_NoBrokenLinksAfter(t *testing.T) {
	dir := mkPromoteCLIBundle(t, promoteFixtureFiles())
	if out, err := runOKF(t, "node", "promote", dir); err != nil {
		t.Fatalf("promote must exit 0: err=%v out=%q", err, out)
	}
	out, err := runOKF(t, "lint", dir, "--strict")
	if strings.Contains(out, "broken-link") {
		t.Fatalf("lint --strict reported broken-link findings after promote:\n%s", out)
	}
	if err != nil && strings.Contains(out, "broken-link") {
		t.Fatalf("broken-link gate fired after promote: err=%v out=%q", err, out)
	}
}

// `--name` applies one basename convention uniformly.
func TestNodePromote_NameOverride(t *testing.T) {
	dir := mkPromoteCLIBundle(t, promoteFixtureFiles())
	if out, err := runOKF(t, "node", "promote", dir, "--name", "overview"); err != nil {
		t.Fatalf("promote --name must exit 0: err=%v out=%q", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "foo", "overview.md")); err != nil {
		t.Fatalf("foo/overview.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bar", "overview.md")); err != nil {
		t.Fatalf("bar/overview.md not created: %v", err)
	}
	if out, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate must pass after --name promote; err=%v out=%q", err, out)
	}
}

// A bundle with no directory-concept indexes is a clean no-op.
func TestNodePromote_NoopWhenNothingToPromote(t *testing.T) {
	dir := mkPromoteCLIBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n",
		"log.md":   "# Change Log\n\n_No entries yet. Record changes with `okfctl log append`._\n",
		"note.md":  "---\ntype: Concept\ntitle: Note\ncreated: 2026-01-01\nmodified: 2026-01-01\n---\n\n# Note\n",
	})
	out, err := runOKF(t, "node", "promote", dir)
	if err != nil {
		t.Fatalf("no-op promote must exit 0: err=%v out=%q", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "no ") {
		t.Fatalf("expected a no-op message, got:\n%s", out)
	}
}
