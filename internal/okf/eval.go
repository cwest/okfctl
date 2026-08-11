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
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// EvalFinding is one TACA-Transparency observation about a node's provenance.
// Like a LintFinding it is curation guidance, never a spec-floor failure: eval
// is advisory by default and only gates under --strict. The four checks:
// "grade-missing" | "grade-vocabulary" | "uncited" | "citation-unresolved".
type EvalFinding struct {
	Check   string `json:"check"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// EvalOptions configures the Transparency gate. Zero values take the defaults.
type EvalOptions struct {
	// GradeVocabularyFloor is the minimum number of nodes that must carry a
	// given epistemic/authority value for that value to count as part of the
	// corpus vocabulary. A value carried by fewer nodes is reported as a
	// vocabulary outlier (a likely typo/drift). Zero means the default.
	//
	// Calibrated against the real cwest/knowledge-base corpus. As measured on
	// 2026-08-09 (corpus HEAD 895693b, 246 nodes) the legitimate epistemic
	// vocabulary bottoms out at "draft"/"active" (2 nodes each) and authority at
	// "SYNTHESIS" (2 nodes); the genuine drift lives at count 1 ("authority:
	// high", "authority: DEPRECATED"). A floor of 2 therefore isolates exactly
	// the count-1 outliers without false-positiving a small-but-real grade. See
	// docs/specs/2026-08-07-taca-eval.md.
	GradeVocabularyFloor int
}

// defaultGradeVocabularyFloor: a grade value carried by only ONE node in the
// corpus is treated as drift, not vocabulary. Re-measured 2026-08-09 against the
// real cwest/knowledge-base (corpus HEAD 895693b, 246 nodes) by sweeping the
// floor: at 1 the check can never fire (no value has <1 carrier); at 2 it flags
// exactly the two genuine count-1 outliers ("authority: high" typo, "authority:
// DEPRECATED" lifecycle-into-trust drift); at 3 it false-positives legitimate
// count-2 grades (SYNTHESIS, epistemic draft, epistemic active). So 2 isolates
// real drift without punishing small-but-real grades. See
// docs/specs/2026-08-07-taca-eval.md.
const defaultGradeVocabularyFloor = 2

func (o EvalOptions) withDefaults() EvalOptions {
	if o.GradeVocabularyFloor <= 0 {
		o.GradeVocabularyFloor = defaultGradeVocabularyFloor
	}
	return o
}

// EvalTransparency runs the deterministic, stdlib-only, offline TACA-Transparency
// checks over a bundle and returns findings sorted by (path, check). It never
// mutates the bundle and never touches the network — external http(s) citations
// are deliberately out of scope (that is the eval-sample / human pass, see
// verifying-citation-link-fit). This is the only TACA dimension okfctl can honestly
// automate; Accuracy/Alignment/Calibration are scaffolded by EvalSample instead.
func EvalTransparency(b *Bundle, opts EvalOptions) []EvalFinding {
	opts = opts.withDefaults()
	var findings []EvalFinding
	findings = append(findings, evalGrades(b, opts)...)
	findings = append(findings, evalCitations(b)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Check < findings[j].Check
	})
	return findings
}

// gradeKeys are the two frontmatter keys the corpus uses to record a node's
// trust grade. Neither is an OKF-defined field (§11 unknown keys): okfctl
// recognizes and surfaces them but never enum-gates them in validate.
var gradeKeys = []string{"epistemic", "authority"}

// nodeGrade returns a node's raw value for a grade key and whether it is present
// and non-empty.
func nodeGrade(n *Node, key string) (string, bool) {
	v, ok := n.Frontmatter[key]
	if !ok {
		return "", false
	}
	s := strings.TrimSpace(scalarString(v))
	if s == "" {
		return "", false
	}
	return s, true
}

// observedGradeVocabulary tallies, per grade key, how many nodes carry each
// value. The dominant values ARE the vocabulary; a rare value is drift. The
// vocabulary is derived from the bundle rather than hardcoded so the tool never
// invents an enum the spec deliberately leaves open (§11), while still catching
// the one-off typo.
func observedGradeVocabulary(b *Bundle) map[string]map[string]int {
	counts := map[string]map[string]int{}
	for _, key := range gradeKeys {
		counts[key] = map[string]int{}
	}
	for _, n := range b.Nodes {
		for _, key := range gradeKeys {
			if v, ok := nodeGrade(n, key); ok {
				counts[key][v]++
			}
		}
	}
	return counts
}

// evalGrades reports grade-missing (no epistemic AND no authority) and
// grade-vocabulary (a value below the observed-vocabulary floor) findings.
func evalGrades(b *Bundle, opts EvalOptions) []EvalFinding {
	vocab := observedGradeVocabulary(b)
	var out []EvalFinding
	for _, path := range sortedNodePaths(b) {
		n := b.Nodes[path]

		_, hasEpistemic := nodeGrade(n, "epistemic")
		_, hasAuthority := nodeGrade(n, "authority")
		if !hasEpistemic && !hasAuthority {
			out = append(out, EvalFinding{
				Check:   "grade-missing",
				Path:    path,
				Message: fmt.Sprintf("grade-missing: %s carries no epistemic or authority grade (provenance is invisible)", path),
			})
			// A node with no grade at all cannot also have a vocabulary outlier.
			continue
		}

		for _, key := range gradeKeys {
			v, ok := nodeGrade(n, key)
			if !ok {
				continue
			}
			if vocab[key][v] < opts.GradeVocabularyFloor {
				out = append(out, EvalFinding{
					Check: "grade-vocabulary",
					Path:  path,
					Message: fmt.Sprintf("grade-vocabulary: %s has %s: %q, an off-vocabulary value carried by only %d node(s) (likely a typo/drift)",
						path, key, v, vocab[key][v]),
				})
			}
		}
	}
	return out
}

// evalCitations reports uncited (a node with zero provenance of any kind) and
// citation-unresolved (a bundle-internal citation target resolving to no node)
// findings.
func evalCitations(b *Bundle) []EvalFinding {
	var out []EvalFinding
	for _, path := range sortedNodePaths(b) {
		n := b.Nodes[path]

		sources := n.Sources()
		legacy := citationCount(n.Body)
		inline := hasInlineNodeReference(b, path, n)

		if len(sources) == 0 && legacy == 0 && !inline {
			out = append(out, EvalFinding{
				Check:   "uncited",
				Path:    path,
				Message: fmt.Sprintf("uncited: %s carries no citations (no sources, no # Citations, no internal reference)", path),
			})
			continue
		}

		// citation-unresolved: any frontmatter source whose resource is a
		// bundle-internal node path that resolves to no node. External http(s)
		// sources are skipped — verifying them needs the network.
		for _, s := range sources {
			target, internal := internalResource(s.Resource)
			if !internal {
				continue
			}
			if b.Nodes[target] == nil {
				out = append(out, EvalFinding{
					Check: "citation-unresolved",
					Path:  path,
					Message: fmt.Sprintf("citation-unresolved: %s cites %s which resolves to no node in the bundle",
						path, s.Resource),
				})
			}
		}
	}
	return out
}

// hasInlineNodeReference reports whether a node body links to at least one other
// node in the bundle (an inline citation to a sibling). A self-link does not
// count as citing a source.
func hasInlineNodeReference(b *Bundle, path string, n *Node) bool {
	dir := filepath.Dir(path)
	for _, l := range scanNodeLinks(b, dir, n.Body) {
		if l.resolved != "" && l.resolved != path {
			return true
		}
	}
	return false
}

// internalResource reports whether a source resource points at a bundle-internal
// node (an OKF "/…"-absolute or a bare ".md" path) and returns the resolved node
// key. An http(s) URL or a non-.md resource is external (internal=false).
func internalResource(resource string) (target string, internal bool) {
	r := strings.TrimSpace(resource)
	if r == "" {
		return "", false
	}
	if strings.HasPrefix(r, "http://") || strings.HasPrefix(r, "https://") {
		return "", false
	}
	// Strip any anchor.
	if i := strings.IndexByte(r, '#'); i >= 0 {
		r = r[:i]
	}
	if !strings.HasSuffix(r, ".md") {
		return "", false
	}
	// OKF §5.1: a "/"-absolute resource resolves against the bundle root.
	r = strings.TrimPrefix(r, "/")
	return filepath.ToSlash(filepath.Clean(r)), true
}
