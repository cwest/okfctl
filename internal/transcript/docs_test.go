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

package transcript

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot is the module root, two levels above this package.
const repoRoot = "../.."

// docFiles lists README.md and every Markdown file under docs/, repo-relative.
func docFiles(t *testing.T) []string {
	t.Helper()
	files := []string{"README.md"}
	err := filepath.WalkDir(filepath.Join(repoRoot, "docs"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			rel, err := filepath.Rel(repoRoot, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// buildOkfctl builds okfctl and the okfctl-search plugin from this tree into a
// fresh bin dir and returns the env that puts it first on PATH. The plugin is
// built too so a doc's `$ okfctl-search` lines run this tree's plugin, never
// an older copy that happens to sit on the host's PATH.
func buildOkfctl(t *testing.T) []string {
	t.Helper()
	bin := t.TempDir()
	for name, pkg := range map[string]string{
		"okfctl":        "github.com/cwest/okfctl",
		"okfctl-search": "github.com/cwest/okfctl/cmd/okfctl-search",
	} {
		build := exec.Command("go", "build", "-o", filepath.Join(bin, name), pkg)
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v\n%s", name, err, out)
		}
	}
	fixtures, err := filepath.Abs(filepath.Join(repoRoot, "testdata", "transcripts"))
	if err != nil {
		t.Fatal(err)
	}
	return []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TRANSCRIPT_FIXTURES=" + fixtures,
	}
}

// TestDocTranscripts replays every `$ okfctl` transcript in README.md and
// docs/**/*.md against the binary built from this tree. A doc that shows
// output the binary doesn't print fails here, naming the file, block, line,
// and the expected and actual text. See the package doc for the placeholder
// and directive syntax.
func TestDocTranscripts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the okfctl binary")
	}
	env := buildOkfctl(t)
	replayed := 0
	for _, file := range docFiles(t) {
		src, err := os.ReadFile(filepath.Join(repoRoot, file))
		if err != nil {
			t.Fatal(err)
		}
		blocks, err := Parse(file, src)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(blocks) == 0 {
			continue
		}
		t.Run(file, func(t *testing.T) {
			for _, f := range Replay(blocks, t.TempDir(), env) {
				t.Error(f)
			}
		})
		for _, b := range blocks {
			if b.Skip == "" {
				replayed++
			}
		}
	}
	// The README quickstart alone is one replayed block; zero means the
	// finder broke, not that the docs are clean.
	if replayed == 0 {
		t.Fatal("found no replayable transcripts in README.md or docs/")
	}
}
