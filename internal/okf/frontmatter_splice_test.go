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
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// OKF v0.3 §4: "A producer that modifies an existing file SHOULD NOT
// re-serialize values that it did not change. This keeps diffs small."
// OKF v0.3 §4.1: "A producer that modifies an existing file MUST keep the keys
// that it does not know."
//
// These tests pin every node writer to a splice: the changed entry is the only
// part of the frontmatter block whose bytes move.

// handFormatted is a node with formatting an encoder has opinions about:
// section comments, alignment padding, blank-line grouping, four-space list
// indentation, a quoted value, an unknown key, an x- extension key, and a
// trailing line comment.
const handFormatted = "---\n" +
	"# Core identity\n" +
	"type:    Reference\n" +
	"title:   Tannin\n" +
	"\n" +
	"# Provenance\n" +
	"created:  2026-01-04T00:00:00Z\n" +
	"modified: 2026-01-04T00:00:00Z\n" +
	"\n" +
	"tags:\n" +
	"    - chemistry\n" +
	"    - wine\n" +
	"\n" +
	"sources:\n" +
	"    - resource: https://example.com/tannin\n" +
	"      title:    Tannin overview\n" +
	"\n" +
	"okf_note:  'unknown key, single-quoted'   # keep me\n" +
	"x-acme-owner: { team: data,  oncall: pager }\n" +
	"---\n" +
	"\n" +
	"# Tannin\n" +
	"\n" +
	"A polyphenol.\n"

func writeSpliceNode(t *testing.T, content string) string {
	t.Helper()
	abs := filepath.Join(t.TempDir(), "concepts", "tannin.md")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

// changedLines returns the lines of a and b that differ, assuming the edit
// replaced, inserted, or deleted one contiguous run of lines (the shape every
// splice produces). It strips the common prefix and suffix.
func changedLines(a, b string) (removed, added []string) {
	al := strings.SplitAfter(a, "\n")
	bl := strings.SplitAfter(b, "\n")
	p := 0
	for p < len(al) && p < len(bl) && al[p] == bl[p] {
		p++
	}
	s := 0
	for s < len(al)-p && s < len(bl)-p && al[len(al)-1-s] == bl[len(bl)-1-s] {
		s++
	}
	return al[p : len(al)-s], bl[p : len(bl)-s]
}

// The #146 reproduction: a one-field refresh is a one-line diff.
func TestConformance_TouchModifiedIsOneLineDiff_V03Section4(t *testing.T) {
	abs := writeSpliceNode(t, handFormatted)
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("TouchModifiedFile: %v", err)
	}
	got := readAll(t, abs)
	want := strings.Replace(handFormatted,
		"modified: 2026-01-04T00:00:00Z\n", "modified: 2026-08-20T00:00:00Z\n", 1)
	if got != want {
		rm, add := changedLines(handFormatted, got)
		t.Fatalf("§4: refresh re-serialized unchanged values\n-%q\n+%q", rm, add)
	}
}

// A quoted value keeps its quotes, its alignment padding, and its trailing
// line comment; only the value text between the quotes changes.
func TestConformance_TouchModifiedKeepsQuotingAndLineComment_V03Section4(t *testing.T) {
	orig := "---\ntype: Concept\nmodified:   \"2026-01-04T00:00:00Z\"   # set by hand\nx-foo: bar\n---\nbody\n"
	abs := writeSpliceNode(t, orig)
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	want := "---\ntype: Concept\nmodified:   \"2026-08-20T09:30:00Z\"   # set by hand\nx-foo: bar\n---\nbody\n"
	if got := readAll(t, abs); got != want {
		t.Fatalf("§4: quoted value not spliced in place\nGOT  %q\nWANT %q", got, want)
	}
}

// A node with no `modified` gains exactly one appended line; nothing above it
// moves (§4 for the untouched keys, §4.1 for the unknown ones).
func TestConformance_TouchModifiedInsertsNewKeyCleanly_V03Section4(t *testing.T) {
	orig := strings.Replace(handFormatted, "modified: 2026-01-04T00:00:00Z\n", "", 1)
	abs := writeSpliceNode(t, orig)
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got := readAll(t, abs)
	want := strings.Replace(orig, "---\n\n# Tannin", "modified: 2026-08-20T00:00:00Z\n---\n\n# Tannin", 1)
	if got != want {
		rm, add := changedLines(orig, got)
		t.Fatalf("§4: adding modified rewrote other lines\n-%q\n+%q", rm, add)
	}
}

// A CRLF file keeps CRLF on every line, including the spliced one.
func TestConformance_TouchModifiedPreservesCRLF_V03Section4(t *testing.T) {
	orig := "---\r\ntype:  Concept\r\nmodified: 2026-01-04T00:00:00Z\r\n---\r\n\r\n# N\r\n"
	abs := writeSpliceNode(t, orig)
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	want := "---\r\ntype:  Concept\r\nmodified: 2026-08-20T00:00:00Z\r\n---\r\n\r\n# N\r\n"
	if got := readAll(t, abs); got != want {
		t.Fatalf("§4: CRLF block not spliced in place\nGOT  %q\nWANT %q", got, want)
	}
}

