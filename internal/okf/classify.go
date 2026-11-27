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
	"strings"
)

// FileKind is what OKF v0.3 §3.2 ("Concepts, assets, and other files") says a
// file under the bundle root is. ClassifyPath and ClassifyMarkdown are the one
// classifier every bundle walker and path-taking writer goes through, so the
// loader, validate, lint, graph, index, search, and the node writers cannot
// disagree about what is a concept.
type FileKind int

const (
	// KindExcluded: a path with any segment starting with "." (§3.2 rule 4).
	// It is not part of the bundle: never read, never reported.
	KindExcluded FileKind = iota
	// KindReserved: index.md or log.md at any depth (§3.1). Never a concept,
	// whatever frontmatter it carries; checked against §8/§9.
	KindReserved
	// KindAsset: a file whose name does not end in ".md" (§3.2 rule 3). It MAY
	// be a link or path-field target; it is never a concept.
	KindAsset
	// KindMarkdown: a non-reserved ".md" whose concept status depends on its
	// frontmatter. ClassifyMarkdown resolves it to one of the three kinds below.
	KindMarkdown
	// KindConcept: frontmatter with a non-empty `type` (§3.2 rule 1).
	KindConcept
	// KindNonConcept: no frontmatter, or no non-empty `type` (§3.2 rule 2).
	// Consumers skip it and it does not affect conformance; validators SHOULD
	// warn about it.
	KindNonConcept
	// KindMalformed: a frontmatter block that does not parse as a YAML mapping.
	// That is the one way a `.md` makes a bundle non-conformant (§11.1 rule 1).
	KindMalformed
)

func (k FileKind) String() string {
	switch k {
	case KindExcluded:
		return "excluded"
	case KindReserved:
		return "reserved"
	case KindAsset:
		return "asset"
	case KindMarkdown:
		return "markdown"
	case KindConcept:
		return "concept"
	case KindNonConcept:
		return "non-concept"
	case KindMalformed:
		return "malformed"
	}
	return fmt.Sprintf("FileKind(%d)", int(k))
}

// ClassifyPath classifies a bundle-relative slash path by name alone (§3.1,
// §3.2 rules 3 and 4). A non-reserved ".md" comes back as KindMarkdown: whether
// it is a concept depends on its content (ClassifyMarkdown).
//
// Rule 4 is checked first, so a reserved name inside a dot directory
// (".hidden/index.md") is excluded, not reserved: it is not in the bundle at
// all. The path is relative to the bundle root, so the root's own name never
// counts as a dot-path.
func ClassifyPath(rel string) FileKind {
	if IsDotPath(rel) {
		return KindExcluded
	}
	if IsReservedPath(rel) {
		return KindReserved
	}
	if !strings.HasSuffix(rel, ".md") {
		return KindAsset
	}
	return KindMarkdown
}

// IsDotPath reports whether any segment of a bundle-relative slash path starts
// with "." (§3.2 rule 4: "Files and directories whose names start with `.` are
// not part of the bundle"). "." and ".." segments are path syntax, not names,
// so callers should pass a cleaned path.
func IsDotPath(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// MarkdownClass is the content half of the §3.2 classification for one
// non-reserved ".md" file, with the parse results the loader keeps.
type MarkdownClass struct {
	Kind        FileKind       // KindConcept, KindNonConcept, or KindMalformed
	Frontmatter map[string]any // nil when Kind == KindMalformed
	Body        string         // the body after the block; the whole file when malformed
	HasBlock    bool           // a `---` frontmatter block was physically present
	// Reason says why a KindNonConcept file is not a concept, in words a
	// curator can act on. Empty for the other kinds.
	Reason string
}

// ClassifyMarkdown applies §3.2 rules 1 and 2 to the content of a non-reserved
// ".md" file. A `type` that is null, an empty string, or only whitespace is the
// same as an absent one (§11.2 rule 1).
func ClassifyMarkdown(src []byte) MarkdownClass {
	fm, body, present, err := ParseFrontmatterDetailed(src)
	if err != nil {
		return MarkdownClass{Kind: KindMalformed, Body: string(src), HasBlock: true}
	}
	c := MarkdownClass{Frontmatter: fm, Body: body, HasBlock: present}
	switch {
	case !present:
		c.Kind, c.Reason = KindNonConcept, "no frontmatter"
	case !hasConceptType(fm):
		c.Kind, c.Reason = KindNonConcept, "frontmatter has no non-empty type"
	default:
		c.Kind = KindConcept
	}
	return c
}

// hasConceptType reports whether a frontmatter mapping carries a non-empty
// string `type` (§3.2 rule 1, §4.1). It agrees with Node.Type, which reads only
// a string value.
func hasConceptType(fm map[string]any) bool {
	t, ok := fm["type"].(string)
	return ok && strings.TrimSpace(t) != ""
}
