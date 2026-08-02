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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitBulkMigrationBundle builds a bundle where 12 nodes were authored on 12
// distinct dates, then a SINGLE bulk mechanical commit adds a frontmatter key to
// all of them. Every node then drifts against the bulk commit date, so a refresh
// plan is large (12) and dominated by one commit — the exact destructive shape.
// Returns the bundle dir and the bulk commit SHA.
func gitBulkMigrationBundle(t *testing.T) (dir, bulkSHA string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	git := func(env []string, args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), env...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(nil, "init", "-q")
	git(nil, "config", "user.email", "t@example.com")
	git(nil, "config", "user.name", "T")
	git(nil, "config", "commit.gpgsign", "false")

	const n = 12
	type node struct {
		rel string
		day time.Time
	}
	var nodes []node
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("wine/n%02d.md", i)
		// Distinct authoring days spread across a month.
		day := time.Date(2026, 6, 1+i, 9, 0, 0, 0, time.UTC)
		nodes = append(nodes, node{rel: rel, day: day})
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("---\ntype: Concept\ntitle: N%02d\ncreated: 2026-01-01T00:00:00Z\nmodified: %s\n---\n\n# N%02d\n",
			i, day.Format("2006-01-02")+"T00:00:00Z", i)
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		iso := day.Format(time.RFC3339)
		git(nil, "add", "-A")
		git([]string{"GIT_AUTHOR_DATE=" + iso, "GIT_COMMITTER_DATE=" + iso}, "commit", "-q", "-m", "author "+rel)
	}

	// One bulk mechanical commit adds a frontmatter key to EVERY node.
	bulkDay := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	for _, nd := range nodes {
		abs := filepath.Join(dir, filepath.FromSlash(nd.rel))
		b, err := os.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		// Insert `status: verified` before the closing frontmatter fence.
		s := strings.Replace(string(b), "\n---\n", "\nstatus: verified\n---\n", 1)
		if err := os.WriteFile(abs, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	iso := bulkDay.Format(time.RFC3339)
	git(nil, "add", "-A")
	git([]string{"GIT_AUTHOR_DATE=" + iso, "GIT_COMMITTER_DATE=" + iso}, "commit", "-q", "-m", "bulk: v0.2 key sweep")
	shaOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	bulkSHA = strings.TrimSpace(string(shaOut))
	return dir, bulkSHA
}

// A refresh whose plan is dominated by a single bulk commit is REFUSED without
// --yes: it exits non-zero, writes NOTHING, and points the user at
// .okf-drift-ignore-revs. This is the stopgap that closes the destructive path.
func TestNodeRefresh_GuardRefusesBulkPlan(t *testing.T) {
	dir, bulkSHA := gitBulkMigrationBundle(t)
	before := readFileStr(t, filepath.Join(dir, "wine", "n00.md"))

	out, err := runOKF(t, "node", "refresh", dir)
	if err == nil {
		t.Fatalf("guard must refuse a bulk-dominated plan (nonzero exit); out=%q", out)
	}
	// SilenceErrors is set on the root, so the refusal text is carried on the
	// returned error, not the output buffer.
	msg := err.Error()
	if !strings.Contains(msg, ".okf-drift-ignore-revs") {
		t.Fatalf("refusal must point at .okf-drift-ignore-revs; got:\n%s", msg)
	}
	if !strings.Contains(msg, bulkSHA) && !strings.Contains(msg, bulkSHA[:12]) {
		t.Fatalf("refusal should name the dominant commit; got:\n%s", msg)
	}
	if after := readFileStr(t, filepath.Join(dir, "wine", "n00.md")); after != before {
		t.Fatalf("refused refresh must not write to disk")
	}
}

// --yes overrides the guard and performs the refresh (the explicit-confirmation
// escape hatch). The refusal is a stopgap, not a hard wall.
func TestNodeRefresh_GuardOverriddenByYes(t *testing.T) {
	dir, _ := gitBulkMigrationBundle(t)
	out, err := runOKF(t, "node", "refresh", "--yes", dir)
	if err != nil {
		t.Fatalf("--yes must override the guard and exit 0; err=%v out=%q", err, out)
	}
	got := readFileStr(t, filepath.Join(dir, "wine", "n00.md"))
	if !strings.Contains(got, "modified: 2026-08-02T00:00:00Z") {
		t.Fatalf("--yes refresh should have rewritten modified to the bulk day:\n%s", got)
	}
}

// --dry-run is never gated: it writes nothing by definition, so a bulk plan
// still lists cleanly and exits 0. The guard only protects the WRITING path.
func TestNodeRefresh_GuardDoesNotBlockDryRun(t *testing.T) {
	dir, _ := gitBulkMigrationBundle(t)
	out, err := runOKF(t, "node", "refresh", "--dry-run", dir)
	if err != nil {
		t.Fatalf("dry-run must never be gated; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "would be refreshed") {
		t.Fatalf("dry-run should list the plan; got:\n%s", out)
	}
}

// With the bulk SHA listed in .okf-drift-ignore-revs, the plan is EMPTY (the
// comparison walks back to the real authoring commits, which agree), so refresh
// is a clean no-op and the guard never fires — the intended cure.
func TestNodeRefresh_IgnoreRevsEmptiesPlan(t *testing.T) {
	dir, bulkSHA := gitBulkMigrationBundle(t)
	ignore := "# migration key sweep\n" + bulkSHA + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".okf-drift-ignore-revs"), []byte(ignore), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "node", "refresh", "--dry-run", dir)
	if err != nil {
		t.Fatalf("ignore-revs no-op must exit 0; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "No drift") {
		t.Fatalf("with the bulk SHA ignored, refresh should find no drift; got:\n%s", out)
	}
}
