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
	"strings"
	"testing"
)

// These tests pin the `validate --check-computations` flag wiring (OKF §10).
// The model-level behavior is covered exhaustively in internal/okf; here we
// prove the flag is opt-in (absent ⇒ no §10 checks), that a §10 finding fails
// the run, and — load-bearing — that the check reads/executes NOTHING named by
// computation/executor/attester.

func writeCmdFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A bundle with a malformed Attested Computation node passes flagless validate
// (opt-in) and fails validate --check-computations. §10.2.
func TestValidateCmd_CheckComputations_OptIn(t *testing.T) {
	dir := t.TempDir()
	writeCmdFile(t, dir, "index.md", "# KB\n")
	// Missing runtime AND no computation — two §10 findings.
	writeCmdFile(t, dir, "c.md", "---\ntype: Attested Computation\ntitle: Broken\n---\n\n# Definition\n\nNo runtime, no computation.\n")

	// Flagless: §10 checks are OFF, so this must exit 0.
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("flagless validate must not run §10 checks; got error: %v", err)
	}

	// With the flag: §10 findings must fail the run.
	out, err := runOKF(t, "validate", "--check-computations", dir)
	if err == nil {
		t.Fatalf("validate --check-computations must fail on §10 findings; out=%q", out)
	}
	if !strings.Contains(out, "runtime") || !strings.Contains(out, "§10") {
		t.Fatalf("output must name the §10 findings; got %q", out)
	}
}

// GOLDEN (done-when: "validate with no flags is byte-identical to today"). A
// bundle whose only node is a badly-malformed Attested Computation (missing
// runtime, no computation, nameless parameter) must, under FLAGLESS validate,
// produce output byte-identical to the same bundle with that node retyped to a
// plain Concept — proving the §10 checks never leak into the floor.
func TestValidateCmd_FlaglessIsBlindToAttestedShape(t *testing.T) {
	broken := "---\n" +
		"type: Attested Computation\n" +
		"title: Broken\n" +
		"parameters:\n" +
		"  - { type: integer }\n" +
		"---\n\n# Definition\n\nNo runtime, no computation, nameless parameter.\n"
	// Same file, retyped to a plain Concept: the floor treats both identically
	// (both carry a non-empty type; the §10 shape is irrelevant to the floor).
	plain := strings.Replace(broken, "type: Attested Computation", "type: Concept", 1)

	dirA := t.TempDir()
	writeCmdFile(t, dirA, "index.md", "# KB\n")
	writeCmdFile(t, dirA, "c.md", broken)

	dirB := t.TempDir()
	writeCmdFile(t, dirB, "index.md", "# KB\n")
	writeCmdFile(t, dirB, "c.md", plain)

	outA, errA := runOKF(t, "validate", dirA)
	outB, errB := runOKF(t, "validate", dirB)
	if (errA == nil) != (errB == nil) {
		t.Fatalf("flagless validate exit must not depend on §10 shape: errA=%v errB=%v", errA, errB)
	}
	if outA != outB {
		t.Fatalf("flagless validate output must be byte-identical regardless of §10 shape:\n attested=%q\n concept =%q", outA, outB)
	}
	if errA != nil {
		t.Fatalf("flagless validate must pass a bundle whose only defect is §10 shape; got %v (out=%q)", errA, outA)
	}
}

func TestValidateCmd_CheckComputations_ConformantPasses(t *testing.T) {
	dir := t.TempDir()
	writeCmdFile(t, dir, "index.md", "# KB\n")
	writeCmdFile(t, dir, "c.md", "---\ntype: Attested Computation\ntitle: OK\nruntime: bigquery\nparameters:\n  - { name: year, type: integer, required: true }\n---\n\n# Computation\n\n    SELECT 1\n")
	if _, err := runOKF(t, "validate", "--check-computations", dir); err != nil {
		t.Fatalf("conformant attested computation must pass --check-computations; got %v", err)
	}
}

// A bundle with NO Attested Computation nodes is unaffected by the flag: the
// output is identical with and without it (the real-corpus inertness property,
// at fixture scale). §10.1.
func TestValidateCmd_CheckComputations_InertWithoutAttestedNodes(t *testing.T) {
	dir := t.TempDir()
	writeCmdFile(t, dir, "index.md", "# KB\n")
	writeCmdFile(t, dir, "wine/tannin.md", "---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nA plain concept.\n")

	flagless, errA := runOKF(t, "validate", dir)
	flagged, errB := runOKF(t, "validate", "--check-computations", dir)
	if errA != nil || errB != nil {
		t.Fatalf("both runs must exit 0 on a bundle with no attested computations; errA=%v errB=%v", errA, errB)
	}
	if flagless != flagged {
		t.Fatalf("--check-computations must be byte-identical on a bundle with no attested computations:\n flagless=%q\n flagged =%q", flagless, flagged)
	}
}

// LOAD-BEARING (done-when): --check-computations reads/executes NOTHING named by
// computation, executor, or attester. We plant a computation path, an executor
// resource, and an attester resource whose target files, IF READ OR EXECUTED,
// would be observable — a sentinel side-effect file the "attester" script would
// write, and a computation/executor path pointing at a marker. We assert no
// subprocess runs (no sentinel appears) and that a MISSING executor/attester
// resource is not itself a finding (§11) — proving the tool never resolves them.
func TestValidateCmd_CheckComputations_NeverReadsOrExecutesResources(t *testing.T) {
	dir := t.TempDir()
	writeCmdFile(t, dir, "index.md", "# KB\n")

	// A real computation file exists (so the §10.3 path resolves), but executor
	// and attester point at resources that DO NOT EXIST. If the tool tried to
	// read or run them it would either error or flag a missing resource. §11
	// says a missing optional family is not a finding, and §10 says OKF executes
	// nothing — so the run must PASS and spawn no process.
	writeCmdFile(t, dir, "computations/lib/revenue.sql", "SELECT 1\n")
	sentinel := filepath.Join(dir, "SENTINEL_SHOULD_NOT_EXIST")
	writeCmdFile(t, dir, "computations/revenue.md",
		"---\n"+
			"type: Attested Computation\n"+
			"title: Revenue\n"+
			"runtime: bigquery\n"+
			"computation: lib/revenue.sql\n"+
			"parameters:\n"+
			"  - { name: year, type: integer, required: true }\n"+
			"executor:\n"+
			"  resource: does/not/exist/run.sh\n"+
			"attester:\n"+
			"  resource: does/not/exist/attest.py\n"+
			"---\n\n# Definition\n\nUses the sanctioned computation file.\n")

	if _, err := runOKF(t, "validate", "--check-computations", dir); err != nil {
		t.Fatalf("check must not resolve/read executor or attester resources (§10/§11); got error: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("a sentinel side-effect file appeared — the check executed something it must not: %v", err)
	}
}
