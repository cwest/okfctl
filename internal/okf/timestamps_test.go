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
	"testing"
	"time"
)

// withClock swaps the package clock for a fixed instant and restores it.
func withClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := nowUTC
	nowUTC = func() time.Time { return at }
	t.Cleanup(func() { nowUTC = prev })
}

func TestNewNodeStampsCreatedAndModified(t *testing.T) {
	at := time.Date(2026, 7, 26, 15, 4, 5, 0, time.UTC)
	withClock(t, at)

	root := t.TempDir()
	if _, err := NewNode(root, "wine/tannin.md", "Concept", "Tannin"); err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	n := b.Nodes["wine/tannin.md"]
	if n == nil {
		t.Fatalf("node not loaded")
	}
	want := at.Format(time.RFC3339)
	if got := fmString(n.Frontmatter["created"]); got != want {
		t.Fatalf("created = %q, want %q", got, want)
	}
	if got := fmString(n.Frontmatter["modified"]); got != want {
		t.Fatalf("modified = %q, want %q", got, want)
	}
}

// fmString renders a frontmatter value the way FrontmatterTime-agnostic callers
// compare timestamps: yaml.v3 parses an RFC3339 scalar into a time.Time, so a
// round-tripped stamp is a time.Time, not a string.
func fmString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func TestTouchModifiedLeavesCreatedFixed(t *testing.T) {
	created := time.Date(2026, 7, 26, 15, 4, 5, 0, time.UTC)
	fm := map[string]any{"type": "Concept"}

	stampCreated(fm, created)
	if fm["created"] != created.Format(time.RFC3339) {
		t.Fatalf("stampCreated created = %v", fm["created"])
	}
	if fm["modified"] != created.Format(time.RFC3339) {
		t.Fatalf("stampCreated modified = %v", fm["modified"])
	}

	later := created.Add(48 * time.Hour)
	touchModified(fm, later)
	if fm["created"] != created.Format(time.RFC3339) {
		t.Fatalf("touchModified rewrote created to %v; must stay %v", fm["created"], created.Format(time.RFC3339))
	}
	if fm["modified"] != later.Format(time.RFC3339) {
		t.Fatalf("touchModified modified = %v, want %v", fm["modified"], later.Format(time.RFC3339))
	}
}

// touchModified must not invent a created timestamp for a node that never had
// one (an $EDITOR-authored node): it advances modified only.
func TestTouchModifiedDoesNotInventCreated(t *testing.T) {
	fm := map[string]any{"type": "Concept"}
	at := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	touchModified(fm, at)
	if _, ok := fm["created"]; ok {
		t.Fatalf("touchModified invented created = %v; must leave it absent", fm["created"])
	}
	if fm["modified"] != at.Format(time.RFC3339) {
		t.Fatalf("modified = %v, want %v", fm["modified"], at.Format(time.RFC3339))
	}
}
