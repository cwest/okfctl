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
	"strings"
	"testing"
)

// TestRenderIndex_PreservesExistingRootHeading is the root-cause regression for
// the sub-directory mis-title footgun. The bundle-root heading is a human-facing
// title, not a tool-mandated literal (OKF §8 leaves the section heading text to
// the producer). When a build runs against a directory whose on-disk index.md
// already carries an H1, that title MUST be preserved verbatim rather than
// overwritten with the generic `# Knowledge Base` default — otherwise building a
// neighborhood sub-directory (passed as the bundle arg, so it becomes the root
// "") silently rewrites e.g. `# Domains` to `# Knowledge Base`.
func TestRenderIndex_PreservesExistingRootHeading(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	// Stand in for a neighborhood whose index.md has a neighborhood-named H1
	// (as every real cwest/knowledge-base neighborhood does), passed to build
	// as the bundle arg so it loads as the root ("").
	writeBundleRootIndex(t, dir, "# Domains\n\n_Progressive-disclosure entry point._\n")
	writeNode(t, dir, "tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	// The root render leads with the §12 okf_version frontmatter block, so the
	// H1 follows it rather than starting the string; assert the heading line is
	// present and the generic default did NOT replace it.
	if !strings.Contains("\n"+got, "\n# Domains\n") {
		t.Errorf("existing root H1 not preserved; want a `# Domains` heading, got:\n%s", got)
	}
	if strings.Contains(got, "# Knowledge Base") {
		t.Errorf("build overwrote the curator H1 with the generic default `# Knowledge Base`; got:\n%s", got)
	}
}

// TestRenderIndex_DefaultsRootHeadingWhenNoIndexOnDisk pins the fallback: a fresh
// bundle with no on-disk root index.md still gets the generic `# Knowledge Base`
// title from a build. (Negative control: preservation must not suppress the
// default when there is nothing to preserve.)
func TestRenderIndex_DefaultsRootHeadingWhenNoIndexOnDisk(t *testing.T) {
	dir := t.TempDir()
	// No Scaffold, no index.md — the directory holds only a concept node.
	writeNode(t, dir, "tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	if !strings.HasPrefix(got, "# Knowledge Base\n") {
		t.Errorf("fresh root index must default to `# Knowledge Base`; got:\n%s", got)
	}
}

// TestRenderIndex_PreservesKnowledgeBaseRootHeading is the zero-diff control for
// bundle-root scope: a root index already titled `# Knowledge Base` must stay
// `# Knowledge Base` after a build (preservation is a no-op when the on-disk
// title already equals the default), so `index build <bundle-root>` stays
// zero-diff on a real corpus.
func TestRenderIndex_PreservesKnowledgeBaseRootHeading(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	writeBundleRootIndex(t, dir, "# Knowledge Base\n\n_Entry point._\n")
	writeNode(t, dir, "tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	// Heading follows the root frontmatter block; assert the line is present.
	if !strings.Contains("\n"+got, "\n# Knowledge Base\n") {
		t.Errorf("root H1 `# Knowledge Base` not retained; got:\n%s", got)
	}
}
