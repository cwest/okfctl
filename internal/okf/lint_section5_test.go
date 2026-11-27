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
	"testing"
)

// Lint detectors for the OKF v0.3 §5 semantics. Every detector carries a
// POSITIVE control (the defect is caught) and a NEGATIVE control (a look-alike
// that v0.3 permits stays silent). None of these is a validate failure: §11.1
// defines conformance without them and §11.2 says a bundle with absent, empty,
// or unexpected-shape values "stays conformant".

const lintIndex = "# Index\n\n- [A](a.md)\n- [B](b.md)\n"

func fmDoc(fm string) string {
	return "---\ntype: Concept\ntitle: A\n" + fm + "---\n\n# A\n\nBody.\n"
}

// --- timestamp-form (§5) -----------------------------------------------------

func TestLint_TimestampForm_FlagsDateTimeWithoutOffset_Section5(t *testing.T) {
	cases := map[string]string{
		"generated.at quoted":           "generated: { by: human:x, at: \"2026-06-20T10:00:00\" }\n",
		"generated.at native":           "generated: { by: human:x, at: 2026-06-20 10:00:00 }\n",
		"verified[].at":                 "verified:\n  - { by: human:x, at: \"2026-06-20T10:00:00\" }\n",
		"stale_after":                   "stale_after: \"2026-06-20T10:00:00\"\n",
		"sources[].last_modified":       "sources:\n  - { resource: https://x.test, last_modified: \"2026-05-30T10:00:00\" }\n",
		"usage_window.from":             "usage_window: { from: \"2026-06-01T00:00:00\", to: \"2026-06-30\" }\n",
		"sources[].usage_window.to":     "sources:\n  - { resource: https://x.test, usage_window: { from: \"2026-06-01\", to: \"2026-06-30T00:00:00\" } }\n",
		"stale_after unparseable":       "stale_after: not-a-date\n",
		"verified bare mapping no offs": "verified: { by: human:x, at: \"2026-06-20T10:00:00\" }\n",
	}
	for name, fm := range cases {
		b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc(fm), "b.md": fmDoc("")})
		fs := findingsFor(Lint(b, LintOptions{}), "timestamp-form")
		if len(fs) != 1 || fs[0].Path != "a.md" {
			t.Errorf("%s: want one timestamp-form finding on a.md, got %+v", name, fs)
			continue
		}
		if !strings.Contains(fs[0].Message, "§5") {
			t.Errorf("%s: message should cite §5: %q", name, fs[0].Message)
		}
	}
}

func TestLint_TimestampForm_ValidFormsSilent_Section5(t *testing.T) {
	// Negative control: every form v0.3 §5 permits, quoted and native, and the
	// §11.2 rule 1 null/empty values, are silent. So are timestamp-looking values
	// under keys §5 does not define (okfctl's own created/modified).
	fm := "generated: { by: human:x, at: 2026-06-20T10:00:00Z }\n" +
		"verified:\n  - { by: human:x, at: \"2026-06-21T10:00:00-04:00\" }\n  - { by: process:y, at: 2026-06-22 }\n" +
		"stale_after: \"2026-09-23\"\n" +
		"sources:\n  - { resource: https://x.test, last_modified: 2026-05-30, usage_window: { from: \"2026-06-01T00:00:00.5+02:00\", to: 2026-06-30 } }\n" +
		"usage_window: { from: 2026-06-01, to: \"2026-06-30T00:00:00Z\" }\n" +
		"created: 2026-06-20 10:00:00\nmodified: \"2026-06-20T10:00:00\"\n"
	b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc(fm), "b.md": fmDoc("stale_after:\nusage_window: { from: \"\", to: ~ }\n")})
	if fs := findingsFor(Lint(b, LintOptions{}), "timestamp-form"); len(fs) != 0 {
		t.Fatalf("valid §5 forms must be silent, got %+v", fs)
	}
}

func TestLint_TimestampForm_LegacyTimestampIsNotChecked_Section12_1(t *testing.T) {
	// Negative control: the v0.1 `timestamp` key is a §12.1 fallback, not one of
	// the six §5 timestamp keys. A v0.1 bundle must lint unchanged.
	b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc("timestamp: '2026-05-28T22:53:05'\n"), "b.md": fmDoc("")})
	if fs := findingsFor(Lint(b, LintOptions{}), "timestamp-form"); len(fs) != 0 {
		t.Fatalf("legacy timestamp is outside the §5 key list, got %+v", fs)
	}
}

