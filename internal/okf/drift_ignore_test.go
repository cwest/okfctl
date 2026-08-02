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
	"os"
	"path/filepath"
	"testing"
	"time"
)

// migratedBundle builds a repo where a node was authored on a real date and
// then touched by a single bulk mechanical commit on a later date. Returns the
// bundle root and the bulk commit's SHA. Without an ignore list this node
// drifts (frontmatter modified = authoring day, git last-commit = bulk day).
func migratedBundle(t *testing.T) (root, bulkSHA string) {
	t.Helper()
	root = t.TempDir()
	initRepo(t, root)
	// Authored 2026-06-15; frontmatter records that real day.
	writeDriftNode(t, root, "wine/tannin.md", "2026-06-15T00:00:00Z")
	commitAt(t, root, "author tannin", time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))

	// Bulk mechanical commit: add a frontmatter key to the same file.
	abs := filepath.Join(root, "wine", "tannin.md")
	bulk := "---\ntype: Concept\ntitle: X\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-06-15T00:00:00Z\nstatus: verified\n---\n\n# X\n"
	if err := os.WriteFile(abs, []byte(bulk), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "bulk: v0.2 key sweep", time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	bulkSHA = headSHA(t, root)
	return root, bulkSHA
}

// POSITIVE CONTROL: with no ignore list, a node last touched by the bulk commit
// still drifts (its frontmatter day != the bulk commit day). The mechanism does
// not silence real drift by default.
func TestDrift_BulkCommitDriftsWithoutIgnoreList(t *testing.T) {
	root, _ := migratedBundle(t)
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 1 {
		t.Fatalf("without ignore list the bulk-touched node must drift; got %d: %+v", len(drift), drift)
	}
}

// NEGATIVE CONTROL: with the bulk SHA listed in .okf-drift-ignore-revs, the
// comparison walks back to the real authoring commit, which agrees with the
// frontmatter — so the node produces ZERO drift findings.
func TestDrift_ListedBulkCommitSilencesDrift(t *testing.T) {
	root, bulkSHA := migratedBundle(t)
	ignore := "# opt out the migration key sweep\n" + bulkSHA + "\n"
	if err := os.WriteFile(filepath.Join(root, ".okf-drift-ignore-revs"), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b); len(drift) != 0 {
		t.Fatalf("listed bulk commit must produce zero drift; got %d: %+v", len(drift), drift)
	}
	// And RefreshPlan (the remediation) proposes nothing for it either.
	if plan := RefreshPlan(b); len(plan) != 0 {
		t.Fatalf("listed bulk commit must yield an empty refresh plan; got %+v", plan)
	}
}

// A genuine incremental edit whose commit is NOT on the ignore list still drifts
// even when an unrelated SHA is listed — the check is not narrowed into
// uselessness by the presence of an ignore file.
func TestDrift_UnlistedIncrementalStillDrifts(t *testing.T) {
	root, bulkSHA := migratedBundle(t)
	// A second, independent node with a stale modified, its own isolated commit
	// NOT on the ignore list.
	writeDriftNode(t, root, "wine/acidity.md", "2026-07-01T00:00:00Z")
	commitAt(t, root, "author acidity", time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC))

	ignore := bulkSHA + "\n" // only the bulk commit is ignored
	if err := os.WriteFile(filepath.Join(root, DriftIgnoreRevsFile), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	drift := DriftFindings(b)
	if len(drift) != 1 {
		t.Fatalf("unlisted incremental edit must still drift; got %d: %+v", len(drift), drift)
	}
	if drift[0].Path != "wine/acidity.md" {
		t.Fatalf("the drifting node should be the unlisted incremental one; got %q", drift[0].Path)
	}
}

// Layer 4 (AGENTS.md): the mechanism must behave identically under a bundle that
// DECLARES v0.2. v0.2 permits the legacy `modified` field as a fallback (§12), so
// a v0.2-declared node carrying `modified` still drifts, and .okf-drift-ignore-revs
// still walks back past a listed bulk commit. Running the same case at v0.2 is
// the control proving "it passes" is not only "v0.1 still passes."
func TestDrift_V02BundleHonorsIgnoreRevs(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	// Declare v0.2 via the .okf sidecar.
	if err := os.WriteFile(filepath.Join(root, ".okf"), []byte("okf_version: 0.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A v0.2-declared node still carrying the legacy `modified` field.
	writeDriftNode(t, root, "wine/tannin.md", "2026-06-15T00:00:00Z")
	commitAt(t, root, "author tannin", time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC))

	abs := filepath.Join(root, "wine", "tannin.md")
	bulk := "---\ntype: Concept\ntitle: X\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-06-15T00:00:00Z\nstatus: stable\n---\n\n# X\n"
	if err := os.WriteFile(abs, []byte(bulk), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAt(t, root, "bulk: v0.2 key sweep", time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	bulkSHA := headSHA(t, root)

	// Without the ignore list, the v0.2 node drifts (positive control).
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if b.OkfVersion != "0.2" {
		t.Fatalf("bundle must declare v0.2; got %q", b.OkfVersion)
	}
	if drift := DriftFindings(b); len(drift) != 1 {
		t.Fatalf("v0.2 bulk-touched node must drift without ignore list; got %d", len(drift))
	}

	// With the bulk SHA listed, drift walks back and is silenced (negative control).
	if err := os.WriteFile(filepath.Join(root, DriftIgnoreRevsFile), []byte(bulkSHA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if drift := DriftFindings(b2); len(drift) != 0 {
		t.Fatalf("v0.2: listed bulk commit must silence drift; got %d: %+v", len(drift), drift)
	}
}
