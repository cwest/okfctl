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

// buildConsumerForAC3 writes a consumer bundle (a couple linked concept nodes, a
// built index, a log) under <root>/kb, plus a referenced template bundle under
// <root>/tpl. templatesKey is written to the consumer's .okf when non-empty.
func buildConsumerForAC3(t *testing.T, root, templatesKey string) {
	t.Helper()
	refTemplateBundle(t, root, "tpl", "owner")
	kb := filepath.Join(root, "kb")
	okfBody := "okf_version: 0.2\n"
	if templatesKey != "" {
		okfBody += "templates: " + templatesKey + "\n"
	}
	writeTplFile(t, kb, ".okf", okfBody)
	writeTplFile(t, kb, "log.md", "# Log\n")
	writeTplFile(t, kb, "concepts/alpha.md", "---\ntype: Concept\ntitle: Alpha\n---\n\n# Alpha\n\nSee [Beta](beta.md).\n")
	writeTplFile(t, kb, "concepts/beta.md", "---\ntype: Concept\ntitle: Beta\n---\n\n# Beta\n")
	// A built, current index so `index check` is stable.
	writeTplFile(t, kb, "index.md", "# KB\n\n- [Alpha](concepts/alpha.md)\n- [Beta](concepts/beta.md)\n")
}

// hashTree returns a deterministic hash of every file path + content under dir,
// so a test can assert a directory subtree is byte-for-byte unchanged.
func hashTree(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(files)
	for _, p := range files {
		rel, _ := filepath.Rel(dir, p)
		b, rerr := os.ReadFile(p) //nolint:gosec // test fixture under t.TempDir()
		if rerr != nil {
			t.Fatalf("read %s: %v", p, rerr)
		}
		fmt.Fprintf(h, "%s\x00%x\n", filepath.ToSlash(rel), sha256.Sum256(b))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestAC3_OutputByteIdenticalWithAndWithoutKey is AC3: a consumer with a
// referenced-template key produces output BYTE-IDENTICAL to the same consumer
// without the key, for validate, lint, index build and index check. The key is
// consulted ONLY by the template commands; it must not perturb any other command.
func TestAC3_OutputByteIdenticalWithAndWithoutKey(t *testing.T) {
	withDir := t.TempDir()
	withoutDir := t.TempDir()
	buildConsumerForAC3(t, withDir, "../tpl")
	buildConsumerForAC3(t, withoutDir, "")

	cases := []struct {
		name string
		args []string
	}{
		{"validate", []string{"validate"}},
		{"lint", []string{"lint"}},
		{"index build", []string{"index", "build"}},
		{"index check", []string{"index", "check"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withArgs := append(append([]string{}, tc.args...), filepath.Join(withDir, "kb"))
			withoutArgs := append(append([]string{}, tc.args...), filepath.Join(withoutDir, "kb"))
			outWith, errWith := runOKF(t, withArgs...)
			outWithout, errWithout := runOKF(t, withoutArgs...)
			// Normalize the two temp roots out of the output so only the key's
			// effect remains; the paths differ by construction.
			normWith := strings.ReplaceAll(outWith, filepath.Join(withDir, "kb"), "<KB>")
			normWithout := strings.ReplaceAll(outWithout, filepath.Join(withoutDir, "kb"), "<KB>")
			if normWith != normWithout {
				t.Errorf("%s output differs with vs without templates key:\nWITH:\n%s\nWITHOUT:\n%s", tc.name, normWith, normWithout)
			}
			if (errWith == nil) != (errWithout == nil) {
				t.Errorf("%s exit status differs: with=%v without=%v", tc.name, errWith, errWithout)
			}
		})
	}
}

// TestAC3_IndexBuildLeavesReferencedDirUntouched is AC3's write-isolation clause:
// `index build` on the consumer must not touch the referenced template bundle —
// hash its whole subtree before and after and assert equality.
func TestAC3_IndexBuildLeavesReferencedDirUntouched(t *testing.T) {
	root := t.TempDir()
	buildConsumerForAC3(t, root, "../tpl")
	tplDir := filepath.Join(root, "tpl")
	before := hashTree(t, tplDir)
	if _, err := runOKF(t, "index", "build", filepath.Join(root, "kb")); err != nil {
		t.Fatalf("index build: %v", err)
	}
	after := hashTree(t, tplDir)
	if before != after {
		t.Errorf("index build mutated the referenced template bundle\nbefore=%s\nafter=%s", before, after)
	}
}
