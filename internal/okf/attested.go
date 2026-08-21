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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// AttestedComputationType is the §10.1 concept type these checks apply to.
// Matching is on the exact string value (§7.4 leaves type values open, so this
// is a targeted opt-in check, never a taxonomy the floor enforces).
const AttestedComputationType = "Attested Computation"

// computationHeadRe matches the body `# Computation` heading (§10.3), the same
// anchored-heading idiom used for the legacy `# Citations` fallback. Any heading
// level, case-insensitive, whole line.
var computationHeadRe = regexp.MustCompile(`(?im)^#+\s*Computation\s*$`)

// CheckComputations runs the three structural §10 attested-computation contract
// checks over a bundle. It is OPT-IN (wired behind `validate --check-computations`)
// and applies ONLY to `type: Attested Computation` nodes (§10.1); every other
// node is inert.
//
// The checks (all structural — this never executes, reads, or resolves the
// CONTENTS of anything named by computation/executor/attester; OKF fixes the
// interface, not the packaging, and does not execute anything itself, §10/§10.5):
//
//   - §10.2 runtime REQUIRED for this type: missing or empty runtime → finding.
//   - §10.3 the computation is provided exactly one way: an inline body
//     `# Computation` code fence OR a `computation` path with the fence omitted.
//     Neither → finding; both → a distinct ambiguity finding; a `computation`
//     path that does not resolve on disk (§6.2) → finding naming the path.
//   - §10.2 parameters entries are { name, type, required }: an entry missing
//     `name` → finding. Entries missing only `type` or `required` are NOT
//     flagged (§11 permissive line — those are not mandatory per-entry).
//
// §11 (load-bearing): missing executor, missing attester, and absent parameters
// are NOT findings — a concept is never rejected for a missing optional family.
//
// Findings are emitted in sorted path order so validate's output stays stable
// for diffing.
func CheckComputations(b *Bundle) []Finding {
	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []Finding
	for _, p := range paths {
		n := b.Nodes[p]
		if n.Frontmatter == nil {
			continue // an unparseable node is the floor's concern, not this check's.
		}
		if strings.TrimSpace(n.Type()) != AttestedComputationType {
			continue // §10.1: inert for every other type.
		}
		out = append(out, checkOneComputation(b, p, n)...)
	}
	return out
}

// checkOneComputation runs the three §10 checks against a single Attested
// Computation node and returns its findings (possibly empty).
func checkOneComputation(b *Bundle, p string, n *Node) []Finding {
	var out []Finding

	// §10.2: runtime is REQUIRED for this type.
	if strings.TrimSpace(stringField(n.Frontmatter, "runtime")) == "" {
		out = append(out, Finding{
			Path:    p,
			Message: "attested computation is missing required field: runtime (§10.2)",
		})
	}

	// §10.3: exactly one computation source — inline body `# Computation` fence
	// OR a `computation` path (with the fence omitted).
	compPath := strings.TrimSpace(stringField(n.Frontmatter, "computation"))
	hasPath := compPath != ""
	hasFence := hasComputationFence(n.Body)
	switch {
	case hasPath && hasFence:
		out = append(out, Finding{
			Path:    p,
			Message: "attested computation declares both a `computation` path and a body `# Computation` fence; provide exactly one (§10.3)",
		})
	case !hasPath && !hasFence:
		out = append(out, Finding{
			Path:    p,
			Message: "attested computation has neither a `computation` path nor a body `# Computation` code fence (§10.3)",
		})
	case hasPath && !computationPathResolves(b, p, compPath):
		out = append(out, Finding{
			Path:    p,
			Message: "attested computation `computation` path does not resolve to a file: " + compPath + " (§10.3)",
		})
	}

	// §10.2: parameters entries are { name, type, required }; `name` is the hole's
	// identity. An entry missing `name` is malformed. Entries missing only `type`
	// or `required` are NOT flagged (§11 permissive line).
	out = append(out, checkParameters(p, n.Frontmatter["parameters"])...)

	return out
}

// checkParameters flags any `parameters` entry that is a mapping missing `name`
// (§10.2). Absent parameters, a non-list parameters value, and non-mapping
// entries are not this check's concern (§11 — do not reject for a missing or
// unrecognized optional family). Entries are indexed in the message so the
// author can find the offending one without a second lookup.
func checkParameters(p string, raw any) []Finding {
	list, ok := raw.([]any)
	if !ok {
		return nil // absent or not a list — §11 permissive.
	}
	var out []Finding
	for i, item := range list {
		m, ok := asStringMap(item)
		if !ok {
			continue // a non-mapping entry is not a { name, ... } entry to check.
		}
		if strings.TrimSpace(stringField(m, "name")) == "" {
			out = append(out, Finding{
				Path:    p,
				Message: "attested computation parameters entry " + strconv.Itoa(i) + " is missing required field: name (§10.2)",
			})
		}
	}
	return out
}

// hasComputationFence reports whether the body carries a `# Computation` heading
// (§10.3) followed by an actual code block — a fenced (``` or ~~~) block or an
// indented (4-space / tab) block. The heading alone is not a computation; §10.3
// requires the fenced/indented code block that holds it.
func hasComputationFence(body string) bool {
	loc := computationHeadRe.FindStringIndex(body)
	if loc == nil {
		return false
	}
	section := body[loc[1]:]
	for _, line := range strings.Split(section, "\n") {
		// A subsequent heading ends the Computation section; stop scanning.
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			break
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			return true // a fenced code block opens the computation.
		}
		// An indented code block: a non-blank line indented by >=4 spaces or a tab.
		if trimmed != "" && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
			return true
		}
	}
	return false
}

// computationPathResolves reports whether a `computation` path (§6.2) resolves
// to a file on disk. Resolution mirrors buildEdges: dir-relative to the linking
// node first, then root-relative — accepting whichever exists. The file is
// Stat'd only; its CONTENTS are never read (OKF does not execute, §10). Paths
// that escape the bundle root are treated as unresolved.
func computationPathResolves(b *Bundle, nodePath, compPath string) bool {
	cleanRoot, err := filepath.Abs(b.Root)
	if err != nil {
		return false
	}
	candidates := []string{
		// dir-relative to the node (§6.2 default for a relative reference).
		path.Clean(path.Join(path.Dir(nodePath), compPath)),
		// root-relative fallback.
		path.Clean(compPath),
	}
	for _, rel := range candidates {
		if rel == "." || strings.HasPrefix(rel, "..") {
			continue // escapes the bundle root — not a resolvable in-bundle file.
		}
		abs := filepath.Join(cleanRoot, filepath.FromSlash(rel))
		// Guard: the resolved absolute path must stay within the bundle root.
		within, err := filepath.Rel(cleanRoot, abs)
		if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
