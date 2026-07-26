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
// This is the single, order-preserving writer for a timestamp refresh — it edits
// the frontmatter block only, so it cannot drop the body the way rewriting a
// parsed sub-region over the whole file would.
func TouchModifiedFile(abs string, at time.Time) error {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read %s: %w", abs, err)
	}
	stamp := at.UTC().Format(timestampLayout)

	sp, ok := splitFrontmatter(raw)
	if !ok {
		// No frontmatter block: prepend a minimal one carrying just modified.
		// (Validate will still flag a missing type — that is the floor's job,
		// not this refresh's.)
		var out bytes.Buffer
		out.WriteString("---\nmodified: " + stamp + "\n---\n")
		out.Write(raw)
		return os.WriteFile(abs, out.Bytes(), 0o644)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(sp.yamlBlock, &doc); err != nil {
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
	out.Write(sp.body)
	return os.WriteFile(abs, out.Bytes(), 0o644)
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
