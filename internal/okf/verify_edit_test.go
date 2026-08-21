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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ValidActor accepts exactly the three §7 actor forms and rejects everything
// else. A tool that guesses at who is a tool that manufactures trust, so the
// gate is strict about form (it does NOT validate the id's contents beyond
// non-emptiness — §7 leaves the id opaque).
func TestValidActor_Section7Forms(t *testing.T) {
	valid := []string{
		"human:ahormati",                 // §7: human:<id>
		"human:casey",                    // §7: human:<id>
		"process:finance-nightly",        // §7: process:<id>
		"reference_agent/gemini-2.5-pro", // §7: <producer>/<version>
		"okfctl/0.2",                     // §7: <producer>/<version>
	}
	for _, a := range valid {
		if !ValidActor(a) {
			t.Errorf("ValidActor(%q) = false, want true (a valid §7 form)", a)
		}
	}

	invalid := []string{
		"",         // empty is never valid
		"casey",    // bare id, no §7 prefix or producer/version slash
		"human:",   // human: prefix but empty id
		"process:", // process: prefix but empty id
		"/1.0",     // slash form but empty producer
		"tool/",    // slash form but empty version
		"me",       // the exact "--by me" anti-pattern the card forbids
		"human",    // no colon
		"process",  // no colon
	}
	for _, a := range invalid {
		if ValidActor(a) {
			t.Errorf("ValidActor(%q) = true, want false (not a §7 form)", a)
		}
	}
}

// verifyEditBundleFile writes a node file with the given frontmatter body and
// returns its absolute path.
func verifyEditNodeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, "n.md")
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

// AppendVerifiedFile APPENDS a verification event to a node with no prior
// `verified` key: a one-element list is created, created/modified are untouched,
// and the body is preserved verbatim. §5.2: verified is a list because
// verification history IS history.
func TestAppendVerifiedFile_NoPriorVerified_CreatesList(t *testing.T) {
	orig := "---\ntype: Concept\ntitle: Tannin\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\n---\n\n# Tannin\n\nBody text.\n"
	abs := verifyEditNodeFile(t, orig)
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	if err := AppendVerifiedFile(abs, "human:casey", at); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}

	// Reload and assert the model reads one human-reviewed event.
	b, err := Load(filepath.Dir(abs))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	n := b.Nodes["n.md"]
	events := n.Verified()
	if len(events) != 1 {
		t.Fatalf("want 1 verified event, got %d (%+v)", len(events), events)
	}
	if events[0].By != "human:casey" {
		t.Errorf("verified[0].by = %q, want human:casey", events[0].By)
	}
	if !events[0].At.Equal(at) {
		t.Errorf("verified[0].at = %v, want %v", events[0].At, at)
	}

	got := readNodeStr(t, abs)
	// created/modified are never touched by verify.
	if !strings.Contains(got, "created: 2026-01-01T00:00:00Z") {
		t.Errorf("created must be immutable:\n%s", got)
	}
	if !strings.Contains(got, "modified: 2026-07-01T00:00:00Z") {
		t.Errorf("modified must not be touched by verify:\n%s", got)
	}
	// Body preserved verbatim.
	if !strings.Contains(got, "# Tannin\n\nBody text.\n") {
		t.Errorf("body not preserved:\n%s", got)
	}
}

// AppendVerifiedFile APPENDS to an existing verified LIST without disturbing the
// prior entry: the earlier event stays first, the new one lands last. Append,
// never replace (§5.2).
func TestAppendVerifiedFile_AppendsToExistingList(t *testing.T) {
	orig := "---\ntype: Concept\ntitle: Acidity\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\nverified:\n  - { by: process:nightly, at: 2026-06-01T00:00:00Z }\n---\n\n# Acidity\n"
	abs := verifyEditNodeFile(t, orig)
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	if err := AppendVerifiedFile(abs, "human:casey", at); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}

	b, err := Load(filepath.Dir(abs))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	events := b.Nodes["n.md"].Verified()
	if len(events) != 2 {
		t.Fatalf("want 2 verified events, got %d (%+v)", len(events), events)
	}
	if events[0].By != "process:nightly" {
		t.Errorf("prior event must be preserved first; got events[0].by = %q", events[0].By)
	}
	if events[1].By != "human:casey" || !events[1].At.Equal(at) {
		t.Errorf("new event must be appended last; got events[1] = %+v", events[1])
	}
}

