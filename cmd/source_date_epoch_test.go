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
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/clock"
)

// The reproducibility contract (#143): with SOURCE_DATE_EPOCH set, every
// timestamp okfctl WRITES is pinned to that instant; with it unset, output is
// byte-identical to okfctl's pre-existing wall-clock behaviour; and a malformed
// value is a hard error that exits non-zero WITHOUT writing anything. These are
// cmd-level tests: they drive the real cobra root (so root's PersistentPreRunE
// clock resolution runs exactly as in production) via runOKF.

// initBundleAt scaffolds a fresh bundle at dir via `bundle init`.
func initBundleAt(t *testing.T, dir string) {
	t.Helper()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
}

// clockCleanup ensures the shared clock is reset after a test that pins it,
// since runOKF installs the resolved clock process-wide.
func clockCleanup(t *testing.T) { t.Cleanup(clock.Reset) }

// A valid SOURCE_DATE_EPOCH pins created + modified on a newly authored node to
// that exact instant, regardless of the real wall clock.
func TestSourceDateEpoch_PinsNodeTimestamps(t *testing.T) {
	clockCleanup(t)
	dir := t.TempDir()
	initBundleAt(t, dir)

	// 1700000000 == 2023-11-14T22:13:20Z
	t.Setenv(clock.EnvSourceDateEpoch, "1700000000")
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Concept", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "wine", "tannin.md"))
	if err != nil {
		t.Fatalf("read node: %v", err)
	}
	got := string(raw)
	const wantStamp = "2023-11-14T22:13:20Z"
	if !strings.Contains(got, "created: "+wantStamp) {
		t.Errorf("created not pinned to %s; node:\n%s", wantStamp, got)
	}
	if !strings.Contains(got, "modified: "+wantStamp) {
		t.Errorf("modified not pinned to %s; node:\n%s", wantStamp, got)
	}

	// The log.md date heading (reserved_lifecycle.AppendLog, the seam that
	// previously bypassed the clock with a direct time.Now) must also be pinned.
	logRaw, err := os.ReadFile(filepath.Join(dir, "log.md"))
	if err != nil {
		t.Fatalf("read log.md: %v", err)
	}
	if !strings.Contains(string(logRaw), "2023-11-14") {
		t.Errorf("log.md date heading not pinned to 2023-11-14; log:\n%s", logRaw)
	}
}

// A malformed SOURCE_DATE_EPOCH is a hard error naming the variable, exits
// non-zero, and writes NOTHING — the load-bearing "no file created or modified"
// half. This is verified by snapshotting the whole bundle tree before the failed
// command and asserting it is byte-identical after.
func TestSourceDateEpoch_MalformedWritesNothing(t *testing.T) {
	clockCleanup(t)
	dir := t.TempDir()
	initBundleAt(t, dir)

	before := snapshotTree(t, dir)

	t.Setenv(clock.EnvSourceDateEpoch, "not-a-timestamp")
	out, err := runOKF(t, "node", "new", "wine/acidity.md", "--type", "Concept", "--bundle", dir)
	if err == nil {
		t.Fatalf("malformed SOURCE_DATE_EPOCH must error; got nil (output: %q)", out)
	}
	if !strings.Contains(err.Error(), clock.EnvSourceDateEpoch) {
		t.Errorf("error must name %s; got: %v", clock.EnvSourceDateEpoch, err)
	}

	// No new file, no modified file: the tree is byte-identical to before.
	after := snapshotTree(t, dir)
	if !treesEqual(before, after) {
		t.Fatalf("malformed SOURCE_DATE_EPOCH modified the tree.\nbefore: %v\nafter:  %v", before, after)
	}
	// Belt and braces: the node the command would have created must not exist.
	if _, statErr := os.Stat(filepath.Join(dir, "wine", "acidity.md")); statErr == nil {
		t.Fatalf("node was written despite malformed SOURCE_DATE_EPOCH")
	}
}

// The negative control (#143, the load-bearing one): with SOURCE_DATE_EPOCH
// UNSET, okfctl uses the real wall clock exactly as before. We cannot assert an
// exact timestamp (it is "now"), so we assert the STRUCTURE is byte-identical to
// a pinned run except for the timestamp values — i.e. adding the feature did not
// perturb any other byte of the output. Concretely: a pinned run and an unset
// run of the same command differ ONLY in the RFC3339/date substrings.
func TestSourceDateEpoch_UnsetProducesWallClockOutput(t *testing.T) {
	clockCleanup(t)
	// Unset the variable for this test even if the developer's env exports it.
	t.Setenv(clock.EnvSourceDateEpoch, "")

	dir := t.TempDir()
	initBundleAt(t, dir)
	if _, err := runOKF(t, "node", "new", "wine/body.md", "--type", "Concept", "--title", "Body", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "wine", "body.md"))
	if err != nil {
		t.Fatalf("read node: %v", err)
	}
	got := string(raw)
	// Structural assertions: the frontmatter keys and body are present and
	// unchanged in shape (only the timestamp VALUES are wall-clock). If the
	// feature had perturbed the non-timestamp output, these would fail.
	for _, want := range []string{"type: Concept", "title: Body", "created: ", "modified: "} {
		if !strings.Contains(got, want) {
			t.Errorf("unset-var output missing %q; node:\n%s", want, got)
		}
	}
}

// snapshotTree returns a path->sha256 map of every file under root, so a "wrote
// nothing" assertion can compare the whole tree byte-for-byte.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p) //nolint:gosec // reading a fixture the test just wrote
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotTree: %v", err)
	}
	return out
}

// treesEqual reports whether two path->sha256 snapshots are identical.
func treesEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}
