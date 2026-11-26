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

// Spec-conformance suite for the OKF v0.3 §5 semantics. It replays the five
// shapes that previously passed with no signal at all and pins where each one
// lands: lint (curation guidance), never validate. §11.1 defines bundle
// conformance as parseable frontmatter plus reserved files that agree with
// §8/§9, and §11.2 says a bundle with absent, empty, or unexpected-shape values
// "stays conformant", so a validate failure for any of these would reject a
// bundle v0.3 calls conformant (over-conformance).

func v03Section5ProbeBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# KB\n\n- [no offset](no-offset.md)\n- [stale](stale.md)\n- [verified](verified.md)\n- [superseded](superseded.md)\n- [status](status.md)\n- [clean](clean.md)\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "no-offset.md", "---\ntype: Concept\ngenerated: { by: human:x, at: \"2026-06-20T10:00:00\" }\n---\n\n# No offset\n")
	writeFile(t, dir, "stale.md", "---\ntype: Concept\nstale_after: not-a-date\n---\n\n# Stale\n")
	writeFile(t, dir, "verified.md", "---\ntype: Concept\nverified: { by: human:x }\n---\n\n# Verified\n")
	writeFile(t, dir, "superseded.md", "---\ntype: Concept\nstatus: deprecated\nsuperseded_by: nope.md\n---\n\n# Superseded\n")
	writeFile(t, dir, "status.md", "---\ntype: Concept\nstatus: frobnicated\n---\n\n# Status\n")
	// Negative control: a v0.3-shaped concept using every §5 family correctly.
	writeFile(t, dir, "clean.md", "---\ntype: Concept\n"+
		"generated: { by: reference_agent/x, at: \"2026-06-20T10:00:00Z\" }\n"+
		"verified:\n  - { by: human:x, at: \"2026-06-25T09:00:00Z\" }\n"+
		"stale_after: \"2099-01-01\"\n"+
		"sources:\n  - { resource: https://x.test, last_modified: 2026-05-30 }\n"+
		"usage_window: { from: \"2026-06-01T00:00:00Z\", to: \"2026-06-30T00:00:00Z\" }\n"+
		"status: stable\n---\n\n# Clean\n")
	return dir
}

func TestConformance_Section5ProbeShapesLintNotValidate_V03(t *testing.T) {
	b, err := Load(v03Section5ProbeBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	// §11.1/§11.2: every shape is conformant, so validate stays clean.
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("§11: these shapes must not fail validate (over-conformance); got %v", f)
	}
	want := map[string]string{
		"no-offset.md":  "timestamp-form",
		"stale.md":      "timestamp-form",
		"verified.md":   "verified-event",
		"superseded.md": "superseded-by",
		"status.md":     "status-lifecycle",
	}
	got := map[string][]string{}
	for _, f := range Lint(b, LintOptions{}) {
		switch f.Check {
		case "timestamp-form", "verified-event", "superseded-by", "status-lifecycle":
			got[f.Path] = append(got[f.Path], f.Check)
		}
	}
	for path, check := range want {
		if len(got[path]) != 1 || got[path][0] != check {
			t.Errorf("%s: §5 lint checks = %v, want exactly [%s]", path, got[path], check)
		}
	}
	if len(got["clean.md"]) != 0 {
		t.Errorf("clean.md (negative control) got %v, want none", got["clean.md"])
	}

	// The reader honors the meanings, not just the findings.
	far := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	if !b.Nodes["stale.md"].IsStale(far) {
		t.Error("§5.5: unparseable stale_after must never be fresh")
	}
	if got := b.Nodes["verified.md"].TrustTier(); got != TrustUnverified {
		t.Errorf("§5.2: an at-less event is ignored; TrustTier = %q, want unverified", got)
	}
	if b.Nodes["status.md"].IsStable() {
		t.Error("§5.4: an unknown status is not stable")
	}
	if got := b.Nodes["clean.md"].TrustTier(); got != TrustHumanReviewed {
		t.Errorf("clean.md TrustTier = %q, want human-reviewed (current human event)", got)
	}
}

// TestConformance_V01BundleUnchangedBySection5Checks_V03 is the v0.1 control
// AGENTS.md requires for any frontmatter/provenance change: a legacy bundle
// (timestamp, body # Citations, no §5 families) lints and validates clean.
func TestConformance_V01BundleUnchangedBySection5Checks_V03(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.1\n")
	writeFile(t, dir, "index.md", "# KB\n\n- [income](income.md)\n")
	writeFile(t, dir, "log.md", logHeader+logPlaceholder)
	writeFile(t, dir, "income.md", "---\ntype: Metric\ntitle: Income\ntimestamp: '2026-05-28T22:53:05+00:00'\ncreated: 2026-05-28\n---\n\n# Income\n\n# Citations\n\n[1] Ledger\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("v0.1 bundle must validate clean; got %v", f)
	}
	if f := Lint(b, LintOptions{}); len(f) != 0 {
		t.Fatalf("v0.1 bundle must lint clean; got %v", f)
	}
}
