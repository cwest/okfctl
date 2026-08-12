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
	"reflect"
	"strings"
	"testing"
)

func sampleFixture() *Bundle {
	nodes := map[string]*Node{}
	for _, p := range []string{"a/one.md", "a/two.md", "b/three.md", "b/four.md", "c/five.md"} {
		nodes[p] = evalNode(map[string]any{
			"title":       "Title " + p,
			"description": "Desc " + p,
			"epistemic":   "verified",
			"authority":   "VERIFIED",
			"sources":     []any{map[string]any{"resource": "/a/one.md"}},
		}, "Intro.\n\n**A load-bearing claim about the system** here.\n\n## A heading claim about behavior\n\nMore prose.")
	}
	return evalBundle(nodes)
}

func TestEvalSample_SeededDeterministic(t *testing.T) {
	b := sampleFixture()
	got1 := EvalSample(b, SampleOptions{Count: 3, Seed: 42})
	got2 := EvalSample(b, SampleOptions{Count: 3, Seed: 42})
	if len(got1) != 3 {
		t.Fatalf("Count=3 sample: got %d scaffolds, want 3", len(got1))
	}
	var p1, p2 []string
	for i := range got1 {
		p1 = append(p1, got1[i].Path)
		p2 = append(p2, got2[i].Path)
	}
	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("same seed produced different samples: %v vs %v", p1, p2)
	}
	// The paths must be a sorted, deterministic subset of the corpus.
	for i := 1; i < len(p1); i++ {
		if p1[i-1] >= p1[i] {
			t.Fatalf("sample not sorted/deduped: %v", p1)
		}
	}
}

func TestEvalSample_DifferentSeedDiffers(t *testing.T) {
	b := sampleFixture()
	a := EvalSample(b, SampleOptions{Count: 2, Seed: 1})
	c := EvalSample(b, SampleOptions{Count: 2, Seed: 999})
	// With 5 nodes choose 2, two seeds should usually differ; assert not identical.
	same := len(a) == len(c)
	for i := range a {
		if i < len(c) && a[i].Path != c[i].Path {
			same = false
		}
	}
	if same {
		t.Fatalf("distinct seeds produced identical samples: %+v", a)
	}
}

func TestEvalSample_CountClampedToCorpus(t *testing.T) {
	b := sampleFixture()
	got := EvalSample(b, SampleOptions{Count: 100, Seed: 7})
	if len(got) != 5 {
		t.Fatalf("Count > corpus: got %d, want all 5 nodes", len(got))
	}
}

func TestEvalSample_ExplicitPathsRespected(t *testing.T) {
	b := sampleFixture()
	got := EvalSample(b, SampleOptions{Paths: []string{"b/three.md", "a/one.md"}})
	if len(got) != 2 {
		t.Fatalf("explicit paths: got %d, want 2", len(got))
	}
	// Explicit selection is returned sorted, ignoring Count/Seed.
	if got[0].Path != "a/one.md" || got[1].Path != "b/three.md" {
		t.Fatalf("explicit paths not sorted: %+v", got)
	}
}

func TestEvalSample_ClaimsSingleLineAndSubstantive(t *testing.T) {
	b := evalBundle(map[string]*Node{
		"a/one.md": evalNode(map[string]any{
			"title": "One", "epistemic": "verified",
		},
			// A bold claim wrapping across two source lines, plus a two-word
			// emphasis and a terse heading that must both be rejected.
			"**A substantive claim that wraps\nacross two lines** and continues.\n\n"+
				"**Two words** only.\n\n## Note\n\nbody"),
	})
	got := EvalSample(b, SampleOptions{Paths: []string{"a/one.md"}})
	claims := got[0].Accuracy
	for _, c := range claims {
		if strings.Contains(c.Claim, "\n") {
			t.Fatalf("claim contains a newline (not collapsed): %q", c.Claim)
		}
		if len(strings.Fields(c.Claim)) < 4 {
			t.Fatalf("claim below substance bar leaked through: %q", c.Claim)
		}
	}
	// The wrapping claim must be present as one clean line.
	var foundWrapped bool
	for _, c := range claims {
		if c.Claim == "A substantive claim that wraps across two lines" {
			foundWrapped = true
		}
		if c.Claim == "Two words" || c.Claim == "Note" {
			t.Fatalf("trivial fragment %q should be rejected", c.Claim)
		}
	}
	if !foundWrapped {
		t.Fatalf("wrapped bold claim not extracted as a single collapsed line: %+v", claims)
	}
}

func TestEvalSample_ScaffoldShape(t *testing.T) {
	b := sampleFixture()
	got := EvalSample(b, SampleOptions{Paths: []string{"a/one.md"}})
	s := got[0]
	if s.Title == "" || s.Description == "" {
		t.Fatalf("scaffold missing title/description: %+v", s)
	}
	if s.Epistemic != "verified" || s.Authority != "VERIFIED" {
		t.Fatalf("scaffold missing grades: %+v", s)
	}
	if s.TrustTier == "" {
		t.Fatalf("scaffold missing derived trust tier: %+v", s)
	}
	if len(s.Sources) == 0 {
		t.Fatalf("scaffold missing sources: %+v", s)
	}
	// Accuracy: at least the bold claim is extracted, each with an empty
	// grounding slot for the judge.
	if len(s.Accuracy) == 0 {
		t.Fatalf("scaffold extracted no accuracy claims: %+v", s)
	}
	for _, c := range s.Accuracy {
		if c.Claim == "" {
			t.Fatalf("accuracy claim empty: %+v", c)
		}
		if c.ExpectedGrounding != "" {
			t.Fatalf("accuracy grounding slot should start empty for the judge: %+v", c)
		}
	}
	// Alignment + Calibration slots exist and start unanswered.
	if s.Alignment.Answered != "" {
		t.Fatalf("alignment slot should start empty: %+v", s.Alignment)
	}
	if s.Calibration.HoldsOnRecheck != "" {
		t.Fatalf("calibration slot should start empty: %+v", s.Calibration)
	}
	if s.Calibration.CurrentGrade == "" {
		t.Fatalf("calibration should carry the current grade to re-check: %+v", s.Calibration)
	}
}
