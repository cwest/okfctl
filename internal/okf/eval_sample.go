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
	"math/rand"
	"regexp"
	"sort"
	"strings"
)

// SampleOptions selects which nodes the spot-check scaffold covers.
//
// Precedence: an explicit Paths set wins (used by the --changed-since curation
// hook, resolved to node paths in the cmd layer). Otherwise a deterministic
// pseudo-random Count sample seeded by Seed is drawn. Count <= 0 with no Paths
// yields no scaffolds.
type SampleOptions struct {
	Paths []string // explicit node paths to scaffold (wins over Count/Seed)
	Count int      // size of a random sample when Paths is empty
	Seed  int64    // seed for reproducible sampling (0 ⇒ a fixed default seed)
}

// maxScaffoldClaims caps the extracted Accuracy claims per node so a long node
// does not produce an unwieldy worksheet. The sampler is a spot-check, not a
// full grader.
const maxScaffoldClaims = 8

// AccuracyClaim is one factual claim extracted from a node body, paired with an
// empty grounding slot for a human or LLM judge to fill in: does the node's
// cited source actually support this claim? okfctl extracts the claim; it never
// judges the grounding.
type AccuracyClaim struct {
	Claim             string `json:"claim"`
	ExpectedGrounding string `json:"expected_grounding"` // filled by the judge
}

// AlignmentCheck asks whether the node answers the question it set out to. The
// question is the node's title/description; Answered is filled by the judge.
type AlignmentCheck struct {
	Question string `json:"question"` // title + description: what the node set out to answer
	Answered string `json:"answered"` // filled by the judge: yes/no/partial + why
}

// CalibrationCheck measures whether a node's grade holds up on re-check, so a
// periodic sample can tell whether the "VERIFIED" rate is calibrated. CurrentGrade
// is what okfctl records today; HoldsOnRecheck is filled by the judge.
type CalibrationCheck struct {
	CurrentGrade   string `json:"current_grade"`    // the grade being re-checked
	HoldsOnRecheck string `json:"holds_on_recheck"` // filled by the judge: yes/no + note
}

// NodeEvalScaffold is the per-node eval-set entry the sampler emits for the three
// un-automatable TACA dimensions. okfctl pre-populates every field it can extract
// (path, title, description, grades, sources, candidate claims) and leaves the
// judgment slots empty. It computes NO truth verdict.
type NodeEvalScaffold struct {
	Path        string           `json:"path"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Epistemic   string           `json:"epistemic"`
	Authority   string           `json:"authority"`
	TrustTier   string           `json:"trust_tier"`
	Sources     []string         `json:"sources"`
	Accuracy    []AccuracyClaim  `json:"accuracy"`
	Alignment   AlignmentCheck   `json:"alignment"`
	Calibration CalibrationCheck `json:"calibration"`
}

// EvalSample selects a spot-check sample of nodes and returns an eval-set scaffold
// per node for the Accuracy/Alignment/Calibration dimensions that okfctl cannot
// automate. It is deterministic: an explicit Paths set is honored verbatim
// (sorted); otherwise a seeded pseudo-random Count sample is drawn reproducibly.
func EvalSample(b *Bundle, opts SampleOptions) []NodeEvalScaffold {
	selected := selectSamplePaths(b, opts)
	out := make([]NodeEvalScaffold, 0, len(selected))
	for _, p := range selected {
		n := b.Nodes[p]
		if n == nil {
			continue
		}
		out = append(out, buildScaffold(n, p))
	}
	return out
}

// selectSamplePaths resolves SampleOptions to a sorted, de-duped set of node
// paths present in the bundle.
func selectSamplePaths(b *Bundle, opts SampleOptions) []string {
	if len(opts.Paths) > 0 {
		seen := map[string]bool{}
		var out []string
		for _, p := range opts.Paths {
			if b.Nodes[p] != nil && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		sort.Strings(out)
		return out
	}
	if opts.Count <= 0 {
		return nil
	}
	all := sortedNodePaths(b) // deterministic base order
	if opts.Count >= len(all) {
		return all
	}
	seed := opts.Seed
	if seed == 0 {
		seed = 1 // a fixed default so a zero seed is still reproducible
	}
	// Seeded math/rand is intentional and correct here: --sample selection is
	// documented as reproducible from --seed, so the sampler MUST be a seedable
	// PRNG. crypto/rand cannot be seeded and would break that determinism. This
	// RNG selects which nodes to hand a human for review — it is not a security
	// primitive, so predictability is a feature, not a weakness.
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // G404: seeded math/rand is required for reproducible sampling; not a security primitive
	// Partial Fisher–Yates over a copy: pick Count indices deterministically.
	idx := make([]int, len(all))
	for i := range idx {
		idx[i] = i
	}
	for i := 0; i < opts.Count; i++ {
		j := i + r.Intn(len(idx)-i)
		idx[i], idx[j] = idx[j], idx[i]
	}
	chosen := make([]string, 0, opts.Count)
	for i := 0; i < opts.Count; i++ {
		chosen = append(chosen, all[idx[i]])
	}
	sort.Strings(chosen) // stable, readable output order
	return chosen
}

func buildScaffold(n *Node, path string) NodeEvalScaffold {
	epistemic, _ := n.Epistemic()
	authority, _ := nodeGrade(n, "authority")

	var srcs []string
	for _, s := range n.Sources() {
		srcs = append(srcs, s.Resource)
	}

	claims := extractClaims(n)
	accuracy := make([]AccuracyClaim, 0, len(claims))
	for _, c := range claims {
		accuracy = append(accuracy, AccuracyClaim{Claim: c})
	}

	title := nodeTitle(n)
	desc := nodeDescription(n)
	question := title
	if desc != "" {
		question = title + " — " + desc
	}

	grade := epistemic
	if grade == "" {
		grade = authority
	}

	return NodeEvalScaffold{
		Path:        path,
		Title:       title,
		Description: desc,
		Epistemic:   epistemic,
		Authority:   authority,
		TrustTier:   string(n.TrustTier()),
		Sources:     srcs,
		Accuracy:    accuracy,
		Alignment:   AlignmentCheck{Question: question},
		Calibration: CalibrationCheck{CurrentGrade: grade},
	}
}

var (
	// A **bold** run — a common load-bearing-assertion marker in the corpus.
	boldClaimRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// An ATX heading line (## Heading) — a claim the section is organized around.
	headingClaimRe = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
)

// extractClaims heuristically pulls candidate factual claims from a node's prose:
// bolded assertions and section headings, in first-seen order, de-duped, capped.
// These are CANDIDATES for a judge to verify, not verified facts. Each candidate
// is whitespace-collapsed to a single line and must clear a substance bar (word
// count) so the worksheet carries assertions, not one-word emphasis or section
// labels.
func extractClaims(n *Node) []string {
	body := proseBody(n)
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		// Collapse any internal whitespace (a bold span or heading may wrap
		// across source lines) into single spaces so a claim is one clean line.
		s = strings.Join(strings.Fields(s), " ")
		// A claim needs real substance: at least four words and eight chars.
		// One- or two-word emphasis ("Block Buzz") and terse section labels
		// ("Grading note.") are not claims a judge can ground.
		if len(s) < 8 || len(strings.Fields(s)) < 4 {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		seen[key] = true
		if len(out) < maxScaffoldClaims {
			out = append(out, s)
		}
	}
	for _, m := range boldClaimRe.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	for _, m := range headingClaimRe.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	return out
}
