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
	"net/url"
	"path"
	"strconv"
	"strings"
)

// This file is the single OKF v0.3 §6 resolver. Every consumer that turns a
// link target or a path field value into a bundle location (edge building,
// orphan/inbound counting, dangling and broken-link detection, move and
// promote rewrites, eval citations, graph export) goes through ResolveLink or
// ResolvePathField, so no two commands can disagree about where a value points.
//
// §6 resolution rules, in order:
//  1. A value with a URI scheme (for example `https:`) is an absolute URL.
//  2. A value that starts with `/` resolves from the bundle root.
//  3. Any other value resolves from the directory of the file that contains it.
//
// A value that resolves outside the bundle root is external and is never read
// from the local file system (§6, §11.2). Paths are case-sensitive and are
// RFC 3986 URI references, so they are percent-decoded before lookup.
//
// §6.2 / §12.1 compatibility: a path field without a leading `/` that does not
// resolve from its file MAY resolve from the bundle root when the bundle
// declares okf_version 0.2 or earlier, or declares no version. okfctl applies
// the same fallback to body links in those bundles (see ResolveLink). The
// fallback keys on the DECLARED version (Bundle.DeclaredOkfVersion), never on
// the build's SpecVersion, so an undeclared bundle keeps the fallback after
// the tool's default version moves.

// RefKind classifies what a link target or path field value refers to.
type RefKind int

const (
	// RefEmpty is a null, empty, or blank value; the same as absent (§11.2 rule 1).
	RefEmpty RefKind = iota
	// RefAnchor is an in-page `#fragment` with no path.
	RefAnchor
	// RefURL is an absolute URL: the value has a URI scheme (§6 rule 1).
	RefURL
	// RefDescriptor is a value that cannot be a URI reference (it contains
	// whitespace), such as a §5.1 population or scope descriptor. It is not a
	// path and is never resolved.
	RefDescriptor
	// RefExternal is a path that resolves outside the bundle root (§6).
	RefExternal
	// RefBundle is a path inside the bundle. Ref.Exists reports whether the
	// bundle contains a file or directory at Ref.Path.
	RefBundle
)

// RefForm records which §6 rule placed an in-bundle path.
type RefForm int

const (
	// FormNone applies to every Ref that is not RefBundle.
	FormNone RefForm = iota
	// FormFileRel is §6 rule 3: resolved from the containing file's directory.
	FormFileRel
	// FormRootAbs is §6 rule 2: a leading `/`, resolved from the bundle root.
	FormRootAbs
	// FormRootFallback is the §6.2/§12.1 compatibility fallback: a bare path
	// field that did not resolve from its file, resolved from the bundle root.
	FormRootFallback
)

// Ref is the result of resolving one link target or path field value.
type Ref struct {
	Kind RefKind
	// Path is the bundle-relative, slash-separated, percent-decoded location
	// for RefBundle ("" is the bundle root directory). Empty otherwise.
	Path string
	Form RefForm
	// Exists reports whether the bundle holds a file or directory at Path.
	// The check is against the walked names, so it is case-sensitive (§6) on
	// every file system.
	Exists bool
	// IsConcept reports whether Path is a concept node of the bundle.
	IsConcept bool
}

// ResolveLink resolves a Markdown link target found in the file fromFile (a
// bundle-relative path) per §6 and §6.1. Any `#fragment` and `?query` are
// dropped before resolution.
//
// §12.1 names the bundle-root fallback for path fields. okfctl extends the same
// fallback to body links in a bundle that declares okf_version 0.2 or earlier,
// or no version: earlier okfctl releases resolved bare links from the bundle
// root first, and `node mv` / `node promote` wrote that form, so a legacy
// bundle's root-relative bare links keep resolving. The fallback only ever
// resolves more (a consumer MUST accept broken links anyway, §6.1), and the
// file-relative target always wins when it exists. A bundle declaring 0.3 or
// later gets strict §6 resolution.
func (b *Bundle) ResolveLink(fromFile, value string) Ref {
	return b.resolve(fromFile, value)
}

