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

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// evalFixtureBundle: a.md is fully transparent (graded + cites a sibling);
// u.md is uncited and ungraded (two findings).
func evalFixtureBundle(t *testing.T) string {
	return writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [U](u.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\nepistemic: verified\n---\n\n# A\n\nSee [u](u.md) for detail.\n",
		"u.md":     "---\ntype: Concept\ntitle: U\n---\n\n# U\n\nA bare unsupported assertion.\n",
	})
}

func TestEvalTransparencyCmd_ReportsFindingsExitsZeroByDefault(t *testing.T) {
	out, err := runOKF(t, "eval", "transparency", evalFixtureBundle(t))
	if err != nil {
		t.Fatalf("eval transparency should exit 0 by default even with findings; got err: %v", err)
	}
	if !contains(out, "uncited") || !contains(out, "u.md") {
		t.Fatalf("expected uncited finding for u.md, got:\n%s", out)
	}
	if !contains(out, "grade-missing") {
		t.Fatalf("expected grade-missing finding, got:\n%s", out)
	}
}

func TestEvalTransparencyCmd_StrictExitsNonZero(t *testing.T) {
	_, err := runOKF(t, "eval", "transparency", "--strict", evalFixtureBundle(t))
	if err == nil {
		t.Fatalf("eval transparency --strict should exit non-zero when there are findings")
	}
}

func TestEvalTransparencyCmd_JSONShape(t *testing.T) {
	out, err := runOKF(t, "eval", "transparency", "--json", evalFixtureBundle(t))
	if err != nil {
		t.Fatalf("eval transparency --json should exit 0 by default: %v", err)
	}
	var findings []okf.EvalFinding
	if err := json.Unmarshal([]byte(out), &findings); err != nil {
		t.Fatalf("--json must emit a JSON array, got err %v for:\n%s", err, out)
	}
	if len(findings) == 0 {
		t.Fatalf("expected findings in JSON, got none")
	}
}

func TestEvalTransparencyCmd_CleanBundleExitsZero(t *testing.T) {
	clean := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     "---\ntype: Concept\ntitle: A\nepistemic: verified\n---\n\n# A\n\nSee [b](b.md).\n",
		"b.md":     "---\ntype: Concept\ntitle: B\nepistemic: verified\n---\n\n# B\n\nSee [a](a.md).\n",
	})
	out, err := runOKF(t, "eval", "transparency", "--strict", clean)
	if err != nil {
		t.Fatalf("clean bundle --strict should exit 0, got err: %v\n%s", err, out)
	}
}

func TestEvalSampleCmd_JSONScaffold(t *testing.T) {
	out, err := runOKF(t, "eval", "sample", "--sample", "2", "--seed", "5", evalFixtureBundle(t))
	if err != nil {
		t.Fatalf("eval sample should exit 0: %v", err)
	}
	var scaffolds []okf.NodeEvalScaffold
	if err := json.Unmarshal([]byte(out), &scaffolds); err != nil {
		t.Fatalf("eval sample --format json must emit a JSON array, got err %v for:\n%s", err, out)
	}
	if len(scaffolds) != 2 {
		t.Fatalf("expected 2 scaffolds, got %d", len(scaffolds))
	}
}

func TestEvalSampleCmd_MarkdownWorksheet(t *testing.T) {
	out, err := runOKF(t, "eval", "sample", "--sample", "1", "--seed", "1", "--format", "md", evalFixtureBundle(t))
	if err != nil {
		t.Fatalf("eval sample --format md should exit 0: %v", err)
	}
	for _, want := range []string{"Accuracy", "Alignment", "Calibration"} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown worksheet missing %q dimension header, got:\n%s", want, out)
		}
	}
}

func TestEvalCmd_UnknownSubcommandShowsHelp(t *testing.T) {
	// Convention across okfctl parent commands (node, index): an unknown
	// subcommand prints the parent's help rather than erroring. eval matches it.
	out, _ := runOKF(t, "eval", "bogus", evalFixtureBundle(t))
	if !contains(out, "transparency") || !contains(out, "sample") {
		t.Fatalf("unknown eval subcommand should surface the parent help listing its verbs, got:\n%s", out)
	}
}

// changedNodePaths passes the user's --changed-since ref into git's argv BEFORE
// the "--" separator, where git parses a leading-dash token as an OPTION rather
// than a revision (e.g. --upload-pack=..., --output=...). The guard must reject
// such refs so a crafted --changed-since cannot become option injection.
func TestChangedNodePaths_RejectsOptionInjectionRef(t *testing.T) {
	dir := evalFixtureBundle(t)
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load fixture bundle: %v", err)
	}
	for _, ref := range []string{"--upload-pack=touch /tmp/x", "--output=/tmp/x", "-x"} {
		_, err := changedNodePaths(dir, ref, b)
		if err == nil {
			t.Fatalf("ref %q begins with %q and must be rejected before reaching git argv, got nil error", ref, "-")
		}
		// Assert the GUARD rejected it (not an incidental git failure): the
		// error must name the guard's own reason, proving the ref never reached
		// git's argv where it could be parsed as an option.
		if !strings.Contains(err.Error(), "must not begin with") {
			t.Fatalf("ref %q must be rejected by the option-injection guard, but the error looks like something else: %v", ref, err)
		}
	}
}
