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

// Reader-level tests for the OKF v0.3 §5 semantics: the timestamp forms (§5),
// verification events (§5.2), current-only trust tiers (§5.3), the unknown
// status meaning (§5.4), and the never-fresh rule for an unparseable
// stale_after (§5.5).

// --- §5 timestamp forms ------------------------------------------------------

func TestParseOKFTimestamp_AcceptsRFC3339DateTimeWithOffsetAndFullDate_Section5(t *testing.T) {
	cases := map[string]time.Time{
		"2026-06-30T14:00:00Z":        time.Date(2026, 6, 30, 14, 0, 0, 0, time.UTC),
		"2026-06-30T14:00:00-04:00":   time.Date(2026, 6, 30, 18, 0, 0, 0, time.UTC),
		"2026-06-30T14:00:00.5+02:00": time.Date(2026, 6, 30, 12, 0, 0, 5e8, time.UTC),
		// RFC 3339 §5.6: "T" and "Z" may be lowercase.
		"2026-06-30t14:00:00z": time.Date(2026, 6, 30, 14, 0, 0, 0, time.UTC),
		// §5 full-date: the start of that day in UTC.
		"2026-06-30": time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	}
	for in, want := range cases {
		got, ok := parseOKFTimestamp(in)
		if !ok {
			t.Errorf("parseOKFTimestamp(%q) ok = false, want true (§5 valid form)", in)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("parseOKFTimestamp(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseOKFTimestamp_RejectsDateTimeWithoutOffsetAndNonRFC3339_Section5(t *testing.T) {
	for _, in := range []string{
		"2026-06-20T10:00:00", // §5: a date-time without an offset stays invalid.
		"2026-06-20 10:00:00",
		"2026-06-20T10:00",
		"not-a-date",
		"2026-6-2", // not RFC 3339 full-date (two-digit month/day).
		"2026-06-20T10:00:00+0200",
		"",
	} {
		if _, ok := parseOKFTimestamp(in); ok {
			t.Errorf("parseOKFTimestamp(%q) ok = true, want false (§5: no other form is valid)", in)
		}
	}
}

// §5 / gap-analysis Q4: yaml.v3 decodes a NATIVE `2026-06-20 10:00:00` into a
// UTC time.Time and drops the fact that there was no offset. The parser keeps
// such a value as its source text at the §5 timestamp keys, so no reader can
// mistake it for an instant.
func TestFrontmatter_NativeNoOffsetTimestampIsNotAnInstant_Section5(t *testing.T) {
	src := "---\n" +
		"generated: { by: human:x, at: 2026-06-20 10:00:00 }\n" +
		"stale_after: 2026-09-23 00:00:00\n" +
		"verified:\n  - { by: human:x, at: 2026-06-21 10:00:00 }\n" +
		"sources:\n  - { resource: https://x.test, last_modified: 2026-05-30 10:00:00, usage_window: { from: 2026-06-01 00:00:00, to: 2026-06-30 } }\n" +
		"usage_window: { from: 2026-06-01 00:00:00, to: 2026-06-30 }\n" +
		"created: 2026-06-20 10:00:00\n" +
		"---\n\n# X\n"
	fm, _, err := ParseFrontmatter([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	gen := fm["generated"].(map[string]any)
	if v, ok := gen["at"].(string); !ok || v != "2026-06-20 10:00:00" {
		t.Errorf("generated.at = %T %v, want the source string (no offset ⇒ not an instant)", gen["at"], gen["at"])
	}
	if _, ok := fm["stale_after"].(string); !ok {
		t.Errorf("stale_after = %T, want string", fm["stale_after"])
	}
	ver := fm["verified"].([]any)[0].(map[string]any)
	if _, ok := ver["at"].(string); !ok {
		t.Errorf("verified[0].at = %T, want string", ver["at"])
	}
	src0 := fm["sources"].([]any)[0].(map[string]any)
	if _, ok := src0["last_modified"].(string); !ok {
		t.Errorf("sources[0].last_modified = %T, want string", src0["last_modified"])
	}
	if _, ok := src0["usage_window"].(map[string]any)["from"].(string); !ok {
		t.Errorf("sources[0].usage_window.from should be a string")
	}
	uw := fm["usage_window"].(map[string]any)
	if _, ok := uw["from"].(string); !ok {
		t.Errorf("usage_window.from = %T, want string", uw["from"])
	}
	// Negative control: a valid native date stays a native timestamp.
	if _, ok := uw["to"].(time.Time); !ok {
		t.Errorf("usage_window.to = %T, want time.Time (a valid native full-date is accepted)", uw["to"])
	}
	// Negative control: keys §5 does not define (okfctl's own `created`) are
	// left exactly as yaml decodes them.
	if _, ok := fm["created"].(time.Time); !ok {
		t.Errorf("created = %T, want time.Time (not a §5 timestamp key; untouched)", fm["created"])
	}
}

// §5: "Consumers MUST accept a quoted string and a YAML-native timestamp." The
// positive control for the reader: every valid form, quoted or native, reads.
func TestGenerated_AcceptsQuotedAndNativeTimestamps_Section5(t *testing.T) {
	want := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	for _, at := range []string{
		`"2026-06-20T10:00:00Z"`, `2026-06-20T10:00:00Z`,
		`"2026-06-20T06:00:00-04:00"`, `2026-06-20T06:00:00-04:00`,
	} {
		n := &Node{Frontmatter: mustFM(t, "generated: { by: human:x, at: "+at+" }\n")}
		g, ok := n.Generated()
		if !ok || !g.At.Equal(want) {
			t.Errorf("at=%s: Generated() = %+v ok=%v, want At %v", at, g, ok, want)
		}
	}
	for _, at := range []string{`"2026-06-20"`, `2026-06-20`} {
		n := &Node{Frontmatter: mustFM(t, "generated: { by: human:x, at: "+at+" }\n")}
		g, ok := n.Generated()
		if !ok || !g.At.Equal(time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("at=%s: Generated() = %+v ok=%v, want start of day UTC", at, g, ok)
		}
	}
}

// mustFM parses a frontmatter body through the PRODUCTION parser (not a bare
// yaml.Unmarshal), so the §5 timestamp handling is part of what is tested.
func mustFM(t *testing.T, yamlBody string) map[string]any {
	t.Helper()
	fm, _, err := ParseFrontmatter([]byte("---\n" + yamlBody + "---\n"))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	return fm
}

// --- §5.2 verification events require by AND at ------------------------------

func TestVerified_IgnoresEventMissingByOrAt_Section5_2(t *testing.T) {
	for name, body := range map[string]string{
		"bare mapping, no at": "verified: { by: human:x }\n",
		"bare mapping, no by": "verified: { at: 2026-06-25T09:00:00Z }\n",
		"list, no at":         "verified:\n  - { by: human:x }\n",
		"malformed at":        "verified: { by: human:x, at: 2026-06-25T09:00:00 }\n",
	} {
		n := &Node{Frontmatter: mustFM(t, body)}
		if got := n.Verified(); len(got) != 0 {
			t.Errorf("%s: Verified() = %+v, want none (§5.2: consumers ignore an event without both by and at)", name, got)
		}
		if got := n.TrustTier(); got != TrustUnverified {
			t.Errorf("%s: TrustTier() = %q, want unverified", name, got)
		}
	}
}

func TestVerified_KeepsCompleteEventsBesideIncompleteOnes_Section5_2(t *testing.T) {
	// Negative control: the complete event in the same list still counts.
	n := &Node{Frontmatter: mustFM(t, "verified:\n  - { by: human:x }\n  - { by: process:nightly, at: 2026-06-26T02:00:00Z }\n")}
	got := n.Verified()
	if len(got) != 1 || got[0].By != "process:nightly" {
		t.Fatalf("Verified() = %+v, want only the complete process:nightly event", got)
	}
	if tier := n.TrustTier(); tier != TrustMachineConfirmed {
		t.Errorf("TrustTier() = %q, want machine-confirmed", tier)
	}
}

// --- §5.3 only current events give the trust tier (§13.1 breaking) -----------

func TestTrustTier_OnlyCurrentEventsCount_Section5_3(t *testing.T) {
	const gen = "generated: { by: reference_agent/x, at: \"2026-06-20T12:00:00Z\" }\n"
	cases := []struct {
		name string
		body string
		want TrustTier
	}{
		{"human sign-off before the last change is history", gen +
			"verified: { by: human:x, at: \"2026-06-20T11:59:59Z\" }\n", TrustUnverified},
		{"equal instant is current (at >= generated.at)", gen +
			"verified: { by: human:x, at: \"2026-06-20T12:00:00Z\" }\n", TrustHumanReviewed},
		{"stale human + current machine ⇒ machine-confirmed", gen +
			"verified:\n  - { by: human:x, at: \"2026-06-19T00:00:00Z\" }\n  - { by: process:nightly, at: \"2026-06-21T00:00:00Z\" }\n", TrustMachineConfirmed},
		{"offsets compare as instants", gen +
			"verified: { by: human:x, at: \"2026-06-20T08:00:00-04:00\" }\n", TrustHumanReviewed},
		{"no generated ⇒ every event is current",
			"verified: { by: human:x, at: \"2020-01-01T00:00:00Z\" }\n", TrustHumanReviewed},
		{"legacy timestamp is the basis when generated is absent (§12.1)",
			"timestamp: \"2026-06-20T12:00:00Z\"\nverified: { by: human:x, at: \"2026-06-01T00:00:00Z\" }\n", TrustUnverified},
		{"generated present without a usable at ⇒ no event is current",
			"generated: { by: reference_agent/x }\nverified: { by: human:x, at: \"2026-06-21T00:00:00Z\" }\n", TrustUnverified},
	}
	for _, c := range cases {
		n := &Node{Frontmatter: mustFM(t, c.body)}
		if got := n.TrustTier(); got != c.want {
			t.Errorf("%s: TrustTier() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCurrentVerified_KeepsHistoryInVerified_Section5_3(t *testing.T) {
	// §5.3: earlier events "stay in the file as history". Verified() still
	// returns them; CurrentVerified() is the trust-tier input.
	n := &Node{Frontmatter: mustFM(t, "generated: { by: a/b, at: \"2026-06-20T00:00:00Z\" }\n"+
		"verified:\n  - { by: human:x, at: \"2026-06-01T00:00:00Z\" }\n  - { by: process:y, at: \"2026-06-21T00:00:00Z\" }\n")}
	if got := len(n.Verified()); got != 2 {
		t.Errorf("Verified() len = %d, want 2 (history is kept)", got)
	}
	cur := n.CurrentVerified()
	if len(cur) != 1 || cur[0].By != "process:y" {
		t.Errorf("CurrentVerified() = %+v, want only process:y", cur)
	}
}

// --- §5.4 unknown status is not stable ---------------------------------------

func TestStatus_UnknownValueIsNotStable_Section5_4(t *testing.T) {
	cases := []struct {
		body         string
		raw          string
		stable, know bool
	}{
		{"type: X\n", "stable", true, true}, // absent ⇒ stable
		{"status: stable\n", "stable", true, true},
		{"status: draft\n", "draft", false, true},
		{"status: deprecated\n", "deprecated", false, true},
		{"status: frobnicated\n", "frobnicated", false, false},
	}
	for _, c := range cases {
		n := &Node{Frontmatter: mustFM(t, c.body)}
		// §5.4 / §11.2: consumers SHOULD show the raw value.
		if got := n.Status(); got != c.raw {
			t.Errorf("%q: Status() = %q, want raw %q", c.body, got, c.raw)
		}
		if got := n.IsStable(); got != c.stable {
			t.Errorf("%q: IsStable() = %v, want %v", c.body, got, c.stable)
		}
		if got := n.StatusKnown(); got != c.know {
			t.Errorf("%q: StatusKnown() = %v, want %v", c.body, got, c.know)
		}
	}
}

// --- §5.5 an unparseable stale_after is never fresh --------------------------

func TestIsStale_UnparseableStaleAfterIsNeverFresh_Section5_5(t *testing.T) {
	far := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC) // long before any value
	for _, v := range []string{"not-a-date", `"2026-06-20T10:00:00"`, "2026-06-20 10:00:00", "42"} {
		n := &Node{Frontmatter: mustFM(t, "stale_after: "+v+"\n")}
		if !n.IsStale(far) {
			t.Errorf("stale_after: %s: IsStale = false, want true (§5.5: MUST NOT treat as fresh)", v)
		}
		if !n.StaleAfterMalformed() {
			t.Errorf("stale_after: %s: StaleAfterMalformed = false, want true (§5.5 SHOULD show as malformed)", v)
		}
	}
}

func TestIsStale_NullOrEmptyStaleAfterIsAbsent_Section5_5(t *testing.T) {
	// Negative control: §5.5 / §11.2 rule 1, an empty or null stale_after is the
	// same as an absent key, so it is neither stale nor malformed.
	far := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, v := range []string{"", "null", "~", `""`} {
		n := &Node{Frontmatter: mustFM(t, "stale_after: "+v+"\n")}
		if n.IsStale(far) {
			t.Errorf("stale_after: %q: IsStale = true, want false (absent)", v)
		}
		if n.StaleAfterMalformed() {
			t.Errorf("stale_after: %q: StaleAfterMalformed = true, want false", v)
		}
	}
	// §11.2 rule 1 makes only the EMPTY string absent. A whitespace-only value
	// is a present value that does not parse, so §5.5 makes it never fresh.
	ws := &Node{Frontmatter: mustFM(t, "stale_after: \"   \"\n")}
	if !ws.IsStale(far) || !ws.StaleAfterMalformed() {
		t.Error(`stale_after: "   ": want stale and malformed (present, unparseable; §11.2 rule 1, §5.5)`)
	}
	// And a valid future value is fresh.
	n := &Node{Frontmatter: mustFM(t, "stale_after: \"2099-06-01T00:00:00Z\"\n")}
	if n.IsStale(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("valid future stale_after: IsStale = true, want false")
	}
}