// ResolvePathField resolves a §6.2 path field value found in the file
// fromFile. It applies the §6.2/§12.1 bundle-root fallback when the bundle
// declares okf_version 0.2 or earlier, or declares none.
func (b *Bundle) ResolvePathField(fromFile, value string) Ref {
	return b.resolve(fromFile, value)
}

func (b *Bundle) resolve(fromFile, value string) Ref {
	v := strings.TrimSpace(value)
	if v == "" {
		return Ref{Kind: RefEmpty}
	}
	// §6 Identifiers: a URI reference cannot contain whitespace (a space MUST
	// be percent-encoded), so a value with whitespace is not a path. For
	// sources[].resource this is the §5.1 scope descriptor case.
	if strings.ContainsAny(v, " \t\r\n") {
		return Ref{Kind: RefDescriptor}
	}
	if hasURIScheme(v) {
		return Ref{Kind: RefURL}
	}
	if strings.HasPrefix(v, "#") {
		return Ref{Kind: RefAnchor}
	}
	p := v
	if i := strings.IndexAny(p, "#?"); i >= 0 {
		p = p[:i]
	}
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}

	if strings.HasPrefix(p, "/") {
		rel, escaped := joinWithin("", p)
		if escaped {
			return Ref{Kind: RefExternal}
		}
		return b.bundleRef(rel, FormRootAbs)
	}

	rel, escaped := joinWithin(path.Dir(fromFile), p)
	if escaped {
		return Ref{Kind: RefExternal}
	}
	r := b.bundleRef(rel, FormFileRel)
	if r.Exists || !b.rootFallback() {
		return r
	}
	if rootRel, esc := joinWithin("", p); !esc {
		if fb := b.bundleRef(rootRel, FormRootFallback); fb.Exists {
			return fb
		}
	}
	return r
}

func (b *Bundle) bundleRef(rel string, form RefForm) Ref {
	_, concept := b.Nodes[rel]
	return Ref{
		Kind:      RefBundle,
		Path:      rel,
		Form:      form,
		Exists:    b.exists(rel),
		IsConcept: concept,
	}
}

// exists reports whether the walked bundle holds a file or directory at rel.
func (b *Bundle) exists(rel string) bool {
	if rel == "" {
		return true // the bundle root
	}
	if _, ok := b.Nodes[rel]; ok {
		return true
	}
	// A §3.2 rule 2 non-concept .md is a real file: a link or path field that
	// names it resolves (§6.1, §6.2), it just does not refer to a concept.
	if _, ok := b.NonConcepts[rel]; ok {
		return true
	}
	if _, ok := b.Reserved[rel]; ok {
		return true
	}
	return b.files[rel] || b.dirs[rel]
}

// rootFallback reports whether the §12.1 path-field fallback applies.
func (b *Bundle) rootFallback() bool {
	return legacyVersion(b.DeclaredOkfVersion)
}

// legacyVersion reports whether a declared okf_version is "0.2 or earlier, or
// no version" (§12.1). An unparseable or later version is not legacy; it gets
// no fallback, and it is still consumed best-effort (§12), never refused.
func legacyVersion(declared string) bool {
	v := strings.TrimPrefix(strings.TrimSpace(declared), "v")
	if v == "" {
		return true
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major == 0 && minor <= 2
}

// joinWithin applies the slash path rel to the bundle-relative directory base,
// processing "." and ".." segments. escaped is true when a ".." segment would
// climb above the bundle root, in which case the value is external (§6). The
// returned path has no leading or trailing slash; "" is the bundle root.
func joinWithin(base, rel string) (string, bool) {
	var stack []string
	if base != "" && base != "." {
		stack = strings.Split(base, "/")
	}
	for _, seg := range strings.Split(rel, "/") {
		switch seg {
		case "", ".":
		case "..":
			if len(stack) == 0 {
				return "", true
			}
			stack = stack[:len(stack)-1]
		default:
			stack = append(stack, seg)
		}
	}
	return strings.Join(stack, "/"), false
}

// hasURIScheme reports whether v starts with an RFC 3986 scheme:
// ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":".
func hasURIScheme(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		case c == ':' && i > 0:
			return true
		default:
			return false
		}
	}
	return false
}

