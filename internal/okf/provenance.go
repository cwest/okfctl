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
	"time"
)

// Provenance, trust, and lifecycle reader (OKF v0.2 §5). Every family is
// OPTIONAL; absence carries meaning but is never rejected (§11). These are
// READ-ONLY accessors: they parse frontmatter that Load already produced and
// never mutate a Node. Two §13.1 v0.1 fallbacks are wired in — legacy
// `timestamp` for `generated.at`, and a legacy body `# Citations` list for
// `sources` — so a v0.1 bundle reads identically to before this family existed.

// UsageWindow is the { from, to } date range that frames a usage_count (§5.1).
type UsageWindow struct {
	From string
	To   string
}

// Source is one parsed `sources` entry (§5.1). Resource is REQUIRED within an
// entry; the other fields are optional credibility signals. UsageWindow is the
// window in effect for this entry: the shared sibling window framed onto it, or
// the entry's own override when present.
type Source struct {
	ID           string
	Resource     string
	Title        string
	Author       string       // actor (§7); an authority signal
	UsageCount   *int         // adoption/liveness signal; nil when absent
	LastModified string       // YYYY-MM-DD; recency signal
	UsageWindow  *UsageWindow // in-effect window (shared or per-entry override)
}

// Actor is a provenance actor: generated.by / verified[].by (§7). Its recorded
// form is one of `<producer>/<version>`, `human:<id>`, or `process:<id>`.
type Actor string

// IsHuman reports whether the actor is a human per the §7 `human:` prefix —
// the key trust classification keys off (§5.3).
func (a Actor) IsHuman() bool { return strings.HasPrefix(string(a), "human:") }

// Generation records how the current content was produced (§5.2): who (an
// actor) and when (last meaningful change).
type Generation struct {
	By Actor
	At time.Time
}

// Verification is one verification event (§5.2): an actor and an instant.
type Verification struct {
	By Actor
	At time.Time
}

// TrustTier is the derived trust level (§5.3), lowest to highest. It is
// DERIVED, never stored.
type TrustTier string

const (
	TrustUnverified       TrustTier = "unverified"        // §5.3: no verified key
	TrustMachineConfirmed TrustTier = "machine-confirmed" // §5.3: non-human actors only
	TrustHumanReviewed    TrustTier = "human-reviewed"    // §5.3: any human:<id> verifier
)

// StatusStable is the default lifecycle status when `status` is absent (§5.4).
const StatusStable = "stable"

// Sources returns the parsed `sources` list (§5.1). Entries missing the
// REQUIRED `resource` are dropped — the reader surfaces only well-formed
// sources. The shared `usage_window` sibling is framed onto every entry; an
// entry MAY carry its own `usage_window` to override it. Returns an empty slice
// when `sources` is absent.
func (n *Node) Sources() []Source {
	raw, ok := n.Frontmatter["sources"].([]any)
	if !ok {
		return nil
	}
	shared := parseUsageWindow(n.Frontmatter["usage_window"])
	out := make([]Source, 0, len(raw))
	for _, item := range raw {
		m, ok := asStringMap(item)
		if !ok {
			continue
		}
		resource := stringField(m, "resource")
		if resource == "" {
			// §5.1: resource is REQUIRED within an entry.
			continue
		}
		s := Source{
			ID:           stringField(m, "id"),
			Resource:     resource,
			Title:        stringField(m, "title"),
			Author:       stringField(m, "author"),
			LastModified: stringField(m, "last_modified"),
			UsageCount:   intField(m, "usage_count"),
			UsageWindow:  shared,
		}
		if own := parseUsageWindow(m["usage_window"]); own != nil {
			s.UsageWindow = own // §5.1: per-entry override.
		}
		out = append(out, s)
	}
	return out
}

// Generated returns how the content was produced (§5.2). §13.1 fallback: when
// `generated` is absent, fall back to the legacy `timestamp` for `.At` (with an
// empty By — v0.1 recorded no author). ok is false when neither yields a usable
// date.
func (n *Node) Generated() (Generation, bool) {
	if m, ok := asStringMap(n.Frontmatter["generated"]); ok {
		by := stringField(m, "by") // §5.2: by is REQUIRED within generated.
		at, dated := frontmatterTime(m["at"])
		if by != "" || dated {
			return Generation{By: Actor(by), At: at}, true
		}
	}
	// §13.1: legacy timestamp fallback.
	if at, ok := frontmatterTime(n.Frontmatter["timestamp"]); ok {
		return Generation{By: "", At: at}, true
	}
	return Generation{}, false
}