// A BARE verified MAPPING (not a list) is normalized to a two-element list per
// §5.2's one-element-list rule when a second event is appended — the prior
// verifier is NOT corrupted or dropped. Fixture required (card done-when).
func TestAppendVerifiedFile_BareMappingNormalizedToList(t *testing.T) {
	orig := "---\ntype: Concept\ntitle: Mouthfeel\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\nverified: { by: human:ahormati, at: 2026-06-25T09:00:00Z }\n---\n\n# Mouthfeel\n"
	abs := verifyEditNodeFile(t, orig)
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	if err := AppendVerifiedFile(abs, "human:casey", at); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}

	b, err := Load(filepath.Dir(abs))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	events := b.Nodes["n.md"].Verified()
	if len(events) != 2 {
		t.Fatalf("bare mapping must normalize to a 2-element list, got %d (%+v)", len(events), events)
	}
	if events[0].By != "human:ahormati" {
		t.Errorf("bare-mapping verifier must be preserved as element 0; got %q", events[0].By)
	}
	if events[1].By != "human:casey" {
		t.Errorf("appended verifier must be element 1; got %q", events[1].By)
	}
	// The result must still validate as a well-formed bundle.
	if fs := Validate(b); len(fs) != 0 {
		t.Fatalf("node must still pass validate after append; findings: %+v", fs)
	}
}

// The append is order-preserving: existing frontmatter keys keep their order and
// `verified` is appended (or extended) without reordering created/type/title.
func TestAppendVerifiedFile_PreservesKeyOrderAndBody(t *testing.T) {
	orig := "---\ntype: Concept\ntitle: Body\ncreated: 2026-01-01T00:00:00Z\nmodified: 2026-07-01T00:00:00Z\ntags: [wine, tasting]\n---\n\n# Body\n\nParagraph one.\n\nParagraph two.\n"
	abs := verifyEditNodeFile(t, orig)
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	if err := AppendVerifiedFile(abs, "human:casey", at); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}
	got := readNodeStr(t, abs)

	// The pre-existing keys keep their relative order.
	order := []string{"type:", "title:", "created:", "modified:", "tags:", "verified:"}
	last := -1
	for _, k := range order {
		i := strings.Index(got, k)
		if i < 0 {
			t.Fatalf("key %q missing after append:\n%s", k, got)
		}
		if i < last {
			t.Fatalf("key %q out of order after append:\n%s", k, got)
		}
		last = i
	}
	// Body preserved verbatim including blank lines.
	if !strings.Contains(got, "# Body\n\nParagraph one.\n\nParagraph two.\n") {
		t.Errorf("body not preserved verbatim:\n%s", got)
	}
}

// A node with no frontmatter block at all gains a minimal one carrying the
// verified event — mirroring TouchModifiedFile's no-frontmatter behavior.
func TestAppendVerifiedFile_NoFrontmatter(t *testing.T) {
	orig := "# Just a body\n\nNo frontmatter here.\n"
	abs := verifyEditNodeFile(t, orig)
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	if err := AppendVerifiedFile(abs, "human:casey", at); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}
	got := readNodeStr(t, abs)
	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("expected a frontmatter block to be prepended:\n%s", got)
	}
	if !strings.Contains(got, "# Just a body\n\nNo frontmatter here.\n") {
		t.Errorf("body must be preserved:\n%s", got)
	}
}

func readNodeStr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
