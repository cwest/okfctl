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

// §6 rule 2 / §6.2: in a v0.3 bundle a bare value resolves from its file's
// directory only. When sub/a.md writes a bare root-relative value, the
// broken-link suggestion must be a form that resolves from sub/a.md (the
// `/`-rooted path), never the unresolved value echoed back.
func TestConformance_Lint_BrokenLink_Section6_HintIsResolvableForm(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":         v03Index + "\n* [A](sub/a.md)\n* [C](Other/Cased.md)\n* [T](target.md)\n",
		"sub/a.md":         rdoc("A", "status: deprecated\nsuperseded_by: target.md\n", "See [C](Other/Cased.md)."),
		"Other/Cased.md":   rdoc("Cased", "", "[A](../sub/a.md)"),
		"target.md":        rdoc("Target", "", "[A](sub/a.md)"),
		"sub/elsewhere.md": rdoc("Else", "", "[A](a.md)"),
	})
	bl := findingsFor(Lint(b, LintOptions{}), "broken-link")
	if len(bl) != 2 {
		t.Fatalf("want 2 broken-link findings (body link + superseded_by), got %d: %+v", len(bl), bl)
	}
	var body, field string
	for _, f := range bl {
		if strings.Contains(f.Message, "superseded_by") {
			field = f.Message
		} else {
			body = f.Message
		}
	}
	if !strings.Contains(body, "did you mean /Other/Cased.md?") {
		t.Errorf("§6 body link: want suggestion /Other/Cased.md, got %q", body)
	}
	if !strings.Contains(field, "did you mean /target.md?") {
		t.Errorf("§6.2 path field: want suggestion /target.md, got %q", field)
	}
	for _, echo := range []string{"did you mean Other/Cased.md?", "did you mean target.md?"} {
		if strings.Contains(body+field, echo) {
			t.Errorf("suggestion must never echo the unresolved value: %q", echo)
		}
	}

	// Every suggestion must actually resolve from the linking file.
	for _, s := range []string{"/Other/Cased.md", "/target.md"} {
		if r := b.ResolveLink("sub/a.md", s); r.Kind != RefBundle || !r.IsConcept {
			t.Errorf("suggested %q does not resolve from sub/a.md: %+v", s, r)
		}
	}
}

// Negative controls: a genuinely moved file still gets its new location
// suggested (in the resolvable form), several basename matches list all
// candidates, and a link with no basename match gets no finding at all.
func TestConformance_Lint_BrokenLink_Section6_HintControls(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":     v03Index + "\n* [A](a.md)\n* [X](new/x.md)\n* [P](p/dup.md)\n* [Q](q/dup.md)\n",
		"a.md":         rdoc("A", "", "[X](old/x.md) [D](dup.md) [G](never-written.md)"),
		"new/x.md":     rdoc("X", "", "[A](../a.md)"),
		"p/dup.md":     rdoc("P", "", "[A](../a.md)"),
		"q/dup.md":     rdoc("Q", "", "[A](../a.md)"),
		"other/rel.md": rdoc("R", "", "[A](../a.md)"),
	})
	bl := findingsFor(Lint(b, LintOptions{}), "broken-link")
	if len(bl) != 2 {
		t.Fatalf("want 2 broken-link findings (moved + ambiguous), got %d: %+v", len(bl), bl)
	}
	msgs := bl[0].Message + "\n" + bl[1].Message
	if !strings.Contains(msgs, "links to old/x.md which resolves to no node; did you mean /new/x.md?") {
		t.Errorf("moved file must suggest its new location: %q", msgs)
	}
	if !strings.Contains(msgs, "did you mean one of: /p/dup.md, /q/dup.md?") {
		t.Errorf("ambiguous basename must list every candidate: %q", msgs)
	}
	if strings.Contains(msgs, "never-written") {
		t.Errorf("a link with no basename match must not be a broken-link: %q", msgs)
	}
}
