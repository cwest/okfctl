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

// ensure goldmark-meta stays pinned as a dependency for downstream rendering.
var _ = gmmeta.Meta