// --- verified-event (§5.2, §11.2 rule 3) ------------------------------------

func TestLint_VerifiedEvent_FlagsEventMissingByOrAt_Section5_2(t *testing.T) {
	for name, fm := range map[string]string{
		"no at":            "verified: { by: human:x }\n",
		"no by":            "verified:\n  - { at: \"2026-06-25T09:00:00Z\" }\n",
		"scalar entry":     "verified:\n  - human:x\n",
		"empty by":         "verified: { by: \"\", at: \"2026-06-25T09:00:00Z\" }\n",
		"one of two empty": "verified:\n  - { by: human:x, at: \"2026-06-25T09:00:00Z\" }\n  - { by: process:y }\n",
	} {
		b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc(fm), "b.md": fmDoc("")})
		fs := findingsFor(Lint(b, LintOptions{}), "verified-event")
		if len(fs) != 1 || fs[0].Path != "a.md" {
			t.Errorf("%s: want one verified-event finding on a.md, got %+v", name, fs)
			continue
		}
		if !strings.Contains(fs[0].Message, "§5.2") || !strings.Contains(fs[0].Message, "ignore") {
			t.Errorf("%s: message should cite §5.2 and say the event is ignored: %q", name, fs[0].Message)
		}
		if strings.Contains(fs[0].Message, "map[") {
			t.Errorf("%s: message leaks Go map syntax: %q", name, fs[0].Message)
		}
	}
}

func TestLint_VerifiedEvent_CompleteEventsAndAbsenceSilent_Section5_2(t *testing.T) {
	// Negative controls: complete events (list and bare mapping), an absent
	// verified family (§5: never rejected), and §11.2 rule 1 null/empty list.
	files := map[string]string{
		"index.md": "# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [D](d.md)\n",
		"a.md":     fmDoc("verified:\n  - { by: human:x, at: \"2026-06-25T09:00:00Z\" }\n  - { by: process:y, at: 2026-06-26 }\n"),
		"b.md":     fmDoc("verified: { by: human:x, at: \"2026-06-25T09:00:00Z\" }\n"),
		"c.md":     fmDoc(""),
		"d.md":     fmDoc("verified: []\n"),
	}
	b := mkLintBundle(t, files)
	if fs := findingsFor(Lint(b, LintOptions{}), "verified-event"); len(fs) != 0 {
		t.Fatalf("complete/absent verified must be silent, got %+v", fs)
	}
}

func TestLint_VerifiedEvent_MalformedAtIsTimestampFormNotVerifiedEvent_Section5(t *testing.T) {
	// A present-but-malformed `at` is reported once, as timestamp-form; the
	// event is still ignored by the reader, but the finding is not duplicated.
	b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc("verified: { by: human:x, at: \"2026-06-25T09:00:00\" }\n"), "b.md": fmDoc("")})
	all := Lint(b, LintOptions{})
	if fs := findingsFor(all, "verified-event"); len(fs) != 0 {
		t.Errorf("malformed at is a timestamp-form finding only, got verified-event %+v", fs)
	}
	if fs := findingsFor(all, "timestamp-form"); len(fs) != 1 {
		t.Errorf("want one timestamp-form finding, got %+v", fs)
	}
}

// --- superseded-by (§5.4, §6.2) ---------------------------------------------

func TestLint_SupersededBy_FlagsDanglingTarget_Section5_4(t *testing.T) {
	for name, fm := range map[string]string{
		"bare relative":    "status: deprecated\nsuperseded_by: nope.md\n",
		"rooted":           "status: deprecated\nsuperseded_by: /nope.md\n",
		"case differs":     "status: deprecated\nsuperseded_by: B.md\n",            // §6: paths are case-sensitive
		"to a non-concept": "status: deprecated\nsuperseded_by: /index.md\n",       // §3.1: index.md is not a concept
		"not a URI":        "status: deprecated\nsuperseded_by: the new concept\n", // §6: whitespace is not a path
		"anchor only":      "status: deprecated\nsuperseded_by: \"#definition\"\n",
	} {
		b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc(fm), "b.md": fmDoc("")})
		fs := findingsFor(Lint(b, LintOptions{}), "superseded-by")
		if len(fs) != 1 || fs[0].Path != "a.md" {
			t.Errorf("%s: want one superseded-by finding on a.md, got %+v", name, fs)
			continue
		}
		if !strings.Contains(fs[0].Message, "§5.4") {
			t.Errorf("%s: message should cite §5.4: %q", name, fs[0].Message)
		}
	}
}

