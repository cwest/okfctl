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

// LifecycleStatuses is the set of status values that §5.4 defines. Absent ⇒
// stable. Unlike the open `type` vocabulary (§7.4), §5.4 names these values,
// and it also DEFINES what any other value means: the concept is not stable,
// and filters to current knowledge exclude it. So a value outside this set is
// permitted and keeps that meaning (§11.2 rule 4: keep the value, MAY warn).
// It drives lint's advisory status-lifecycle note and IsStable/StatusKnown,
// never a validate rejection.
var LifecycleStatuses = map[string]bool{
	"draft":      true,
	"stable":     true,
	"deprecated": true,
}

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
		by := stringField(m, "by")    // §5.2: by is REQUIRED within generated.
		at, dated := okfTime(m["at"]) // §5: an RFC 3339 date-time or full-date.
		if by != "" || dated {
			return Generation{By: Actor(by), At: at}, true
		}
	}
	// §12.1 / Appendix B: legacy timestamp fallback. It keeps its historical
	// reader: it is a v0.1 key, not one of the §5 timestamp keys.
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

// TrustTier derives the trust tier from the CURRENT verification events (v0.3
// §5.3, a §13.1 breaking change): no current event ⇒ unverified; current events
// from non-human actors only ⇒ machine-confirmed; a current human:<id> event ⇒
// human-reviewed. Earlier events stay in the file as history and never raise
// the tier, so the tier can only go down relative to v0.2 (the safe direction).
// Derived, never stored.
func (n *Node) TrustTier() TrustTier {
	events := n.CurrentVerified()
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

// CurrentVerified returns the verification events that are current (v0.3
// §5.3): an event is current when its `at` is equal to or later than
// `generated.at`, or when `generated` is absent. Consumers MUST calculate the
// trust tier only from these (§11.2).
//
// The basis is Generated(), so a v0.1 bundle's legacy `timestamp` stands in
// for generated.at (§12.1 fallback). A `generated` family that is present but
// has no usable `at` gives no basis to compare against; okfctl then treats no
// event as current. §5.3 names only "generated is absent" as the everything-is-
// current case, and when a concept says it was generated at an unknown time,
// claiming a human reviewed the current text would be the unsafe reading.
func (n *Node) CurrentVerified() []Verification {
	events := n.Verified()
	g, hasGen := n.Generated()
	if !hasGen {
		return events // §5.3: generated is absent ⇒ every event is current.
	}
	if g.At.IsZero() {
		return nil
	}
	var out []Verification
	for _, e := range events {
		if !e.At.Before(g.At) { // at >= generated.at
			out = append(out, e)
		}
	}
	return out
}

// Status returns the raw lifecycle status (§5.4). Absent ⇒ stable. An unknown
// value is returned as written: §5.4 and §11.2 say consumers SHOULD show the
// raw value. Use IsStable for its meaning.
func (n *Node) Status() string {
	if v, ok := n.Frontmatter["status"].(string); ok && v != "" {
		return v
	}
	return StatusStable // §5.4: absent status ⇒ stable.
}

// StatusKnown reports whether the status is one of the §5.4 values (draft,
// stable, deprecated), counting an absent status as stable.
func (n *Node) StatusKnown() bool { return LifecycleStatuses[n.Status()] }

// IsStable reports whether the concept is stable (§5.4): status absent or
// `stable`. "A value that is not in this list is not stable", so an unknown
// value, like draft and deprecated, is not stable. Filters to current knowledge
// exclude every concept for which IsStable is false.
func (n *Node) IsStable() bool { return n.Status() == StatusStable }

// Epistemic returns the node's `epistemic` grade value, and whether the key is
// present. `epistemic` is NOT an OKF-defined field: it is an unknown frontmatter
// key (§11) the corpus carries to preserve the pre-v0.2 conflated grade verbatim
// (migration 13.1). okfctl RECOGNIZES it — surfacing its value distribution in
// analyze — but never enum-gates it, because §11 forbids rejecting a bundle for
// an unknown key or its values. A non-string or empty value reports present=true
// with the value rendered as text, so a typo is surfaced rather than dropped.
func (n *Node) Epistemic() (string, bool) {
	v, ok := n.Frontmatter["epistemic"]
	if !ok {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return scalarString(v), true
}

// StaleAfter returns the absolute stale-after instant (§5.5). ok is false when
// the field is absent or unparseable; StaleAfterMalformed tells those apart.
func (n *Node) StaleAfter() (time.Time, bool) {
	return okfTime(n.Frontmatter["stale_after"])
}

// StaleAfterMalformed reports whether stale_after is present but is not a §5
// timestamp. §5.5: a consumer SHOULD show such a value as malformed. An empty
// or null stale_after is the same as an absent key (§5.5, §11.2 rule 1) and is
// not malformed.
func (n *Node) StaleAfterMalformed() bool {
	raw, present := n.Frontmatter["stale_after"]
	if !present || isAbsentValue(raw) {
		return false
	}
	_, ok := okfTime(raw)
	return !ok
}

// IsStale reports whether the node is stale as of `now` (§5.5): stale when
// now >= stale_after. A node with no stale_after (absent, null, or empty) is
// never stale. A stale_after that does not parse is ALWAYS stale: §5.5 and
// §11.2 say a consumer MUST NOT treat it as fresh.
func (n *Node) IsStale(now time.Time) bool {
	if n.StaleAfterMalformed() {
		return true // §5.5: never fresh.
	}
	sa, ok := n.StaleAfter()
	if !ok {
		return false
	}
	return !now.Before(sa) // now >= stale_after
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

// parseVerification reads a { by, at } mapping into a Verification. v0.3 §5.2:
// both keys are REQUIRED in an event, and consumers ignore an event that does
// not have both. ok is false unless `by` is a non-empty actor AND `at` is a
// valid §5 timestamp. lintVerifiedEvents warns about each ignored event
// (§11.2 rule 3).
func parseVerification(m map[string]any) (Verification, bool) {
	by := stringField(m, "by")
	at, dated := okfTime(m["at"])
	if by == "" || !dated {
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
