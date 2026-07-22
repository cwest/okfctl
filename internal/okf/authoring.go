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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// NewNode creates a conformant concept node at relPath (bundle-relative) with a
// required non-empty type (§7.2). It refuses an empty type and refuses to
// overwrite an existing file. Frontmatter is YAML-marshaled (never concatenated)
// so a type/title containing newlines or YAML metacharacters is safely quoted
// and cannot inject additional frontmatter keys. Returns the absolute path written.
func NewNode(root, relPath, typ, title string) (string, error) {
	if strings.TrimSpace(typ) == "" {
		return "", fmt.Errorf("type is required and must be non-empty (OKF §7)")
	}
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("node already exists: %s", relPath)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}

	// Build frontmatter as an ordered YAML mapping (type first, then optional
	// title), marshaled — NOT string-concatenated — so any value is safely quoted.
	// yaml.v3 has no MapSlice/MapItem (that is yaml.v2); an explicit MappingNode
	// preserves key order deterministically while still escaping every scalar.
	fm := &yaml.Node{Kind: yaml.MappingNode}
	appendKV := func(key, val string) {
		fm.Content = append(fm.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: val},
		)
	}
	appendKV("type", typ)
	if strings.TrimSpace(title) != "" {
		appendKV("title", title)
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	heading := title
	if strings.TrimSpace(heading) == "" {
		heading = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}
	// If the heading spans multiple lines (e.g. a multiline title), keep only the
	// first line for the H1 so the body stays well-formed.
	if i := strings.IndexByte(heading, '\n'); i >= 0 {
		heading = heading[:i]
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")
	sb.WriteString("# " + heading + "\n")
	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}
