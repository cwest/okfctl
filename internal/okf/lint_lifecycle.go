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
	"strings"
)

// lintStatusLifecycle notes a `status` value outside the values §5.4 defines
// (draft|stable|deprecated). v0.3 §5.4 DEFINES such a value: "A value that is
// not in this list is not stable. Consumers that filter to current knowledge
// exclude the concept." It is permitted, and §11.2 rule 4 says the consumer
// keeps the value and MAY warn. So the finding states that meaning rather than
// calling the value a defect, and it is a LINT note, never a validate
// rejection. It stays in the `lint --strict` set because the most common cause
// is a slip worth a curator's look: an old conflated grade (verified,
// decision, …) left in `status` instead of moved to `epistemic`. The check
// fires only when the key is PRESENT; absent ⇒ stable (§5.4) is silent, and
// the §5.4 values are silent.
func lintStatusLifecycle(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		raw, present := b.Nodes[p].Frontmatter["status"]
		if !present {
			continue // §5.4: absent ⇒ stable — not a finding.
		}
		val := strings.TrimSpace(scalarString(raw))
		if LifecycleStatuses[val] {
			continue // a §5.4 value.
		}
		out = append(out, LintFinding{
			Check: "status-lifecycle",
			Path:  p,
			Message: fmt.Sprintf(
				"status-lifecycle: %s has status %q, which §5.4 does not define (draft|stable|deprecated); per §5.4 the concept is not stable and filters to current knowledge exclude it. If this is an epistemic grade, move it to `epistemic`",
				p, val),
		})
	}
	return out
}

// lintSpecVersion flags a per-node `okf_spec_version` frontmatter key. The v0.2
// migration deduped the per-node version up to a single bundle-level
// `okf_version` on the bundle-root index.md (§12): a per-node version invites
// drift with its own bundle. A resurrected per-node `okf_spec_version` is
// therefore a curation defect. This is a LINT finding (soft guidance) — §11
// permits unknown frontmatter keys at the floor, so it is never a validate
// rejection. The bundle-level `okf_version` on index.md is the sanctioned marker
// and is never itself flagged here (index.md is reserved, not a concept node).
func lintSpecVersion(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		if _, present := b.Nodes[p].Frontmatter["okf_spec_version"]; !present {
			continue
		}
		out = append(out, LintFinding{
			Check: "spec-version",
			Path:  p,
			Message: fmt.Sprintf(
				"spec-version: %s carries a per-node okf_spec_version; the bundle version is declared once as okf_version on the bundle-root index.md (§12) — remove the per-node key to avoid drift",
				p),
		})
	}
	return out
}
