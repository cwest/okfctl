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
	"strings"
	"testing"
)

// vendoredCLIBundle writes a 2-node bundle plus a fake vendored .venv tree and a
// derived dist dir, mirroring the loader-level fixture but through the CLI.
func vendoredCLIBundle(t *testing.T) string {
	return writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "Body."),
		"tool/.venv/lib/python3.12/site-packages/pkg/README.md": "# vendored\nnobody wrote this\n",
		"dist/generated.md": "# generated\n",
	})
}

// TestLintCmd_SkipsVendoredAndAnnounces is the guardrail control: lint skips the
// vendored/derived dirs by default (no findings about them) AND announces the
// skip on stderr — never silent.
func TestLintCmd_SkipsVendoredAndAnnounces(t *testing.T) {
	dir := vendoredCLIBundle(t)
	stdout, stderr, err := runOKFSplit(t, "lint", dir)
	if err != nil {
		t.Fatalf("lint should exit 0 by default; err: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	if strings.Contains(stdout, ".venv") || strings.Contains(stdout, "dist/generated") {
		t.Errorf("vendored/derived paths must not appear in findings; stdout:\n%s", stdout)
	}
	// The skip is announced on stderr with a pointer to the escape hatch.
	if !strings.Contains(stderr, "skipped") || !strings.Contains(stderr, "--no-ignore") {
		t.Errorf("expected a stderr skip note mentioning --no-ignore; stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "tool/.venv") || !strings.Contains(stderr, "dist") {
		t.Errorf("stderr note must name the skipped dirs; stderr:\n%s", stderr)
	}
}

// TestLintCmd_NoIgnoreRestoresWalk is the positive control: --no-ignore walks
// the vendored content back in, so lint now sees it (the vendored README with no
// frontmatter surfaces), and nothing is announced as skipped.
func TestLintCmd_NoIgnoreRestoresWalk(t *testing.T) {
	dir := vendoredCLIBundle(t)
	stdout, stderr, err := runOKFSplit(t, "lint", "--no-ignore", dir)
	if err != nil {
		t.Fatalf("lint --no-ignore should exit 0 by default; err: %v", err)
	}
	if !strings.Contains(stdout, ".venv") {
		t.Errorf("--no-ignore must surface vendored content in findings; stdout:\n%s", stdout)
	}
	if strings.Contains(stderr, "skipped") {
		t.Errorf("--no-ignore must skip nothing, so no stderr note; stderr:\n%s", stderr)
	}
}

// TestAnalyzeCmd_NodeCountExcludesVendored proves the fix flows to analyze: the
// node count reflects only authored nodes, not the vendored tree.
func TestAnalyzeCmd_NodeCountExcludesVendored(t *testing.T) {
	dir := vendoredCLIBundle(t)
	def, _, err := runOKFSplit(t, "analyze", dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !strings.Contains(def, "2 node(s)") {
		t.Errorf("analyze default should count 2 authored nodes; got:\n%s", firstLine(def))
	}
	full, _, err := runOKFSplit(t, "analyze", "--no-ignore", dir)
	if err != nil {
		t.Fatalf("analyze --no-ignore: %v", err)
	}
	if !strings.Contains(full, "4 node(s)") {
		t.Errorf("analyze --no-ignore should count all 4 nodes; got:\n%s", firstLine(full))
	}
}

// TestCleanBundleNoNote is the second control: a bundle with no vendored/derived
// dirs produces no skip note at all.
func TestCleanBundleNoNote(t *testing.T) {
	dir := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc("Concept", "A", "Body."),
	})
	_, stderr, err := runOKFSplit(t, "lint", dir)
	if err != nil {
		t.Fatalf("lint clean bundle: %v", err)
	}
	if strings.Contains(stderr, "skipped") {
		t.Errorf("clean bundle must produce no skip note; stderr:\n%s", stderr)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
