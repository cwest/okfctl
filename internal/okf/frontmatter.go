// Copyright 2026 Casey West
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

	"github.com/yuin/goldmark"
	gmmeta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
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

// splitFrontmatter detects a leading `---\n ... \n---\n` block.
func splitFrontmatter(src []byte) (split, bool) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim+"\n")) && !bytes.HasPrefix(src, []byte(delim+"\r\n")) {
		return split{}, false
	}
	nl := bytes.IndexByte(src, '\n')
	rest := src[nl+1:]
	end := bytes.Index(rest, []byte("\n"+delim))
	if end < 0 {
		return split{}, false
	}
	yamlBlock := rest[:end]
	after := rest[end+1+len(delim):]
	after = bytes.TrimPrefix(after, []byte("\n"))
	after = bytes.TrimPrefix(after, []byte("\r\n"))
	return split{yamlBlock: yamlBlock, body: after}, true
}

// ensure goldmark-meta is a real dependency for downstream body rendering.
var _ = func() goldmark.Markdown {
	return goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()), goldmark.WithExtensions(gmmeta.Meta))
}
