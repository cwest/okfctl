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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the OKF §10 attested-computation contract-shape checks
// (CheckComputations). §10.2 fixes the contract fields (runtime REQUIRED for
// this type; parameters entries { name, type, required }); §10.3 fixes the
// two-ways-to-provide-the-computation rule; §11 forbids rejecting a concept for
// a missing optional family. Every check applies ONLY to
// `type: Attested Computation` nodes and is inert for every other node.

// writeRaw writes a node verbatim so the frontmatter is exactly what the test
// declares (writeNode synthesizes type+title, which is too coarse here).
func writeRaw(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// loadRaw loads a bundle rooted at dir, failing the test on error.
func loadRaw(t *testing.T, dir string) *Bundle {
	t.Helper()
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%s): %v", dir, err)
	}
	return b
}

func hasFindingContaining(fs []Finding, path, substr string) bool {
	for _, f := range fs {
		if f.Path == path && strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}

// A fully conformant Attested Computation node (inline # Computation fence,
// runtime present, well-formed parameters) produces NO findings. §10.2/§10.3.
func TestCheckComputations_ConformantInlineFence_NoFindings(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "computations/revenue.md", `---
type: Attested Computation
title: Revenue for fiscal year
runtime: bigquery
parameters:
  - { name: year, type: integer, required: true }
executor:
  resource: references/skills/run-on-bq.md
attester:
  resource: references/attesters/revenue.py
---

# Computation

    SELECT SUM(amount) AS revenue
    FROM finance.recognized_revenue
    WHERE fiscal_year = @year
`)
	b := loadRaw(t, dir)
	if fs := CheckComputations(b); len(fs) != 0 {
		t.Fatalf("conformant attested computation must produce no findings; got %v", fs)
	}
}

// A conformant node using the `computation` FILE path (body fence omitted) and
// the file resolving on disk produces NO findings. §10.3 file form.
func TestCheckComputations_ConformantComputationFile_NoFindings(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "computations/lib/revenue.sql", "SELECT 1\n")
	writeRaw(t, dir, "computations/revenue.md", `---
type: Attested Computation
title: Revenue
runtime: bigquery
computation: lib/revenue.sql
parameters:
  - { name: year, type: integer, required: true }
---

# Definition

Uses the sanctioned computation file.
`)
	b := loadRaw(t, dir)
	if fs := CheckComputations(b); len(fs) != 0 {
		t.Fatalf("conformant computation-file node must produce no findings; got %v", fs)
	}
}

// §10.2: runtime is REQUIRED for this type. Missing runtime -> finding.
func TestCheckComputations_MissingRuntime_Flagged_S10_2(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: No runtime
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "runtime") || !hasFindingContaining(fs, "c.md", "§10.2") {
		t.Fatalf("missing runtime must be flagged citing §10.2; got %v", fs)
	}
}

// §10.2: empty/whitespace runtime is the same as missing -> finding.
func TestCheckComputations_EmptyRuntime_Flagged_S10_2(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Empty runtime
runtime: "   "
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "runtime") {
		t.Fatalf("empty runtime must be flagged; got %v", fs)
	}
}

// §10.3: neither a `computation` path nor a body `# Computation` fence -> finding.
func TestCheckComputations_NoComputationAnywhere_Flagged_S10_3(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: No computation
runtime: bigquery
---

# Definition

There is prose here but no computation fence and no computation path.
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "§10.3") {
		t.Fatalf("no computation source must be flagged citing §10.3; got %v", fs)
	}
}

// §10.3: BOTH a computation path AND a body fence present -> a DISTINCT finding
// naming the ambiguity (which governs is unspecified).
func TestCheckComputations_BothComputationSources_DistinctAmbiguityFinding_S10_3(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "lib.sql", "SELECT 1\n")
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Both sources
runtime: bigquery
computation: lib.sql
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "both") && !hasFindingContaining(fs, "c.md", "ambiguous") {
		t.Fatalf("both computation sources must yield a distinct ambiguity finding; got %v", fs)
	}
	if !hasFindingContaining(fs, "c.md", "§10.3") {
		t.Fatalf("ambiguity finding must cite §10.3; got %v", fs)
	}
}

// §10.3/§6.2: a `computation` path that does not resolve to a file on disk ->
// finding naming the unresolved path.
func TestCheckComputations_UnresolvableComputationPath_Flagged_S10_3(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Dangling path
runtime: bigquery
computation: lib/does-not-exist.sql
---

# Definition

The computation lives in a file that is not present.
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "lib/does-not-exist.sql") {
		t.Fatalf("unresolvable computation path must be named in the finding; got %v", fs)
	}
}

