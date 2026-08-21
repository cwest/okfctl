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
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// TouchModifiedFile refreshes the frontmatter `modified` field of the node at
// abs to `at` (RFC3339 UTC), writing the file back in place. It is order- and
// body-preserving: the frontmatter block is round-tripped through a yaml.Node
// so existing keys keep their order and the Markdown body is preserved verbatim.
// `created` is never rewritten (only modified is touched); a node without a
// `modified` key gains one appended to the end of its frontmatter, and one
// without frontmatter at all gains a minimal block. It never fabricates created.
//
// Two write paths, tried in this order:
//
//  1. Byte-SPLICE (spliceScalar). When `modified` is a top-level, plain,
//     single-line scalar and the new stamp is plain-safe, only the value span of
//     that one field is overwritten — every other byte of the frontmatter block
//     (blank-line grouping, list indentation, alignment padding, comments,
//     quoting style, key order) is preserved exactly. `node refresh` exists to
//     remediate git drift, so it must not manufacture git noise doing it; the
//     splice keeps a one-field refresh to a one-line diff, which is the whole
//     point of this path.
//
//  2. Whole-block RE-ENCODE (fallback). When the splice declines — the key is
//     absent, its value is non-scalar or block/quoted style, or the new value
//     would need quoting — the block is re-emitted through the yaml encoder.
//     This is the correctness backstop: it always produces valid, order-
//     preserving YAML, at the cost of normalising layout the splice would have
//     kept. The fallback is deliberately conservative-by-omission — the splice
//     handles only the narrow, provably-safe case and everything else lands here.
//
// Both paths write the frontmatter block only, so neither can drop the body the
// way rewriting a parsed sub-region over the whole file would.
func TouchModifiedFile(abs string, at time.Time) error {
	// abs is the user's node file being refreshed; reading it is intended.
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle node
	if err != nil {
		return fmt.Errorf("read %s: %w", abs, err)
	}
	stamp := at.UTC().Format(timestampLayout)

	yamlBlock, rawAfter, ok := splitFrontmatterRaw(raw)
	if !ok {
		// No frontmatter block: prepend a minimal one carrying just modified.
		// (Validate will still flag a missing type — that is the floor's job,
		// not this refresh's.)
		var out bytes.Buffer
		out.WriteString("---\nmodified: " + stamp + "\n---\n")
		out.Write(raw)
		// A bundle node is a shareable knowledge document; 0o644 is intended.
		return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: shareable bundle content file
	}

	// Path 1: try the byte-preserving splice of just the modified value span.
	if spliced, ok := spliceScalar(yamlBlock, "modified", stamp); ok {
		var out bytes.Buffer
		out.WriteString("---\n")
		out.Write(spliced)
		out.WriteString("---\n")
		out.Write(rawAfter)
		// A bundle node is a shareable knowledge document; 0o644 is intended.
		return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: shareable bundle content file
	}

	// Path 2: fallback — whole-block re-encode through the yaml encoder.
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return fmt.Errorf("parse frontmatter of %s: %w", abs, err)
	}
	root := frontmatterMapping(&doc)
	if root == nil {
		return fmt.Errorf("frontmatter of %s is not a mapping", abs)
	}
	setScalar(root, "modified", stamp)

	var fmBuf bytes.Buffer
	enc := yaml.NewEncoder(&fmBuf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("encode frontmatter of %s: %w", abs, err)
	}
	_ = enc.Close()

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(fmBuf.Bytes())
	out.WriteString("---\n")
	// rawAfter is the body region verbatim (including any blank separator line),
	// so the only bytes that change are inside the frontmatter block.
	out.Write(rawAfter)
	// A bundle node is a shareable knowledge document; 0o644 is intended.
	return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: shareable bundle content file
}

