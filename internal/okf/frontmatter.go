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

	gmmeta "github.com/yuin/goldmark-meta"
	"gopkg.in/yaml.v3"
)

// ParseFrontmatter splits a source file into its YAML frontmatter map and the
// Markdown body. Missing frontmatter yields an empty (non-nil) map and no error;
// malformed YAML frontmatter is an error.
func ParseFrontmatter(src []byte) (map[string]any, string, error) {
	fm := map[string]any{}
	rest, ok := splitFrontmatter(src)
	if !ok {
		return fm, string(src), nil
	}
	if err := yaml.Unmarshal(rest.yamlBlock, &fm); err != nil {
		return nil, "", err
	}
	// yaml.Unmarshal of a null document (a bare `!` tag, a literal `null`, or a
	// block that reduces to null) into a map leaves fm nil and returns NO error.
	// A nil Frontmatter is the sentinel Load/validate treat as "unparseable"
	// (validate.go), so returning it here on a successful parse would both
	// masquerade as a parse failure and drop the body. The success contract is a
	// usable, non-nil map — a node with zero frontmatter fields is legal at parse
	// time (§7's required `type` is a validate concern, not a parser crash).
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, string(rest.body), nil
}

type split struct {
	yamlBlock []byte
	body      []byte
}

// splitFrontmatter detects a leading `---\n ... \n---\n` block. The closing
// delimiter must be a line that is exactly `---` (optionally with a trailing
// \r); a body line merely starting with `---` (e.g. a horizontal rule) is not
// mistaken for the fence.
func splitFrontmatter(src []byte) (split, bool) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim+"\n")) && !bytes.HasPrefix(src, []byte(delim+"\r\n")) {
		return split{}, false
	}
	nl := bytes.IndexByte(src, '\n')
	rest := src[nl+1:]

	lines := bytes.SplitAfter(rest, []byte("\n"))
	var yamlBuf bytes.Buffer
	consumed := 0
	found := false
	for _, line := range lines {
		consumed += len(line)
		// Compare the line without its trailing newline / carriage return.
		trimmed := bytes.TrimSuffix(line, []byte("\n"))
		trimmed = bytes.TrimSuffix(trimmed, []byte("\r"))
		if bytes.Equal(trimmed, []byte(delim)) {
			found = true
			break
		}
		// Strip a trailing \r so CRLF sources parse cleanly.
		yamlBuf.Write(bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r")))
		yamlBuf.WriteByte('\n')
	}
	if !found {
		return split{}, false
	}
	after := rest[consumed:]
	after = bytes.TrimPrefix(after, []byte("\r\n"))
	after = bytes.TrimPrefix(after, []byte("\n"))
	return split{yamlBlock: yamlBuf.Bytes(), body: after}, true
}

// splitFrontmatterRaw is the byte-preserving variant of splitFrontmatter. It
// returns the frontmatter YAML block and the body region EXACTLY as it appears
// after the closing `---` fence's newline — including any leading blank
// separator line. splitFrontmatter deliberately strips that separator for
// parsing (a parsed body should not start with a blank line); this variant
// keeps it so an in-place rewriter can reproduce the file byte-for-byte outside
// the field it changed. rawAfter is everything after the closing `---\n`.
func splitFrontmatterRaw(src []byte) (yamlBlock, rawAfter []byte, ok bool) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim+"\n")) && !bytes.HasPrefix(src, []byte(delim+"\r\n")) {
		return nil, nil, false
	}
	nl := bytes.IndexByte(src, '\n')
	rest := src[nl+1:]

	lines := bytes.SplitAfter(rest, []byte("\n"))
	var yamlBuf bytes.Buffer
	consumed := 0
	found := false
	for _, line := range lines {
		consumed += len(line)
		trimmed := bytes.TrimSuffix(line, []byte("\n"))
		trimmed = bytes.TrimSuffix(trimmed, []byte("\r"))
		if bytes.Equal(trimmed, []byte(delim)) {
			found = true
			break
		}
		yamlBuf.Write(bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r")))
		yamlBuf.WriteByte('\n')
	}
	if !found {
		return nil, nil, false
	}
	// rest[consumed:] is everything after the closing fence line (including its
	// own trailing newline already consumed), i.e. the verbatim body region.
	return yamlBuf.Bytes(), rest[consumed:], true
}

// ensure goldmark-meta stays pinned as a dependency for downstream rendering.
var _ = gmmeta.Meta