func TestLint_SupersededBy_ResolvingTargetSilent_Section5_4(t *testing.T) {
	// Negative controls: every in-bundle form that resolves (§6 rules 2 and 3,
	// the §6.2 v0.2 bundle-root fallback for an undeclared bundle, a fragment,
	// percent-encoding), plus an external URL (§6 rule 1: not checked against
	// the file system) and §11.2 rule 1 null/empty.
	files := map[string]string{
		"index.md":          "# Index\n\n- [A](a.md)\n",
		"a.md":              fmDoc("status: deprecated\nsuperseded_by: /sub/new.md\n"),
		"sub/index.md":      "# Sub\n\n- [New](new.md)\n- [Old](old.md)\n- [Old2](old2.md)\n- [Old3](old3.md)\n- [Old4](old4.md)\n- [Old5](old5.md)\n- [Old6](old6.md)\n- [My concept](my%20concept.md)\n",
		"sub/new.md":        fmDoc(""),
		"sub/my concept.md": fmDoc(""),
		"sub/old.md":        fmDoc("status: deprecated\nsuperseded_by: new.md\n"),                // file-relative
		"sub/old2.md":       fmDoc("status: deprecated\nsuperseded_by: sub/new.md\n"),            // §6.2 bundle-root fallback
		"sub/old3.md":       fmDoc("status: deprecated\nsuperseded_by: ./new.md#definition\n"),   // fragment
		"sub/old4.md":       fmDoc("status: deprecated\nsuperseded_by: https://example.com/x\n"), // external
		"sub/old5.md":       fmDoc("status: deprecated\nsuperseded_by:\n"),                       // null ⇒ absent
		"sub/old6.md":       fmDoc("status: deprecated\nsuperseded_by: my%20concept.md\n"),       // §6 percent-encoding
		// §6: a value that resolves outside the bundle root is external, not
		// broken; consumers MUST NOT read it, so lint cannot call it dangling.
		"sub/old7.md": fmDoc("status: deprecated\nsuperseded_by: ../../outside.md\n"),
	}
	b := mkLintBundle(t, files)
	if fs := findingsFor(Lint(b, LintOptions{}), "superseded-by"); len(fs) != 0 {
		t.Fatalf("resolving superseded_by must be silent, got %+v", fs)
	}
}

func TestLint_SupersededBy_DeclaredV03BundleHasNoRootFallback_Section6_2(t *testing.T) {
	// §6.2: the bundle-root fallback applies only to a bundle that declares 0.2
	// or earlier, or no version. A bundle that declares 0.3 resolves a bare
	// value from the file's directory only.
	files := map[string]string{
		".okf":       "okf_version: \"0.3\"\n",
		"index.md":   "# Index\n\n- [Old](sub/old.md)\n- [New](sub/new.md)\n",
		"sub/new.md": fmDoc(""),
		"sub/old.md": fmDoc("status: deprecated\nsuperseded_by: sub/new.md\n"),
	}
	b := mkLintBundle(t, files)
	fs := Lint(b, LintOptions{})
	// The value resolves file-relative to sub/sub/new.md, which is missing; the
	// basename names sub/new.md, so it is the stale-path defect broken-link
	// reports. Exactly one finding for the value, never two.
	bl, sb := findingsFor(fs, "broken-link"), findingsFor(fs, "superseded-by")
	if len(bl) != 1 || !strings.Contains(bl[0].Message, "superseded_by") || len(sb) != 0 {
		t.Fatalf("declared-0.3 bundle: bare root-relative superseded_by must not resolve and must be reported once (broken-link); broken-link=%+v superseded-by=%+v", bl, sb)
	}
	// Same declared-0.3 bundle, successor with no node anywhere: superseded-by
	// owns the finding.
	files["sub/old.md"] = fmDoc("status: deprecated\nsuperseded_by: sub/gone.md\n")
	b = mkLintBundle(t, files)
	if sb := findingsFor(Lint(b, LintOptions{}), "superseded-by"); len(sb) != 1 || !strings.Contains(sb[0].Message, "sub/sub/gone.md") {
		t.Fatalf("declared-0.3 bundle: bare value resolves file-relative (sub/sub/gone.md); got %+v", sb)
	}
}