// spliceScalar overwrites ONLY the value span of a top-level scalar key in a
// frontmatter block, returning the edited block and ok=true when the edit is
// provably safe, or (nil, false) when the caller must fall back to a full
// re-encode. It deliberately handles only the narrow case the git-drift refresh
// needs — a single top-level scalar whose value stays plain — so that everything
// outside that value (blank lines, indentation, alignment padding, comments,
// quoting, key order, the body) is preserved byte-for-byte by construction.
//
// It DECLINES (ok=false) for every case outside that envelope, each of which is a
// corruption vector if spliced blindly:
//   - the block does not parse, or is not a mapping;
//   - the key is absent (nothing to splice);
//   - the value is not a scalar (a sequence/mapping value would be truncated);
//   - the value is not plain style (a quoted or block scalar has delimiters the
//     value-span replacement would leave mismatched);
//   - the new value would itself need quoting (splicing a plain span with a value
//     that requires quotes yields invalid or reinterpreted YAML).
//
// The Line/Column on the value yaml.Node come from the same parse, so no second
// parse is needed; they locate the start of the value token, and the token runs
// to the end of its source line minus any trailing comment.
func spliceScalar(block []byte, key, newVal string) (out []byte, ok bool) {
	var doc yaml.Node
	if err := yaml.Unmarshal(block, &doc); err != nil {
		return nil, false
	}
	root := frontmatterMapping(&doc)
	if root == nil {
		return nil, false
	}
	var val *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			val = root.Content[i+1]
			break
		}
	}
	if val == nil {
		return nil, false // key absent
	}
	if val.Kind != yaml.ScalarNode {
		return nil, false // non-scalar value (sequence/mapping)
	}
	// An empty / null scalar value (`modified:` with nothing after the colon)
	// has no value token to overwrite — the Column points at or past the line
	// end, so a splice there would fabricate a `key:value` with no separator.
	// Let the encoder path add the value properly.
	if val.Value == "" {
		return nil, false
	}
	// Style 0 is plain; any other style (single/double quoted, literal `|`,
	// folded `>`) carries delimiters a value-span splice would leave dangling.
	if val.Style != 0 {
		return nil, false
	}
	// The new value must itself emit as a plain scalar; otherwise splicing it
	// into a plain span would produce a value that needs quoting.
	if !plainSafeScalar(newVal) {
		return nil, false
	}

	// Locate the value token by its 1-based Line/Column over the source block.
	start, found := lineColOffset(block, val.Line, val.Column)
	if !found {
		return nil, false
	}
	// The token ends at the end of its line; trim a trailing ` #...` comment and
	// any run of spaces/tabs before it so the comment (and its spacing) survive.
	lineEnd := bytes.IndexByte(block[start:], '\n')
	if lineEnd < 0 {
		lineEnd = len(block) - start
	}
	valueLine := block[start : start+lineEnd]
	tokenLen := len(valueLine)
	if ci := commentIndex(valueLine); ci >= 0 {
		tokenLen = ci
		for tokenLen > 0 && (valueLine[tokenLen-1] == ' ' || valueLine[tokenLen-1] == '	') {
			tokenLen--
		}
	} else {
		// Strip a trailing \r (CRLF source) from the value token.
		for tokenLen > 0 && valueLine[tokenLen-1] == '\r' {
			tokenLen--
		}
	}
	// A zero-length value token means the Line/Column did not land on real value
	// bytes (e.g. a folded/implicit-null shape the guards above did not catch).
	// Decline rather than splice an empty span.
	if tokenLen == 0 {
		return nil, false
	}
	// The extracted single-line token must equal the decoded scalar value. For a
	// single-line plain scalar the two are identical (plain scalars carry no
	// escaping). If they differ, the value is a MULTI-LINE folded plain scalar
	// (e.g. `k: 00\n 0` decodes to "00 0"): the token is only its first line, so
	// splicing it would leave the continuation dangling. Decline to the encoder.
	if string(valueLine[:tokenLen]) != val.Value {
		return nil, false
	}

	var buf bytes.Buffer
	buf.Grow(len(block) - tokenLen + len(newVal))
	buf.Write(block[:start])
	buf.WriteString(newVal)
	buf.Write(block[start+tokenLen:])
	return buf.Bytes(), true
}

// lineColOffset converts a 1-based (line, col) position into a 0-based byte
// offset into src, or ok=false if the position is out of range.
func lineColOffset(src []byte, line, col int) (int, bool) {
	if line < 1 || col < 1 {
		return 0, false
	}
	off := 0
	for l := 1; l < line; l++ {
		nl := bytes.IndexByte(src[off:], '\n')
		if nl < 0 {
			return 0, false
		}
		off += nl + 1
	}
	off += col - 1
	if off > len(src) {
		return 0, false
	}
	return off, true
}

// commentIndex returns the index of a YAML comment start (` #` — a `#` preceded
// by whitespace, or at the very start of the region) within a single value line,
// or -1 if there is none. A `#` embedded in an unquoted scalar with no leading
// space is a literal character, not a comment, per YAML's comment rule.
func commentIndex(line []byte) int {
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if i == 0 || line[i-1] == ' ' || line[i-1] == '	' {
			return i
		}
	}
	return -1
}

// plainSafeScalar reports whether s would be emitted by the yaml encoder as a
// plain (unquoted) scalar. It is the guard that keeps spliceScalar from writing a
// plain span with a value the encoder would have quoted — which would either be
// invalid YAML or reinterpret the value's type. It errs conservative: any doubt
// returns false and the caller falls back to the encoder path.
func plainSafeScalar(s string) bool {
	if s == "" {
		return false
	}
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: s}
	b, err := yaml.Marshal(n)
	if err != nil {
		return false
	}
	// yaml.Marshal of a scalar node emits the value plus a trailing newline. If
	// the round-tripped form is not byte-identical to the input (plus newline),
	// the encoder added quoting, a tag, or block folding — not plain-safe.
	return bytes.Equal(b, append([]byte(s), '\n'))
}

// frontmatterMapping unwraps a decoded yaml document to its top-level mapping
// node, or nil when the document is empty or not a mapping.
func frontmatterMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	return doc
}

// setScalar updates the scalar value of key in a mapping node in place,
// preserving key order; if the key is absent it is appended, keeping the
// original keys ahead of it.
func setScalar(mapping *yaml.Node, key, val string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			v := mapping.Content[i+1]
			v.Kind = yaml.ScalarNode
			v.Tag = ""
			v.Style = 0
			v.Value = val
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: val},
	)
}