// Negative control: a refresh to the SAME value writes the file back
// byte-identical. A splicer that "normalizes" on a no-op is re-serializing.
func TestConformance_TouchModifiedSameValueIsByteIdentical_V03Section4(t *testing.T) {
	abs := writeSpliceNode(t, handFormatted)
	if err := TouchModifiedFile(abs, time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, abs); got != handFormatted {
		rm, add := changedLines(handFormatted, got)
		t.Fatalf("§4: a no-op refresh changed bytes\n-%q\n+%q", rm, add)
	}
}

// Fallback control: a block whose top-level mapping is indented (legal YAML,
// a shape the splicer does not model) still gets the right value via the
// re-encoding backstop, and the result parses with every key kept (§4.1).
func TestConformance_TouchModifiedFallbackStaysCorrect_V03Section4_1(t *testing.T) {
	orig := "---\n  type: Concept\n  x-acme-owner: data\n  modified: 2026-01-04T00:00:00Z\n---\n\n# N\n"
	abs := writeSpliceNode(t, orig)
	if err := TouchModifiedFile(abs, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got := readAll(t, abs)
	fm, body, err := ParseFrontmatter([]byte(got))
	if err != nil {
		t.Fatalf("fallback output does not parse: %v\n%s", err, got)
	}
	if fm["type"] != "Concept" || fm["x-acme-owner"] != "data" {
		t.Fatalf("§4.1: fallback dropped a key: %v", fm)
	}
	if ts, ok := fm["modified"].(time.Time); !ok || !ts.Equal(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("fallback did not set modified: %#v", fm["modified"])
	}
	if !strings.HasSuffix(got, "---\n\n# N\n") || !strings.Contains(body, "# N") {
		t.Fatalf("fallback did not keep the body verbatim:\n%q", got)
	}
}

// The migrator's per-node edit (timestamp -> generated) touches only the
// removed and added entries; the rest of the hand-formatted block survives.
func TestConformance_MigrateTimestampRenameSplices_V03Section4(t *testing.T) {
	orig := "---\n" +
		"type:      Metric\n" +
		"timestamp: 2026-01-04T00:00:00Z\n" +
		"\n" +
		"tags:\n" +
		"    - revenue\n" +
		"x-acme-owner:  data   # owning team\n" +
		"---\n\n# A\n"
	nm := NodeMigration{Generated: &GeneratedEdit{By: "human:casey", At: "2026-01-04T00:00:00Z"}}
	out, err := renderMigratedNode([]byte(orig), nm)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n" +
		"type:      Metric\n" +
		"\n" +
		"tags:\n" +
		"    - revenue\n" +
		"x-acme-owner:  data   # owning team\n" +
		"generated: {by: \"human:casey\", at: \"2026-01-04T00:00:00Z\"}\n" +
		"---\n\n# A\n"
	if string(out) != want {
		t.Fatalf("§4: migrate re-serialized unchanged values\nGOT  %q\nWANT %q", out, want)
	}
}

// The migrator's sources fold appends to an existing block sequence at the
// indentation it already uses; the existing items (and their extension keys,
// §4.1) keep their bytes.
func TestConformance_MigrateSourcesAppendSplices_V03Section4(t *testing.T) {
	orig := "---\n" +
		"type: Metric\n" +
		"sources:\n" +
		"    - resource:  https://a.example/x\n" +
		"      x-acme-seen: true\n" +
		"\n" +
		"x-acme-owner: data\n" +
		"---\n\n# A\n"
	nm := NodeMigration{Sources: []SourceEdit{{Resource: "https://b.example/y"}}}
	out, err := renderMigratedNode([]byte(orig), nm)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n" +
		"type: Metric\n" +
		"sources:\n" +
		"    - resource:  https://a.example/x\n" +
		"      x-acme-seen: true\n" +
		"    - resource: \"https://b.example/y\"\n" +
		"\n" +
		"x-acme-owner: data\n" +
		"---\n\n# A\n"
	if string(out) != want {
		t.Fatalf("§4: sources fold re-serialized unchanged values\nGOT  %q\nWANT %q", out, want)
	}
}

// The migrator's okf_version bump on the bundle-root index splices the one
// value, double-quoting the new number-like version (§4) and keeping the key
// padding and trailing comment.
func TestConformance_MigrateRootMarkerBumpSplices_V03Section4(t *testing.T) {
	dir := t.TempDir()
	orig := "---\n# bundle marker\nokf_version:  '0.1'   # do not hand-edit\n---\n\n# KB\n"
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bumpRootIndexMarker(dir, "0.2"); err != nil {
		t.Fatal(err)
	}
	want := "---\n# bundle marker\nokf_version:  \"0.2\"   # do not hand-edit\n---\n\n# KB\n"
	if got := readAll(t, filepath.Join(dir, "index.md")); got != want {
		t.Fatalf("§4: marker bump re-serialized the block\nGOT  %q\nWANT %q", got, want)
	}
}

// FuzzEditFrontmatter drives the splicing editor with arbitrary frontmatter.
// The oracle is the pre-splice writer (re-encode the whole mutated block):
// whichever path runs, the result must decode to exactly what the old writer
// produced, so the splice changes HOW an edit reaches disk and never WHAT it
// means. §4.1 (unknown keys kept) therefore holds wherever it held before. The
// body must be byte-identical (§4: only the changed value moves).
func FuzzEditFrontmatter(f *testing.F) {
	seeds := []string{
		handFormatted,
		"---\ntype: Concept\nmodified: 2026-01-04T00:00:00Z\n---\nbody\n",
		"---\ntype: Concept\nmodified:   \"2026-01-04\"   # c\n---\n",
		"---\r\ntype: Concept\r\nmodified: x\r\n---\r\nbody\r\n",
		"---\n  type: Concept\n  modified: x\n---\n",
		"---\ntype: Concept\nmodified: |\n  multi\n  line\n---\n",
		"---\ntype: Concept\nmodified: &a x\nother: *a\n---\n",
		"---\ntype: Concept\nmodified: !!str 2026\n---\n",
		"---\nmodified:\n  - a\n---\n",
		"---\nplain: a\n  b\nmodified: 'it''s'\n---\n",
		"---\n# only a comment\n---\n",
		"---\ntype: é\nmodified: é # é\n---\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s), "2026-08-20T00:00:00Z")
	}
	f.Fuzz(func(t *testing.T, raw []byte, val string) {
		before, _, err := ParseFrontmatter(raw)
		if err != nil {
			return
		}
		out, err := editFrontmatter(raw, func(m *yaml.Node) { setScalar(m, "modified", val) })
		if err != nil {
			return // not a mapping / no block: refused, never corrupted
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(mustBlock(t, raw), &doc); err != nil {
			t.Fatalf("oracle parse: %v", err)
		}
		m := frontmatterMapping(&doc)
		setScalar(m, "modified", val)
		oracleRaw, err := reencodeFrontmatter(raw, m)
		if err != nil {
			return // the old writer could not express this edit either
		}
		oracle, _, err := ParseFrontmatter(oracleRaw)
		if err != nil {
			// The re-encode writer itself emits unparseable YAML for some values
			// (a yaml.v3 encoder quirk, e.g. a value of "\t"). The splicer must
			// not do worse: it either produced the same bytes via the fallback,
			// or a verified splice that parses.
			if _, _, perr := ParseFrontmatter(out); perr != nil && !bytes.Equal(out, oracleRaw) {
				t.Fatalf("splice output unparseable and differs from the writer's\nIN  %q\nOUT %q", raw, out)
			}
			return
		}
		after, _, err := ParseFrontmatter(out)
		if err != nil {
			t.Fatalf("edited frontmatter no longer parses: %v\nIN  %q\nOUT %q", err, raw, out)
		}
		// The edited key may legitimately differ in one way: the splice keeps
		// the author's quoting, so a quoted value stays a string where the
		// re-encode writer dropped the quotes (§4; and §4 SHOULD-quote for
		// values like okf_version). Everything else must match the oracle.
		if s, isStr := after["modified"].(string); isStr && s == val {
			after["modified"] = oracle["modified"]
		}
		// Faithful also means "the original document with only modified
		// changed": the splice keeps untouched bytes, so it can be MORE faithful
		// than the re-encode writer on YAML shapes the encoder normalizes
		// (yaml.v3 decodes `&0: ` with the key dropped but re-encodes it as a real
		// `&0 '':` key).
		unchanged := map[string]any{}
		for k, v := range before {
			unchanged[k] = v
		}
		unchanged["modified"] = oracle["modified"]
		if !reflect.DeepEqual(after, oracle) && !reflect.DeepEqual(after, unchanged) {
			t.Fatalf("splice diverges from the re-encode writer\nIN     %q\nOUT    %q\nORACLE %q\ngot  %#v\nwant %#v", raw, out, oracleRaw, after, oracle)
		}
		_, _, rawTail, _ := splitFrontmatterVerbatim(raw)
		if _, rawBody, ok := splitFrontmatterRaw(raw); ok {
			if _, outBody, _ := splitFrontmatterRaw(out); !bytes.Equal(rawBody, outBody) {
				t.Fatalf("body changed\nIN  %q\nOUT %q (tail %q)", raw, out, rawTail)
			}
		}
	})
}

// mustBlock returns the frontmatter block of raw.
func mustBlock(t *testing.T, raw []byte) []byte {
	t.Helper()
	b, _, ok := splitFrontmatterRaw(raw)
	if !ok {
		t.Fatal("no frontmatter block")
	}
	return b
}
