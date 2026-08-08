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
//
// NewNode is the no-template path: it delegates to NewNodeFromTemplate with an
// empty template, so both paths share one containment/marshal/write mechanism.
func NewNode(root, relPath, typ, title string) (string, error) {
	return NewNodeFromTemplate(root, relPath, typ, title, Template{})
}

// NewNodeFromTemplate creates a node conformant to both the spec floor and a
// governing type template (PRD §9.3). Beyond the required type + title, it stubs
// the template's required fields (with a "TODO" placeholder so the node starts
// free of template drift) and recommended fields (empty), and lays down its
// body_sections as empty `## ` headings. An empty Template scaffolds nothing —
// that is the plain NewNode path. Existing-file, containment, and empty-type
// refusals apply. Returns the absolute path written.
func NewNodeFromTemplate(root, relPath, typ, title string, t Template) (string, error) {
	if strings.TrimSpace(typ) == "" {
		return "", fmt.Errorf("type is required and must be non-empty (OKF §7)")
	}
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absAbs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, absAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("node path escapes the bundle root: %s", relPath)
	}
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("node already exists: %s", relPath)
	}
	// Bundle directories hold shareable knowledge documents committed to git and
	// read by others; 0o755 is the intended, conventional mode for content dirs.
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { //nolint:gosec // G301: shareable bundle content dir
		return "", err
	}

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
	// Stamp created + modified at birth (both equal), so a node authored through
	// okfctl starts life with an accurate, computed timestamp rather than a
	// hand-maintained one. created is immutable from here on; modified advances
	// on every subsequent okfctl write. (Corpus form: RFC3339 UTC.)
	birth := nowUTC().Format(timestampLayout)
	appendKV("created", birth)
	appendKV("modified", birth)
	// Stub the template's fields. Required fields get a "TODO" placeholder so the
	// node starts life free of template drift (§9.3: "conformant to both the spec
	// floor and the team's convention") while still signalling the author what to
	// fill in; recommended fields are stubbed empty (they are advisory, never
	// drift). Skip keys already written (type, title).
	required := map[string]bool{}
	for _, key := range t.RequiredFields {
		required[key] = true
	}
	fields, sections := TemplateScaffold(t)
	written := map[string]bool{"type": true, "title": true, "created": true, "modified": true}
	for _, key := range fields {
		if written[key] {
			continue
		}
		val := ""
		if required[key] {
			val = "TODO"
		}
		appendKV(key, val)
		written[key] = true
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	heading := title
	if strings.TrimSpace(heading) == "" {
		heading = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}
	if i := strings.IndexByte(heading, '\n'); i >= 0 {
		heading = heading[:i]
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")
	sb.WriteString("# " + heading + "\n")
	for _, section := range sections {
		sb.WriteString("\n## " + section + "\n")
	}
	// A bundle node is a shareable knowledge document (committed to git, read by
	// others); 0o644 is the intended, conventional mode for content files.
	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil { //nolint:gosec // G306: shareable bundle content file
		return "", err
	}
	return abs, nil
}