// Verified returns the verification events (§5.2). §11 MUST: a BARE MAPPING is
// treated as a one-element list. Returns an empty slice when `verified` is
// absent.
func (n *Node) Verified() []Verification {
	raw, present := n.Frontmatter["verified"]
	if !present {
		return nil
	}
	// §11 MUST: a bare mapping normalizes to a one-element list.
	if m, ok := asStringMap(raw); ok {
		if v, ok := parseVerification(m); ok {
			return []Verification{v}
		}
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Verification, 0, len(list))
	for _, item := range list {
		if m, ok := asStringMap(item); ok {
			if v, ok := parseVerification(m); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

// TrustTier derives the trust tier from `verified` (§5.3): no verified ⇒
// unverified; non-human actors only ⇒ machine-confirmed; any human:<id> ⇒
// human-reviewed. Derived, never stored.
func (n *Node) TrustTier() TrustTier {
	events := n.Verified()
	if len(events) == 0 {
		return TrustUnverified
	}
	for _, e := range events {
		if e.By.IsHuman() {
			return TrustHumanReviewed
		}
	}
	return TrustMachineConfirmed
}

// Status returns the lifecycle status (§5.4). Absent ⇒ stable.
func (n *Node) Status() string {
	if v, ok := n.Frontmatter["status"].(string); ok && v != "" {
		return v
	}
	return StatusStable // §5.4: absent status ⇒ stable.
}

// StaleAfter returns the absolute stale-after date (§5.5, YYYY-MM-DD). ok is
// false when the field is absent or unparseable.
func (n *Node) StaleAfter() (time.Time, bool) {
	return frontmatterTime(n.Frontmatter["stale_after"])
}

// IsStale reports whether the node is stale as of `today` (§5.5): stale when
// today >= stale_after. A node with no (or unparseable) stale_after is never
// stale.
func (n *Node) IsStale(today time.Time) bool {
	sa, ok := n.StaleAfter()
	if !ok {
		return false
	}
	return !today.Before(sa) // today >= stale_after
}

// SourceCitations returns how many provenance entries a node carries (§13.1
// fallback). It reads frontmatter `sources` first; for a v0.1 document with no
// `sources`, it falls back to counting the legacy body `# Citations` list.
func (n *Node) SourceCitations() int {
	if s := n.Sources(); len(s) > 0 {
		return len(s)
	}
	return citationCount(n.Body) // §13.1: legacy body fallback for v0.1 docs.
}

// parseVerification reads a { by, at } mapping into a Verification. ok is false
// when neither field is usable.
func parseVerification(m map[string]any) (Verification, bool) {
	by := stringField(m, "by")
	at, dated := frontmatterTime(m["at"])
	if by == "" && !dated {
		return Verification{}, false
	}
	return Verification{By: Actor(by), At: at}, true
}

// parseUsageWindow reads a { from, to } mapping into a *UsageWindow, or nil when
// the value is absent or not a mapping.
func parseUsageWindow(v any) *UsageWindow {
	m, ok := asStringMap(v)
	if !ok {
		return nil
	}
	from := stringField(m, "from")
	to := stringField(m, "to")
	if from == "" && to == "" {
		return nil
	}
	return &UsageWindow{From: from, To: to}
}

// asStringMap coerces a yaml.v3-decoded mapping (map[string]any) to a string map.
func asStringMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// stringField reads a string frontmatter value, rendering a date scalar
// (which yaml.v3 may decode into time.Time) back to YYYY-MM-DD so a bare date
// like last_modified round-trips as the author wrote it.
func stringField(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case time.Time:
		return v.UTC().Format("2006-01-02")
	}
	return ""
}

// intField reads an integer frontmatter value, or nil when absent/non-integer.
// yaml.v3 decodes bare integers as int.
func intField(m map[string]any, key string) *int {
	switch v := m[key].(type) {
	case int:
		return &v
	case int64:
		n := int(v)
		return &n
	}
	return nil
}