func TestLint_SupersededBy_OneFindingPerDanglingValue_Section6_2(t *testing.T) {
	// A dangling superseded_by whose basename names a node elsewhere is the
	// stale-path defect broken-link reports (§6.2 path field, "did you mean").
	// superseded-by must not report the same value a second time. Control: a
	// successor with no node anywhere is reported once, by superseded-by.
	b := mkLintBundle(t, map[string]string{
		"index.md":      "# Index\n\n- [A](a.md)\n- [C](c.md)\n- [GM](metrics/gm.md)\n",
		"metrics/gm.md": fmDoc(""),
		"a.md":          fmDoc("status: deprecated\nsuperseded_by: gm.md\n"),
		"c.md":          fmDoc("status: deprecated\nsuperseded_by: never-written.md\n"),
	})
	fs := Lint(b, LintOptions{})
	bl, sb := findingsFor(fs, "broken-link"), findingsFor(fs, "superseded-by")
	if len(bl) != 1 || bl[0].Path != "a.md" {
		t.Fatalf("want one broken-link on a.md, got %+v", bl)
	}
	if len(sb) != 1 || sb[0].Path != "c.md" {
		t.Fatalf("want one superseded-by on c.md only (a.md is already broken-link), got %+v", sb)
	}
}

func TestLint_SupersededBy_FlagsNonDeprecatedConcept_Section5_4(t *testing.T) {
	// §5.4: superseded_by "has a meaning only with status: deprecated". On any
	// other status it is meaningless, a curation slip worth a lint note.
	for _, st := range []string{"", "status: stable\n", "status: draft\n"} {
		b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc(st + "superseded_by: b.md\n"), "b.md": fmDoc("")})
		fs := findingsFor(Lint(b, LintOptions{}), "superseded-by")
		if len(fs) != 1 || !strings.Contains(fs[0].Message, "deprecated") {
			t.Errorf("status %q: want one superseded-by finding naming deprecated, got %+v", st, fs)
		}
	}
}

func TestLint_SupersededBy_DeprecatedWithoutSuccessorSilent_Section5_4(t *testing.T) {
	// Negative control: superseded_by is optional. A deprecated concept with no
	// replacement is the documented "retire a concept" path.
	b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": fmDoc("status: deprecated\n"), "b.md": fmDoc("")})
	if fs := findingsFor(Lint(b, LintOptions{}), "superseded-by"); len(fs) != 0 {
		t.Fatalf("deprecated without superseded_by is valid, got %+v", fs)
	}
}

// --- status-lifecycle wording (§5.4, §11.2 rule 4) ---------------------------

func TestLint_Status_UnknownValueMessageStatesSection5_4Meaning(t *testing.T) {
	// v0.3 §5.4 DEFINES an unknown status: it is not stable and filters to
	// current knowledge exclude it. §11.2 rule 4: the consumer keeps the value
	// and MAY warn. The finding must state that meaning, not call the value a
	// defect or "not allowed".
	b := mkLintBundle(t, map[string]string{"index.md": lintIndex, "a.md": statusDoc("A", "frobnicated"), "b.md": fmDoc("")})
	fs := findingsFor(Lint(b, LintOptions{}), "status-lifecycle")
	if len(fs) != 1 {
		t.Fatalf("want one status-lifecycle finding, got %+v", fs)
	}
	msg := fs[0].Message
	for _, want := range []string{"frobnicated", "not stable", "§5.4", "current"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q should contain %q", msg, want)
		}
	}
	for _, bad := range []string{"defect", "invalid", "not allowed", "not in the §5.4 lifecycle enum"} {
		if strings.Contains(msg, bad) {
			t.Errorf("message %q must not claim %q: v0.3 permits the value", msg, bad)
		}
	}
}
