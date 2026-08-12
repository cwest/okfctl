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

// node builds a concept *Node with the given frontmatter and body for tests.
func evalNode(fm map[string]any, body string) *Node {
	return &Node{Frontmatter: fm, Body: body}
}

// evalBundle assembles a *Bundle from a path->node map for eval tests.
func evalBundle(nodes map[string]*Node) *Bundle {
	b := &Bundle{Nodes: map[string]*Node{}, Reserved: map[string]*Node{}}
	for p, n := range nodes {
		n.Path = p
		b.Nodes[p] = n
	}
	return b
}

func findingsByCheck(fs []EvalFinding) map[string][]EvalFinding {
	out := map[string][]EvalFinding{}
	for _, f := range fs {
		out[f.Check] = append(out[f.Check], f)
	}
	return out
}

func TestEvalTransparency_GradeMissing(t *testing.T) {
	b := evalBundle(map[string]*Node{
		"a/graded.md": evalNode(map[string]any{
			"title": "Graded", "epistemic": "verified", "sources": []any{
				map[string]any{"resource": "/a/graded.md"},
			},
		}, "body"),
		"a/authority-only.md": evalNode(map[string]any{
			"title": "Auth", "authority": "VERIFIED", "sources": []any{
				map[string]any{"resource": "/a/graded.md"},
			},
		}, "body"),
		"a/ungraded.md": evalNode(map[string]any{
			"title": "Ungraded", "sources": []any{
				map[string]any{"resource": "/a/graded.md"},
			},
		}, "body"),
	})
	fs := EvalTransparency(b, EvalOptions{})
	got := findingsByCheck(fs)["grade-missing"]
	if len(got) != 1 {
		t.Fatalf("grade-missing: got %d findings, want 1: %+v", len(got), fs)
	}
	if got[0].Path != "a/ungraded.md" {
		t.Fatalf("grade-missing: got path %q, want a/ungraded.md", got[0].Path)
	}
}

func TestEvalTransparency_GradeVocabulary(t *testing.T) {
	// Build a corpus whose dominant epistemic vocabulary is {verified, documented}
	// (many nodes), plus one node with an off-vocabulary typo value. The rare
	// outlier must be flagged; the dominant values must not.
	nodes := map[string]*Node{}
	for i, v := range []string{"verified", "verified", "verified", "documented", "documented"} {
		p := string(rune('a'+i)) + "/n.md"
		nodes[p] = evalNode(map[string]any{
			"title": "N", "epistemic": v,
			"sources": []any{map[string]any{"resource": "/z/home.md"}},
		}, "body")
	}
	nodes["z/home.md"] = evalNode(map[string]any{
		"title": "Home", "epistemic": "verified",
		"sources": []any{map[string]any{"resource": "/z/home.md"}},
	}, "body")
	// The outlier: a typo'd authority value.
	nodes["y/typo.md"] = evalNode(map[string]any{
		"title": "Typo", "epistemic": "verified", "authority": "high",
		"sources": []any{map[string]any{"resource": "/z/home.md"}},
	}, "body")
	b := evalBundle(nodes)
	fs := EvalTransparency(b, EvalOptions{})
	got := findingsByCheck(fs)["grade-vocabulary"]
	if len(got) != 1 {
		t.Fatalf("grade-vocabulary: got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Path != "y/typo.md" || !strings.Contains(got[0].Message, "high") {
		t.Fatalf("grade-vocabulary: got %+v, want the y/typo.md 'high' outlier", got[0])
	}
}

func TestEvalTransparency_Uncited(t *testing.T) {
	b := evalBundle(map[string]*Node{
		"a/cited-fm.md": evalNode(map[string]any{
			"title": "FM cited", "epistemic": "verified",
			"sources": []any{map[string]any{"resource": "/a/cited-fm.md"}},
		}, "body"),
		"a/cited-inline.md": evalNode(map[string]any{
			"title": "Inline cited", "epistemic": "verified",
		}, "see [home](/a/cited-fm.md) for detail"),
		"a/cited-legacy.md": evalNode(map[string]any{
			"title": "Legacy cited", "epistemic": "verified",
		}, "body\n\n# Citations\n\n- [1] Some Source, https://example.com/x\n"),
		"a/uncited.md": evalNode(map[string]any{
			"title": "Uncited", "epistemic": "verified",
		}, "a claim with no provenance at all"),
	})
	fs := EvalTransparency(b, EvalOptions{})
	got := findingsByCheck(fs)["uncited"]
	if len(got) != 1 {
		t.Fatalf("uncited: got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Path != "a/uncited.md" {
		t.Fatalf("uncited: got path %q, want a/uncited.md", got[0].Path)
	}
}

func TestEvalTransparency_CitationUnresolved(t *testing.T) {
	b := evalBundle(map[string]*Node{
		"a/home.md": evalNode(map[string]any{
			"title": "Home", "epistemic": "verified",
			"sources": []any{map[string]any{"resource": "/a/home.md"}},
		}, "body"),
		"a/good.md": evalNode(map[string]any{
			"title": "Good", "epistemic": "verified",
			"sources": []any{map[string]any{"resource": "/a/home.md"}},
		}, "body"),
		"a/bad-internal.md": evalNode(map[string]any{
			"title": "Bad internal", "epistemic": "verified",
			"sources": []any{map[string]any{"resource": "/a/missing.md"}},
		}, "body"),
		"a/external.md": evalNode(map[string]any{
			"title": "External", "epistemic": "verified",
			"sources": []any{map[string]any{"resource": "https://example.com/paper"}},
		}, "body"),
	})
	fs := EvalTransparency(b, EvalOptions{})
	got := findingsByCheck(fs)["citation-unresolved"]
	if len(got) != 1 {
		t.Fatalf("citation-unresolved: got %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Path != "a/bad-internal.md" || !strings.Contains(got[0].Message, "/a/missing.md") {
		t.Fatalf("citation-unresolved: got %+v, want a/bad-internal.md -> /a/missing.md", got[0])
	}
	// The external https source must NOT be flagged (network is out of scope).
	for _, f := range got {
		if f.Path == "a/external.md" {
			t.Fatalf("citation-unresolved: external https source should not be flagged: %+v", f)
		}
	}
}

func TestEvalTransparency_SortedAndStable(t *testing.T) {
	b := evalBundle(map[string]*Node{
		"b/n.md": evalNode(map[string]any{"title": "B"}, "no cite"),
		"a/n.md": evalNode(map[string]any{"title": "A"}, "no cite"),
	})
	fs := EvalTransparency(b, EvalOptions{})
	if len(fs) < 2 {
		t.Fatalf("want >= 2 findings, got %d", len(fs))
	}
	for i := 1; i < len(fs); i++ {
		if fs[i-1].Path > fs[i].Path {
			t.Fatalf("findings not sorted by path: %q before %q", fs[i-1].Path, fs[i].Path)
		}
	}
}
