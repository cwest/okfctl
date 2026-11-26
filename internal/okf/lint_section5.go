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
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Lint detectors for the OKF v0.3 §5 frontmatter semantics. Each is a LINT
// finding, never a validate failure: §11.1 defines bundle conformance as
// parseable frontmatter plus reserved files that agree with §8/§9, and §11.2
// says that for absent, empty, or unexpected-shape values "the bundle stays
// conformant". Rejecting any of these in validate would be over-conformance.

// timestampFinding is one §5 timestamp key whose value is not a valid form.
type timestampFinding struct {
	key string
	val any
}

// section5Timestamps returns the §5 timestamp keys in n whose value is present
// (not null/empty, §11.2 rule 1) but is neither an RFC 3339 date-time with an
// offset nor a full-date. The key list is exactly §5's: generated.at,
// verified[].at, stale_after, sources[].last_modified, usage_window.from/to
// (shared and per-entry). The legacy v0.1 `timestamp` is not a §5 key (§12.1).
func section5Timestamps(n *Node) []timestampFinding {
	var out []timestampFinding
	check := func(key string, v any, present bool) {
		if !present || isAbsentValue(v) {
			return
		}
		if _, ok := okfTime(v); !ok {
			out = append(out, timestampFinding{key: key, val: v})
		}
	}
	window := func(prefix string, v any) {
		if m, ok := asStringMap(v); ok {
			from, hasFrom := m["from"]
			check(prefix+".from", from, hasFrom)
			to, hasTo := m["to"]
			check(prefix+".to", to, hasTo)
		}
	}
	fm := n.Frontmatter
	if g, ok := asStringMap(fm["generated"]); ok {
		at, has := g["at"]
		check("generated.at", at, has)
	}
	for i, ev := range listItems(fm["verified"]) {
		if m, ok := asStringMap(ev); ok {
			at, has := m["at"]
			check(fmt.Sprintf("verified[%d].at", i), at, has)
		}
	}
	sa, hasSA := fm["stale_after"]
	check("stale_after", sa, hasSA)
	for i, s := range listItems(fm["sources"]) {
		if m, ok := asStringMap(s); ok {
			lm, has := m["last_modified"]
			check(fmt.Sprintf("sources[%d].last_modified", i), lm, has)
			window(fmt.Sprintf("sources[%d].usage_window", i), m["usage_window"])
		}
	}
	window("usage_window", fm["usage_window"])
	return out
}

// listItems applies §11.2 rule 2 to a list-valued family: a list yields its
// items, and one non-list, non-absent value yields itself as a one-item list.
func listItems(v any) []any {
	if isAbsentValue(v) {
		return nil
	}
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

// renderValue shows a frontmatter value as the author wrote it, for messages.
// A mapping renders in YAML flow style with sorted keys ({ by: human:x }).
func renderValue(v any) string {
	switch t := v.(type) {
	case string:
		return strconv.Quote(t)
	case time.Time:
		return t.Format(time.RFC3339Nano)
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+fmt.Sprintf("%v", t[k]))
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	}
	return fmt.Sprintf("%v", v)
}

// lintTimestampForms flags a §5 timestamp key whose value is not a §5 form.
// v0.3 §5: "No other form is valid", and §13.2: "A date-time without an offset
// stays invalid." The value is kept (§11.2 rule 4) and the bundle stays
// conformant, so this is advisory curation guidance. For stale_after the
// message also states the §5.5 consequence: the concept is never fresh.
func lintTimestampForms(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		if n.Frontmatter == nil {
			continue
		}
		for _, tf := range section5Timestamps(n) {
			msg := fmt.Sprintf(
				"timestamp-form: %s %s = %s is not a §5 timestamp (an RFC 3339 date-time with an offset, e.g. \"2026-06-30T14:00:00Z\", or a full-date, e.g. \"2026-06-30\"); a date-time without an offset is not valid",
				p, tf.key, renderValue(tf.val))
			if tf.key == "stale_after" {
				msg += "; §5.5: a stale_after that does not parse is treated as stale, never fresh"
			}
			out = append(out, LintFinding{Check: "timestamp-form", Path: p, Message: msg})
		}
	}
	return out
}

// lintVerifiedEvents flags a `verified` event that lacks its REQUIRED `by` or
// `at` (v0.3 §5.2: "Both keys are REQUIRED in an event. Consumers ignore an
// event that does not have both."). §11.2 rule 3: consumers ignore such an
// entry and SHOULD warn about it; this is that warning. A present-but-malformed
// `at` is reported by lintTimestampForms instead, so one defect is one finding.
func lintVerifiedEvents(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		if n.Frontmatter == nil {
			continue
		}
		for i, ev := range listItems(n.Frontmatter["verified"]) {
			m, ok := asStringMap(ev)
			var missing []string
			if !ok {
				missing = []string{"by", "at"} // not a { by, at } mapping at all
			} else {
				if stringField(m, "by") == "" {
					missing = append(missing, "by")
				}
				if isAbsentValue(m["at"]) {
					missing = append(missing, "at")
				}
			}
			if len(missing) == 0 {
				continue
			}
			out = append(out, LintFinding{
				Check: "verified-event",
				Path:  p,
				Message: fmt.Sprintf(
					"verified-event: %s verified[%d] = %s has no %s; §5.2 requires both by and at, so consumers ignore this event and it does not count toward the trust tier",
					p, i, renderValue(ev), strings.Join(missing, " or ")),
			})
		}
	}
	return out
}

