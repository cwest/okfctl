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
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// OKF v0.3 §5 timestamps. The timestamp keys are generated.at, verified[].at,
// stale_after, sources[].last_modified, usage_window.from, and usage_window.to.
// Each value is an RFC 3339 `date-time` (with an offset) or an RFC 3339
// `full-date` (the start of that day in UTC). "No other form is valid", and a
// date-time without an offset "stays invalid" (§13.2). Consumers MUST accept a
// quoted string and a YAML-native timestamp.

var (
	// rfc3339FullDate is RFC 3339 §5.6 full-date: four-digit year, two-digit
	// month and day.
	rfc3339FullDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// rfc3339DateTime is RFC 3339 §5.6 date-time: full-date "T" partial-time
	// time-offset. The offset is mandatory; "T" and "Z" are case-insensitive.
	rfc3339DateTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]\d{2}:\d{2})$`)
)

// parseOKFTimestampLocal parses a §5 timestamp string, keeping the value's own
// offset. ok is false for any form §5 does not allow.
func parseOKFTimestampLocal(s string) (time.Time, bool) {
	switch {
	case rfc3339FullDate.MatchString(s):
		t, err := time.Parse("2006-01-02", s) // §5: start of that day in UTC.
		return t, err == nil
	case rfc3339DateTime.MatchString(s):
		norm := strings.ToUpper(s) // RFC 3339 allows lowercase "t" and "z".
		t, err := time.Parse(time.RFC3339Nano, norm)
		return t, err == nil
	}
	return time.Time{}, false
}

// parseOKFTimestamp parses a §5 timestamp string as an instant in UTC.
func parseOKFTimestamp(s string) (time.Time, bool) {
	t, ok := parseOKFTimestampLocal(s)
	if !ok {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// okfTime reads a decoded frontmatter value at a §5 timestamp key. A
// YAML-native timestamp arrives as time.Time (the parser has already demoted
// any native value §5 does not allow back to its source string, see
// demoteInvalidNativeTimestamps); a quoted value arrives as a string.
func okfTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), true
	case string:
		return parseOKFTimestamp(t)
	}
	return time.Time{}, false
}

// demoteInvalidNativeTimestamps walks a parsed frontmatter document and, at the
// §5 timestamp keys only, retags a YAML-native timestamp scalar that §5 does
// not allow (e.g. `2026-06-20 10:00:00`, no offset) as a plain string.
//
// yaml.v3 resolves such a scalar to a UTC time.Time and drops the fact that no
// offset was written, so without this step a reader could not tell an invalid
// native value from a valid one (gap-analysis Q4). Demoting it to its source
// text keeps the value (§11.2 rule 4: keep it) while making it read as
// unparseable everywhere: no instant, a timestamp-form lint finding, and for
// stale_after never fresh (§5.5). Keys §5 does not define (okfctl's own
// created/modified, the legacy timestamp) are left exactly as yaml decodes them.
func demoteInvalidNativeTimestamps(doc *yaml.Node) {
	root := doc
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	demote := func(n *yaml.Node) {
		if n != nil && n.Kind == yaml.ScalarNode && n.ShortTag() == "!!timestamp" {
			if _, ok := parseOKFTimestampLocal(n.Value); !ok {
				n.Tag = "!!str"
			}
		}
	}
	window := func(n *yaml.Node) {
		demote(mappingValue(n, "from"))
		demote(mappingValue(n, "to"))
	}
	demote(mappingValue(mappingValue(root, "generated"), "at"))
	demote(mappingValue(root, "stale_after"))
	window(mappingValue(root, "usage_window"))
	for _, ev := range itemsOf(mappingValue(root, "verified")) {
		demote(mappingValue(ev, "at"))
	}
	for _, src := range itemsOf(mappingValue(root, "sources")) {
		demote(mappingValue(src, "last_modified"))
		window(mappingValue(src, "usage_window"))
	}
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// itemsOf returns the items of a list node, or the node itself as a one-item
// list when it is a mapping (§11.2 rule 2: one item is a list with one item).
func itemsOf(n *yaml.Node) []*yaml.Node {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.SequenceNode:
		return n.Content
	case yaml.MappingNode:
		return []*yaml.Node{n}
	}
	return nil
}