// PathField is one §6.2 path field occurrence in a node's frontmatter.
type PathField struct {
	// Field names where the value sits, e.g. "resource",
	// "sources[1].resource", "executor.resource", "superseded_by".
	Field string
	Value string
}

// PathFields returns every §6.2 path field in the node's frontmatter, in a
// fixed order: resource, sources[].resource, computation, executor.resource,
// attester.resource, superseded_by. Only string values are returned; a null or
// empty value is the same as absent (§11.2 rule 1). A single `sources` mapping
// is read as a one-item list (§11.2 rule 2). Scope descriptors are returned
// too; ResolvePathField classifies them.
func (n *Node) PathFields() []PathField {
	if n == nil || n.Frontmatter == nil {
		return nil
	}
	var out []PathField
	add := func(field string, v any) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, PathField{Field: field, Value: s})
		}
	}
	fm := n.Frontmatter
	add("resource", fm["resource"])
	var sources []any
	switch s := fm["sources"].(type) {
	case []any:
		sources = s
	case map[string]any:
		sources = []any{s}
	}
	for i, item := range sources {
		if m, ok := item.(map[string]any); ok {
			add(fmt.Sprintf("sources[%d].resource", i), m["resource"])
		}
	}
	add("computation", fm["computation"])
	for _, k := range []string{"executor", "attester"} {
		if m, ok := fm[k].(map[string]any); ok {
			add(k+".resource", m["resource"])
		}
	}
	add("superseded_by", fm["superseded_by"])
	return out
}

// pathFieldConcepts returns the concept nodes that src's path fields resolve
// to: the §6.4 path-field edges of the bundle graph, in PathFields order.
func (b *Bundle) pathFieldConcepts(src string, n *Node) []string {
	var out []string
	for _, f := range n.PathFields() {
		if r := b.ResolvePathField(src, f.Value); r.Kind == RefBundle && r.IsConcept {
			out = append(out, r.Path)
		}
	}
	return out
}

// Where a bundle's okf_version declaration came from (Bundle.OkfVersionSource).
const (
	// VersionFromIndex: the bundle-root index.md frontmatter (§8.1, §12).
	VersionFromIndex = "index.md"
	// VersionFromSidecar: okfctl's legacy .okf sidecar, read only when the
	// root index.md declares nothing.
	VersionFromSidecar = ".okf"
	// VersionUndeclared: neither declares a version.
	VersionUndeclared = ""
)

// readDeclaredVersion reads the bundle's declared okf_version. The bundle-root
// index.md frontmatter is the spec-defined declaration (§8.1, §12); the .okf
// sidecar is a legacy fallback used only when the index declares nothing. It
// returns ("", VersionUndeclared) when neither declares one.
func (b *Bundle) readDeclaredVersion() (string, string) {
	if v := indexDeclaredVersion(b); v != "" {
		return v, VersionFromIndex
	}
	if v := sidecarDeclaredVersion(b.Root); v != "" {
		return v, VersionFromSidecar
	}
	return "", VersionUndeclared
}

func indexDeclaredVersion(b *Bundle) string {
	idx, ok := b.Reserved["index.md"]
	if !ok || idx.Frontmatter == nil {
		return ""
	}
	v, ok := idx.Frontmatter["okf_version"]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(scalarString(v))
}

func sidecarDeclaredVersion(root string) string {
	return strings.Trim(strings.TrimSpace(readOkfSidecar(root)["okf_version"]), `"'`)
}

// VersionConflict reports a bundle whose root index.md and .okf sidecar both
// declare okf_version with different values. The index wins (§8.1, §12); the
// returned message is an advisory warning, never a conformance failure.
func VersionConflict(b *Bundle) (string, bool) {
	if b.OkfVersionSource != VersionFromIndex {
		return "", false
	}
	sc := sidecarDeclaredVersion(b.Root)
	if sc == "" || sc == b.DeclaredOkfVersion {
		return "", false
	}
	return fmt.Sprintf("okf_version %q in the bundle-root index.md disagrees with %q in the legacy .okf sidecar; "+
		"the index.md declaration wins (§8.1, §12)", b.DeclaredOkfVersion, sc), true
}