// uriScheme matches an RFC 3986 scheme prefix (§6 rule 1: a value with a URI
// scheme is an absolute URL).
var uriScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*:`)

// resolveConceptPath resolves a link target or path field value written in the
// file at fromPath, per v0.3 §6. It returns:
//   - external=true for an absolute URL (§6 rule 1) or a value that resolves
//     outside the bundle root (§6: external; consumers MUST NOT read it);
//   - otherwise the bundle-relative target and whether it is a concept node.
//
// Rule 2: a leading `/` resolves from the bundle root. Rule 3: any other value
// resolves from the directory of the file. With rootFallback (§6.2 / §12.1: a
// bundle that declares okf_version 0.2 or earlier, or no version), a bare value
// that does not resolve from its file MAY resolve from the bundle root. A
// `#fragment` is not part of the path (§6.1). Values are URI references, so a
// percent-encoded value is decoded (§6). Paths are case-sensitive.
func resolveConceptPath(b *Bundle, fromPath, value string, rootFallback bool) (target string, concept, external bool) {
	v := strings.TrimSpace(value)
	if uriScheme.MatchString(v) {
		return "", false, true
	}
	if i := strings.IndexByte(v, '#'); i >= 0 {
		v = v[:i]
	}
	if dec, err := url.PathUnescape(v); err == nil {
		v = dec
	}
	inBundle := func(p string) (string, bool) {
		c := path.Clean(p)
		if c == ".." || strings.HasPrefix(c, "../") {
			return "", false
		}
		return c, true
	}
	if strings.HasPrefix(v, "/") {
		t, ok := inBundle(strings.TrimPrefix(v, "/"))
		if !ok {
			return "", false, true
		}
		return t, b.Nodes[t] != nil, false
	}
	t, ok := inBundle(path.Join(path.Dir(fromPath), v))
	if !ok {
		return "", false, true
	}
	if b.Nodes[t] != nil {
		return t, true, false
	}
	if rootFallback {
		if rt, ok := inBundle(v); ok && b.Nodes[rt] != nil {
			return rt, true, false
		}
	}
	return t, false, false
}

// bundleRootFallback reports whether the §6.2 / §12.1 v0.2 bundle-root
// fallback applies: the bundle declares okf_version 0.2 or earlier, or declares
// no version. The declaration is read from the bundle-root index.md
// frontmatter (§8.1, §12), then from okfctl's .okf sidecar. A version okfctl
// cannot parse gets best-effort treatment (§12): the fallback stays on, which
// is the direction that produces fewer findings.
func bundleRootFallback(b *Bundle) bool {
	declared := ""
	if idx, ok := b.Reserved["index.md"]; ok && idx.Frontmatter != nil {
		if v, ok := idx.Frontmatter["okf_version"]; ok {
			declared = strings.TrimSpace(scalarString(v))
		}
	}
	if declared == "" && b.Root != "" {
		declared = strings.Trim(strings.TrimSpace(readOkfSidecar(b.Root)["okf_version"]), `"'`)
	}
	if declared == "" {
		return true // declares no version
	}
	major, minor, ok := parseOkfVersion(declared)
	if !ok {
		return true // unknown version: best-effort (§12)
	}
	return major == 0 && minor <= 2
}

// parseOkfVersion parses an OKF `<major>.<minor>` version (§12).
func parseOkfVersion(s string) (major, minor int, ok bool) {
	a, c, found := strings.Cut(s, ".")
	if !found {
		return 0, 0, false
	}
	ma, err1 := strconv.Atoi(a)
	mi, err2 := strconv.Atoi(c)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return ma, mi, true
}

// lintSupersededBy checks the v0.3 §5.4 `superseded_by` path field:
//   - it has a meaning only with `status: deprecated`, so on any other status
//     it is a curation slip;
//   - it is "one path (§6.2) to the concept that replaces this concept", so a
//     value that is not a string, or that resolves (§6) to no concept in the
//     bundle, gives a consumer that follows it nowhere to go.
//
// Advisory only: §6.1 and §11.2 say consumers MUST NOT reject a bundle for a
// broken link. An external target (a URL, or a path outside the bundle root)
// is not checked: §6 forbids reading it from the local file system. A null or
// empty value is absent (§11.2 rule 1).
func lintSupersededBy(b *Bundle) []LintFinding {
	fallback := bundleRootFallback(b)
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		if n.Frontmatter == nil {
			continue
		}
		raw, present := n.Frontmatter["superseded_by"]
		if !present || isAbsentValue(raw) {
			continue
		}
		add := func(msg string) {
			out = append(out, LintFinding{Check: "superseded-by", Path: p, Message: "superseded-by: " + p + " " + msg})
		}
		if st := n.Status(); st != "deprecated" {
			add(fmt.Sprintf("sets superseded_by but has status %q; §5.4: superseded_by has a meaning only with status: deprecated", st))
		}
		val, ok := raw.(string)
		if !ok {
			add(fmt.Sprintf("superseded_by = %s is not a path; §5.4: it is one path (§6.2) to the replacing concept", renderValue(raw)))
			continue
		}
		target, concept, external := resolveConceptPath(b, p, val, fallback)
		if external || concept {
			continue
		}
		add(fmt.Sprintf("superseded_by = %q resolves to %s, which is not a concept in the bundle; §5.4: consumers that filter to current knowledge follow it to the replacing concept", val, target))
	}
	return out
}
