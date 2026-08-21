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
	"testing"

	"github.com/cwest/okfctl/internal/clock"
)

// End-to-end reproducibility (#143): the full write surface — scaffold (bundle
// init) + author (node new, which also appends to log.md and regenerates
// index.md) + an explicit index rebuild — run TWICE under the SAME
// SOURCE_DATE_EPOCH but at DIFFERENT real wall-clock instants, must produce
// byte-identical trees. The two runs genuinely occur at different wall moments
// (a later call is never the same nanosecond as the first), so identical output
// is the proof the wall clock no longer leaks into any written byte. Before this
// feature, created/modified and the log.md date heading tracked wall time and
// the trees would diverge.
func TestSourceDateEpoch_E2EByteIdenticalTrees(t *testing.T) {
	clockCleanup(t)
	t.Setenv(clock.EnvSourceDateEpoch, "1700000000")

	build := func(dir string) map[string]string {
		initBundleAt(t, dir)
		if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Concept", "--title", "Tannin", "--bundle", dir); err != nil {
			t.Fatalf("node new tannin: %v", err)
		}
		if _, err := runOKF(t, "node", "new", "wine/acidity.md", "--type", "Concept", "--title", "Acidity", "--bundle", dir); err != nil {
			t.Fatalf("node new acidity: %v", err)
		}
		if _, err := runOKF(t, "index", dir); err != nil {
			t.Fatalf("index: %v", err)
		}
		return snapshotTree(t, dir)
	}

	first := build(t.TempDir())
	second := build(t.TempDir())

	if !treesEqual(first, second) {
		for p, h := range first {
			if second[p] != h {
				t.Errorf("path %q differs between two pinned runs: %s vs %s", p, h, second[p])
			}
		}
		for p := range second {
			if _, ok := first[p]; !ok {
				t.Errorf("path %q present in second run only", p)
			}
		}
		t.Fatalf("two runs under the same SOURCE_DATE_EPOCH produced different trees")
	}
}

// Positive control for the E2E: with the SAME pipeline run under two DIFFERENT
// pinned epochs, the trees MUST diverge (the timestamps differ). A test that
// passed with identical epochs AND different epochs would prove nothing — this
// asserts the E2E's byte-identical result above is actually caused by the shared
// pin, not by the tree being clock-insensitive.
func TestSourceDateEpoch_E2EDifferentEpochsDiverge(t *testing.T) {
	clockCleanup(t)

	build := func(dir, epoch string) map[string]string {
		t.Setenv(clock.EnvSourceDateEpoch, epoch)
		initBundleAt(t, dir)
		if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Concept", "--bundle", dir); err != nil {
			t.Fatalf("node new: %v", err)
		}
		return snapshotTree(t, dir)
	}

	a := build(t.TempDir(), "1700000000") // 2023-11-14
	b := build(t.TempDir(), "1600000000") // 2020-09-13

	if treesEqual(a, b) {
		t.Fatalf("two DIFFERENT epochs produced identical trees; the pin is not reaching the written timestamps")
	}
}