// §10.2: a `parameters` entry missing `name` -> finding.
func TestCheckComputations_ParameterMissingName_Flagged_S10_2(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Nameless parameter
runtime: bigquery
parameters:
  - { type: integer, required: true }
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "name") || !hasFindingContaining(fs, "c.md", "§10.2") {
		t.Fatalf("parameter entry missing name must be flagged citing §10.2; got %v", fs)
	}
}

// §10.2 permissive: a `parameters` entry missing only `type` or only `required`
// (but carrying `name`) is NOT a finding — those are not mandatory per-entry.
func TestCheckComputations_ParameterMissingTypeOrRequired_NoFinding_S10_2(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Partial parameters
runtime: bigquery
parameters:
  - { name: year }
  - { name: region, type: string }
  - { name: cutoff, required: false }
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	for _, f := range fs {
		if f.Path == "c.md" {
			t.Fatalf("parameters missing only type/required (name present) must NOT be flagged; got %v", fs)
		}
	}
}

// NEGATIVE CONTROL (§11, load-bearing): missing executor, missing attester,
// absent parameters produce NO finding. §11 forbids rejecting a concept for a
// missing optional family; over-conformance is the failure mode to avoid.
func TestCheckComputations_MissingOptionalFamilies_NoFinding_S11(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Minimal but conformant
runtime: bigquery
---

# Computation

    SELECT 1
`)
	fs := CheckComputations(loadRaw(t, dir))
	if len(fs) != 0 {
		t.Fatalf("§11: missing executor/attester/parameters must NOT be flagged; got %v", fs)
	}
}

// The check is inert for every non-attested node: a plain Concept node missing
// runtime, parameters, executor, and any computation is untouched.
func TestCheckComputations_InertForOtherTypes(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "wine/tannin.md", `---
type: Concept
title: Tannin
---

# Tannin

Tannin is a plain concept with no attested-computation fields at all.
`)
	fs := CheckComputations(loadRaw(t, dir))
	if len(fs) != 0 {
		t.Fatalf("non-attested node must be untouched by CheckComputations; got %v", fs)
	}
}

// A body fence detection edge: a `# Computation` heading with no code block
// under it is NOT a computation — it must still be flagged as "no computation"
// (§10.3 requires a fenced/indented code block, not just the heading).
func TestCheckComputations_ComputationHeadingWithoutCodeBlock_Flagged_S10_3(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", `---
type: Attested Computation
title: Heading only
runtime: bigquery
---

# Computation

Just prose describing the intent, but no fenced or indented code block.
`)
	fs := CheckComputations(loadRaw(t, dir))
	if !hasFindingContaining(fs, "c.md", "§10.3") {
		t.Fatalf("a # Computation heading with no code block is not a computation; must be flagged §10.3; got %v", fs)
	}
}

// A fenced (```) code block under # Computation also counts as inline. §10.3.
func TestCheckComputations_FencedCodeBlock_CountsAsInline_S10_3(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "c.md", "---\ntype: Attested Computation\ntitle: Fenced\nruntime: python\n---\n\n# Computation\n\n```python\nreturn revenue(year)\n```\n")
	fs := CheckComputations(loadRaw(t, dir))
	for _, f := range fs {
		if f.Path == "c.md" {
			t.Fatalf("a fenced code block under # Computation is a valid inline computation; got %v", fs)
		}
	}
}

// LOAD-BEARING (done-when): the attested-computation check must execute NO
// subprocess and must READ nothing named by computation/executor/attester.
// This is a static guarantee, asserted by parsing attested.go: it must not
// import os/exec (or any subprocess package), and must not call os.ReadFile /
// os.Open / ioutil.ReadFile. The ONLY filesystem touch permitted is os.Stat
// (existence check for the §10.3 computation path — never its contents). A
// runtime sentinel test complements this at the cmd layer.
func TestCheckComputations_NoSubprocessNoResourceReads(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "attested.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse attested.go: %v", err)
	}
	bannedImports := map[string]string{
		`"os/exec"`: "spawns subprocesses",
		`"syscall"`: "can spawn/exec",
		`"github.com/cwest/okfctl/internal/exec"`: "subprocess wrapper",
	}
	for _, imp := range f.Imports {
		if why, banned := bannedImports[imp.Path.Value]; banned {
			t.Fatalf("attested.go imports %s (%s); the §10 check must execute nothing", imp.Path.Value, why)
		}
	}

	// Full-source scan: no content-reading call may appear. os.Stat (existence
	// only) is the sole sanctioned filesystem touch.
	src, err := os.ReadFile("attested.go")
	if err != nil {
		t.Fatalf("read attested.go: %v", err)
	}
	text := string(src)
	for _, banned := range []string{"os.ReadFile", "os.Open", "ioutil.ReadFile", "exec.Command", "exec.CommandContext"} {
		if strings.Contains(text, banned) {
			t.Fatalf("attested.go references %q; the §10 check must not read or execute resource contents", banned)
		}
	}
}
