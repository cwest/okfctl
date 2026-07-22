# okfctl Walking Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the `init → author → validate` loop: scaffold a conformant OKF bundle, author nodes with a required `type`, and validate the spec floor.

**Architecture:** Layered per PRD §11. A stdlib-first core model (`internal/okf`) loads a bundle dir into an in-memory typed graph; thin cobra commands (`cmd/`) are adapters over it. `internal/okf` never imports cobra.

**Tech Stack:** Go 1.26; `spf13/cobra@v1.10.2`; `yuin/goldmark@v1.8.4` + `goldmark-meta@v1.1.0`; `gopkg.in/yaml.v3@v3.0.1`; `dominikbraun/graph@v0.23.0` (health-bar-confirmed at adoption; fall back to internal adjacency if it fails vet/test).

---

## Task 1: Go module + entrypoint + CI foundation

**Files:**
- Create: `go.mod`, `main.go`, `cmd/root.go`
- Create: `.github/workflows/ci.yml`
- Test: `cmd/root_test.go`

- [ ] **Step 1: Write the failing test** — `cmd/root_test.go`

```go
package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand_HasName(t *testing.T) {
	root := NewRootCmd()
	if root.Use != "okfctl" {
		t.Fatalf("root Use = %q, want %q", root.Use, "okfctl")
	}
}

func TestRootCommand_HelpRunsCleanly(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("--help produced no output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestRootCommand -v`
Expected: FAIL — package/`NewRootCmd` undefined (module not initialized yet).

- [ ] **Step 3: Initialize the module and write minimal code**

```bash
go mod init github.com/cwest/okfctl
go get github.com/spf13/cobra@v1.10.2
```

`cmd/root.go`:

```go
// Package cmd implements the okfctl command tree.
package cmd

import "github.com/spf13/cobra"

// NewRootCmd builds the okfctl root command with its subcommand tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "okfctl",
		Short:         "Manage Open Knowledge Format (OKF) bundles",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return root
}

// Execute runs the root command; main() calls this.
func Execute() error {
	return NewRootCmd().Execute()
}
```

`main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/cwest/okfctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "okfctl:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestRootCommand -v`
Expected: PASS (both tests).

- [ ] **Step 5: Add CI** — `.github/workflows/ci.yml`

```yaml
name: ci
on:
  push: { branches: [main] }
  pull_request:
jobs:
  build-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - name: gofmt
        run: test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
      - name: vet
        run: go vet ./...
      - name: build
        run: CGO_ENABLED=0 go build ./...
      - name: test
        run: go test ./... -race
```

- [ ] **Step 6: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add go.mod go.sum main.go cmd/root.go cmd/root_test.go .github/workflows/ci.yml
git commit -S -m "feat(cmd): scaffold cobra root command and CI"
```

---

## Task 2: Node model + frontmatter parsing

**Files:**
- Create: `internal/okf/node.go`, `internal/okf/frontmatter.go`
- Test: `internal/okf/frontmatter_test.go`

- [ ] **Step 1: Write the failing test** — `internal/okf/frontmatter_test.go`

```go
package okf

import "testing"

func TestParseFrontmatter_ExtractsTypeAndBody(t *testing.T) {
	src := []byte("---\ntype: Reference\ntitle: Widgets\n---\n\n# Body\n\nText here.\n")
	fm, body, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("ParseFrontmatter error: %v", err)
	}
	if fm["type"] != "Reference" {
		t.Errorf("type = %v, want Reference", fm["type"])
	}
	if fm["title"] != "Widgets" {
		t.Errorf("title = %v, want Widgets", fm["title"])
	}
	if want := "# Body"; !contains(body, want) {
		t.Errorf("body missing %q; got %q", want, body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	fm, _, err := ParseFrontmatter([]byte("# Just markdown\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
}

func TestParseFrontmatter_Malformed(t *testing.T) {
	_, _, err := ParseFrontmatter([]byte("---\ntype: [unterminated\n---\n"))
	if err == nil {
		t.Fatal("expected error on malformed YAML frontmatter, got nil")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestParseFrontmatter -v`
Expected: FAIL — `ParseFrontmatter` undefined.

- [ ] **Step 3: Write minimal implementation**

```bash
go get github.com/yuin/goldmark@v1.8.4 github.com/yuin/goldmark-meta@v1.1.0 gopkg.in/yaml.v3@v3.0.1
```

`internal/okf/node.go`:

```go
// Package okf is the core in-memory model for an OKF bundle.
// It must not import cobra or any CLI package.
package okf

// Node is a single OKF concept: its bundle-relative path is its identity,
// its frontmatter carries typed metadata (type is required, §7), and body
// holds the Markdown after the frontmatter block.
type Node struct {
	Path        string         // bundle-relative, e.g. "wine/tannin.md"
	Frontmatter map[string]any // parsed YAML frontmatter
	Body        string         // markdown after the frontmatter
}

// Type returns the node's type value ("" if absent or not a string).
func (n *Node) Type() string {
	if v, ok := n.Frontmatter["type"].(string); ok {
		return v
	}
	return ""
}
```

`internal/okf/frontmatter.go`:

```go
package okf

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
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

// splitFrontmatter detects a leading `---\n ... \n---\n` block.
func splitFrontmatter(src []byte) (split, bool) {
	const delim = "---"
	if !bytes.HasPrefix(src, []byte(delim+"\n")) && !bytes.HasPrefix(src, []byte(delim+"\r\n")) {
		return split{}, false
	}
	// Find the closing delimiter on its own line.
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
```

> Note: the hand-rolled `splitFrontmatter` keeps frontmatter+body separation
> under our control (goldmark-meta parses meta but does not hand back the body
> verbatim). goldmark-meta remains wired for later body-rendering needs (`serve`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/okf/ -run TestParseFrontmatter -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add internal/okf/node.go internal/okf/frontmatter.go internal/okf/frontmatter_test.go go.mod go.sum
git commit -S -m "feat(okf): node model and frontmatter parsing"
```

---

## Task 3: Bundle loader → in-memory typed graph

**Files:**
- Create: `internal/okf/bundle.go`
- Create: `testdata/good-bundle/` (fixtures)
- Test: `internal/okf/bundle_test.go`

- [ ] **Step 1: Create the good fixture bundle**

```bash
mkdir -p testdata/good-bundle/wine
printf -- '---\ntype: Index\n---\n\n# Knowledge Base\n\n- [Tannin](wine/tannin.md)\n' > testdata/good-bundle/index.md
printf -- '# Log\n\n- 2026-07-22 init\n' > testdata/good-bundle/log.md
printf -- '---\ntype: Reference\ntitle: Tannin\n---\n\n# Tannin\n\nSee [acidity](wine/acidity.md).\n' > testdata/good-bundle/wine/tannin.md
printf -- '---\ntype: Reference\ntitle: Acidity\n---\n\n# Acidity\n' > testdata/good-bundle/wine/acidity.md
```

- [ ] **Step 2: Write the failing test** — `internal/okf/bundle_test.go`

```go
package okf

import (
	"path/filepath"
	"testing"
)

func TestLoad_CountsConceptNodes(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "testdata", "good-bundle"))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// index.md and log.md are reserved, not concept nodes.
	if got := len(b.Nodes); got != 2 {
		t.Fatalf("concept nodes = %d, want 2 (%v)", got, keys(b.Nodes))
	}
	if _, ok := b.Nodes["wine/tannin.md"]; !ok {
		t.Errorf("missing node wine/tannin.md; have %v", keys(b.Nodes))
	}
}

func TestLoad_ExtractsEdgesFromLinks(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "testdata", "good-bundle"))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	edges := b.OutboundLinks("wine/tannin.md")
	found := false
	for _, e := range edges {
		if e == "wine/acidity.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edge tannin -> acidity, got %v", edges)
	}
}

func keys(m map[string]*Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestLoad -v`
Expected: FAIL — `Load` / `Bundle` undefined.

- [ ] **Step 4: Write minimal implementation** — `internal/okf/bundle.go`

```go
package okf

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReservedFiles are the bundle-level files that are not concept nodes.
var ReservedFiles = map[string]bool{"index.md": true, "log.md": true}

// Bundle is a loaded OKF bundle: concept nodes keyed by bundle-relative path,
// plus the reserved files, plus the derived link graph.
type Bundle struct {
	Root     string
	Nodes    map[string]*Node // concept nodes only (excludes reserved)
	Reserved map[string]*Node // index.md, log.md
	edges    map[string][]string
}

var mdLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Load walks root, parses every .md file, and builds the in-memory graph.
func Load(root string) (*Bundle, error) {
	b := &Bundle{
		Root:     root,
		Nodes:    map[string]*Node{},
		Reserved: map[string]*Node{},
		edges:    map[string][]string{},
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, body, ferr := ParseFrontmatter(src)
		if ferr != nil {
			// Preserve the node with nil frontmatter so validate can report it.
			n := &Node{Path: rel, Frontmatter: nil, Body: string(src)}
			b.place(rel, n)
			return nil
		}
		n := &Node{Path: rel, Frontmatter: fm, Body: body}
		b.place(rel, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	b.buildEdges()
	return b, nil
}

func (b *Bundle) place(rel string, n *Node) {
	if ReservedFiles[rel] {
		b.Reserved[rel] = n
		return
	}
	b.Nodes[rel] = n
}

func (b *Bundle) buildEdges() {
	for path, n := range b.Nodes {
		dir := filepath.Dir(path)
		for _, m := range mdLinkRe.FindAllStringSubmatch(n.Body, -1) {
			target := m[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
				continue
			}
			// Resolve relative to the bundle root: links are written relative
			// to root in these fixtures; normalize and keep only in-bundle targets.
			norm := filepath.ToSlash(filepath.Clean(target))
			if _, ok := b.Nodes[norm]; ok {
				b.edges[path] = append(b.edges[path], norm)
				continue
			}
			// Fall back to dir-relative resolution.
			rel := filepath.ToSlash(filepath.Clean(filepath.Join(dir, target)))
			if _, ok := b.Nodes[rel]; ok {
				b.edges[path] = append(b.edges[path], rel)
			}
		}
	}
}

// OutboundLinks returns the in-bundle nodes that path links to.
func (b *Bundle) OutboundLinks(path string) []string { return b.edges[path] }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/okf/ -run TestLoad -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add internal/okf/bundle.go internal/okf/bundle_test.go testdata/good-bundle
git commit -S -m "feat(okf): bundle loader builds in-memory typed graph"
```

---

## Task 4: Spec-floor validation (managed `type`, §7)

**Files:**
- Create: `internal/okf/validate.go`
- Create: `testdata/no-type/`, `testdata/empty-type/`, `testdata/unknown-type/`
- Test: `internal/okf/validate_test.go`

- [ ] **Step 1: Create bad + unknown-type fixtures**

```bash
mkdir -p testdata/no-type testdata/empty-type testdata/unknown-type
printf -- '---\ntype: Index\n---\n# KB\n' > testdata/no-type/index.md
printf -- '# Log\n' > testdata/no-type/log.md
printf -- '---\ntitle: Orphan\n---\n# No type here\n' > testdata/no-type/orphan.md

printf -- '---\ntype: Index\n---\n# KB\n' > testdata/empty-type/index.md
printf -- '# Log\n' > testdata/empty-type/log.md
printf -- '---\ntype: ""\ntitle: Empty\n---\n# Empty type\n' > testdata/empty-type/empty.md

printf -- '---\ntype: Index\n---\n# KB\n' > testdata/unknown-type/index.md
printf -- '# Log\n' > testdata/unknown-type/log.md
printf -- '---\ntype: GreebleFrobnicator9000\n---\n# Weird but valid\n' > testdata/unknown-type/weird.md
```

- [ ] **Step 2: Write the failing test** — `internal/okf/validate_test.go`

```go
package okf

import (
	"path/filepath"
	"testing"
)

func loadOrFail(t *testing.T, name string) *Bundle {
	t.Helper()
	b, err := Load(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return b
}

func TestValidate_GoodBundlePasses(t *testing.T) {
	if f := Validate(loadOrFail(t, "good-bundle")); len(f) != 0 {
		t.Errorf("good bundle should pass, got findings: %v", f)
	}
}

func TestValidate_MissingTypeFails(t *testing.T) {
	f := Validate(loadOrFail(t, "no-type"))
	if !hasFindingFor(f, "orphan.md") {
		t.Errorf("expected a missing-type finding for orphan.md, got %v", f)
	}
}

func TestValidate_EmptyTypeFails(t *testing.T) {
	f := Validate(loadOrFail(t, "empty-type"))
	if !hasFindingFor(f, "empty.md") {
		t.Errorf("expected an empty-type finding for empty.md, got %v", f)
	}
}

func TestValidate_UnknownTypePasses(t *testing.T) {
	// Presence, not taxonomy (PRD §7.4): an unfamiliar type value is valid.
	if f := Validate(loadOrFail(t, "unknown-type")); len(f) != 0 {
		t.Errorf("unknown type value must PASS, got findings: %v", f)
	}
}

func hasFindingFor(fs []Finding, path string) bool {
	for _, f := range fs {
		if f.Path == path {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestValidate -v`
Expected: FAIL — `Validate` / `Finding` undefined.

- [ ] **Step 4: Write minimal implementation** — `internal/okf/validate.go`

```go
package okf

import "strings"

// Finding is a single spec-floor violation. Path is bundle-relative.
type Finding struct {
	Path    string
	Message string
}

// Validate enforces the OKF spec floor only (PRD §6.2, §7.1):
//   - frontmatter must be parseable (nil frontmatter == parse failure);
//   - every concept node has a non-empty `type` (§7 rule 2);
// It never enforces a taxonomy of type VALUES (§7.4): unknown types pass.
// It returns findings; an empty slice means the bundle passes the floor.
func Validate(b *Bundle) []Finding {
	var out []Finding
	for path, n := range b.Nodes {
		if n.Frontmatter == nil {
			out = append(out, Finding{Path: path, Message: "unparseable frontmatter"})
			continue
		}
		if strings.TrimSpace(n.Type()) == "" {
			out = append(out, Finding{Path: path, Message: "missing or empty required field: type"})
		}
	}
	return out
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/okf/ -run TestValidate -v`
Expected: PASS (all four).

- [ ] **Step 6: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add internal/okf/validate.go internal/okf/validate_test.go testdata/no-type testdata/empty-type testdata/unknown-type
git commit -S -m "feat(okf): spec-floor validation with managed type presence"
```

---

## Task 5: `validate` command (wire model → CLI, exit codes)

**Files:**
- Create: `cmd/validate.go`
- Modify: `cmd/root.go` (register subcommand)
- Test: `cmd/validate_test.go`

- [ ] **Step 1: Write the failing test** — `cmd/validate_test.go`

```go
package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func runOKF(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestValidateCmd_GoodBundleExitsZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "good-bundle")
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate good bundle returned error (nonzero exit): %v", err)
	}
}

func TestValidateCmd_MissingTypeExitsNonZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "no-type")
	out, err := runOKF(t, "validate", dir)
	if err == nil {
		t.Fatalf("validate no-type must return error (nonzero exit); out=%q", out)
	}
}
```

> Note: `cmd` tests reference `../testdata/...`; the fixtures created under the
> repo-root `testdata/` in Tasks 3–4 are reachable from `cmd/` via `..`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestValidateCmd -v`
Expected: FAIL — no `validate` subcommand registered.

- [ ] **Step 3: Write minimal implementation** — `cmd/validate.go`

```go
package cmd

import (
	"fmt"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [bundle-dir]",
		Short: "Check a bundle for OKF spec-floor conformance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			findings := okf.Validate(b)
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK: bundle conforms to the OKF spec floor")
				return nil
			}
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %s\n", f.Path, f.Message)
			}
			return fmt.Errorf("%d conformance finding(s)", len(findings))
		},
	}
}
```

Modify `cmd/root.go` — register inside `NewRootCmd()` before `return root`:

```go
	root.AddCommand(newValidateCmd())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestValidateCmd -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add cmd/validate.go cmd/root.go cmd/validate_test.go
git commit -S -m "feat(cmd): validate command with spec-floor exit codes"
```

---

## Task 6: `bundle init` scaffold

**Files:**
- Create: `internal/okf/reserved.go`, `cmd/bundle.go`
- Modify: `cmd/root.go`
- Test: `internal/okf/reserved_test.go`, `cmd/bundle_test.go`

- [ ] **Step 1: Write the failing test** — `internal/okf/reserved_test.go`

```go
package okf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffold_ProducesValidatableBundle(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	for _, f := range []string{"index.md", "log.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load scaffolded bundle: %v", err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("scaffolded bundle must validate clean, got %v", f)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestScaffold -v`
Expected: FAIL — `Scaffold` undefined.

- [ ] **Step 3: Write minimal implementation** — `internal/okf/reserved.go`

```go
package okf

import (
	"os"
	"path/filepath"
)

// SpecVersion is the OKF spec version this build targets.
const SpecVersion = "0.1"

// Scaffold writes a minimal conformant bundle into dir: a reserved index.md and
// log.md and an .okf spec pin. The result passes Validate with zero findings
// (it has no concept nodes yet, so the type floor is vacuously satisfied).
func Scaffold(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"index.md": "---\ntype: Index\n---\n\n# Knowledge Base\n\n_Progressive-disclosure entry point. Add \"start here\" links as nodes land._\n",
		"log.md":   "# Change Log\n\n_Append entries with `okfctl log append` (coming soon)._\n",
		".okf":     "okf_version: " + SpecVersion + "\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
```

`cmd/bundle.go`:

```go
package cmd

import (
	"fmt"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newBundleCmd() *cobra.Command {
	bundle := &cobra.Command{Use: "bundle", Short: "Bundle lifecycle commands"}
	bundle.AddCommand(&cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a minimal conformant OKF bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := okf.Scaffold(dir); err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized OKF bundle in %s\n", dir)
			return nil
		},
	})
	return bundle
}
```

Modify `cmd/root.go` — add inside `NewRootCmd()`:

```go
	root.AddCommand(newBundleCmd())
```

- [ ] **Step 4: Write the command test** — `cmd/bundle_test.go`

```go
package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundleInit_CreatesValidatableBundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err != nil {
		t.Fatalf("index.md not created: %v", err)
	}
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("freshly-init bundle failed validate: %v", err)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/okf/ -run TestScaffold -v && go test ./cmd/ -run TestBundleInit -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add internal/okf/reserved.go internal/okf/reserved_test.go cmd/bundle.go cmd/bundle_test.go cmd/root.go
git commit -S -m "feat(cmd): bundle init scaffolds a conformant bundle"
```

---

## Task 7: `node new` (requires type, §7.2)

**Files:**
- Create: `internal/okf/authoring.go`, `cmd/node.go`
- Modify: `cmd/root.go`
- Test: `internal/okf/authoring_test.go`, `cmd/node_test.go`

- [ ] **Step 1: Write the failing test** — `internal/okf/authoring_test.go`

```go
package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewNode_WritesConformantFile(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	path, err := NewNode(dir, "wine/tannin.md", "Reference", "Tannin")
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "type: Reference") {
		t.Errorf("node missing type; got:\n%s", data)
	}
	if f := Validate(mustLoad(t, dir)); len(f) != 0 {
		t.Errorf("authored node must validate clean, got %v", f)
	}
}

func TestNewNode_RejectsEmptyType(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if _, err := NewNode(dir, "x.md", "", "X"); err == nil {
		t.Fatal("NewNode with empty type must error")
	}
}

func TestNewNode_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if _, err := NewNode(dir, "a.md", "Reference", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewNode(dir, "a.md", "Reference", "A2"); err == nil {
		t.Fatal("NewNode must refuse to overwrite an existing node")
	}
}

func mustLoad(t *testing.T, dir string) *Bundle {
	t.Helper()
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

var _ = filepath.Join
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestNewNode -v`
Expected: FAIL — `NewNode` undefined.

- [ ] **Step 3: Write minimal implementation** — `internal/okf/authoring.go`

```go
package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewNode creates a conformant concept node at relPath (bundle-relative) with a
// required non-empty type (§7.2). It refuses an empty type and refuses to
// overwrite an existing file. Returns the absolute path written.
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
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: " + typ + "\n")
	if strings.TrimSpace(title) != "" {
		sb.WriteString("title: " + title + "\n")
	}
	sb.WriteString("---\n\n")
	heading := title
	if heading == "" {
		heading = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}
	sb.WriteString("# " + heading + "\n")
	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}
```

`cmd/node.go`:

```go
package cmd

import (
	"fmt"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	node := &cobra.Command{Use: "node", Short: "Author and inspect nodes"}

	var typ, title, dir string
	newC := &cobra.Command{
		Use:   "new <path>",
		Short: "Create a conformant node (type required, §7)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if typ == "" {
				return fmt.Errorf("--type is required (OKF §7: every node needs a non-empty type)")
			}
			p, err := okf.NewNode(dir, args[0], typ, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", p)
			return nil
		},
	}
	newC.Flags().StringVar(&typ, "type", "", "node type (required)")
	newC.Flags().StringVar(&title, "title", "", "node title")
	newC.Flags().StringVar(&dir, "bundle", ".", "bundle directory")
	node.AddCommand(newC)
	return node
}
```

Modify `cmd/root.go` — add inside `NewRootCmd()`:

```go
	root.AddCommand(newNodeCmd())
```

- [ ] **Step 4: Write the command test** — `cmd/node_test.go`

```go
package cmd

import "testing"

func TestNodeNew_RequiresType(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "node", "new", "x.md", "--bundle", dir); err == nil {
		t.Fatal("node new without --type must error")
	}
}

func TestNodeNew_CreatesNode(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./... -run 'NewNode|NodeNew' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add internal/okf/authoring.go internal/okf/authoring_test.go cmd/node.go cmd/node_test.go cmd/root.go
git commit -S -m "feat(cmd): node new creates a node with required type"
```

---

## Task 8: `node show` / `node list` (surface type, §7.3)

**Files:**
- Modify: `cmd/node.go`
- Test: `cmd/node_test.go` (append)

- [ ] **Step 1: Write the failing test** — append to `cmd/node_test.go`

```go
import (
	"strings"
	"testing"
)

func TestNodeList_SurfacesType(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "node", "list", "--bundle", dir)
	if err != nil {
		t.Fatalf("node list: %v", err)
	}
	if !strings.Contains(out, "wine/tannin.md") || !strings.Contains(out, "Reference") {
		t.Errorf("node list must surface path and type; got:\n%s", out)
	}
}

func TestNodeShow_SurfacesType(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	_, _ = runOKF(t, "node", "new", "a.md", "--type", "Playbook", "--bundle", dir)
	out, err := runOKF(t, "node", "show", "a.md", "--bundle", dir)
	if err != nil {
		t.Fatalf("node show: %v", err)
	}
	if !strings.Contains(out, "Playbook") {
		t.Errorf("node show must surface type; got:\n%s", out)
	}
}
```

> Note: if `cmd/node_test.go` already imports `testing`, merge imports rather
> than duplicating the import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run 'NodeList|NodeShow' -v`
Expected: FAIL — no `list`/`show` subcommands.

- [ ] **Step 3: Write minimal implementation** — add to `newNodeCmd()` in `cmd/node.go`, before `return node`

```go
	var showBundle string
	showC := &cobra.Command{
		Use:   "show <path>",
		Short: "Print a node, surfacing its type (§7.3)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(showBundle)
			if err != nil {
				return err
			}
			key := args[0]
			if len(key) < 3 || key[len(key)-3:] != ".md" {
				key += ".md"
			}
			n, ok := b.Nodes[key]
			if !ok {
				return fmt.Errorf("node not found: %s", key)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "path: %s\ntype: %s\n\n%s\n", n.Path, n.Type(), n.Body)
			return nil
		},
	}
	showC.Flags().StringVar(&showBundle, "bundle", ".", "bundle directory")
	node.AddCommand(showC)

	var listBundle string
	listC := &cobra.Command{
		Use:   "list",
		Short: "List nodes with their type (§7.3)",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(listBundle)
			if err != nil {
				return err
			}
			paths := make([]string, 0, len(b.Nodes))
			for p := range b.Nodes {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			for _, p := range paths {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", p, b.Nodes[p].Type())
			}
			return nil
		},
	}
	listC.Flags().StringVar(&listBundle, "bundle", ".", "bundle directory")
	node.AddCommand(listC)
```

Add `"sort"` to the `cmd/node.go` import block.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run 'NodeList|NodeShow' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./...
git add cmd/node.go cmd/node_test.go
git commit -S -m "feat(cmd): node show and list surface type"
```

---

## Task 9: `completion` + `config` + `bundle info`; README; full verify

**Files:**
- Create: `cmd/completion.go`, `cmd/config.go`
- Modify: `cmd/bundle.go` (add `info`), `cmd/root.go`, `README.md`
- Test: `cmd/config_test.go`, `cmd/bundle_test.go` (append)

- [ ] **Step 1: Write the failing test** — `cmd/config_test.go`

```go
package cmd

import (
	"strings"
	"testing"
)

func TestConfigSetGet_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", dir)
	if _, err := runOKF(t, "config", "set", "editor", "vim"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	out, err := runOKF(t, "config", "get", "editor")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if !strings.Contains(out, "vim") {
		t.Errorf("config get editor = %q, want vim", out)
	}
}
```

And append to `cmd/bundle_test.go`:

```go
func TestBundleInfo_ReportsCounts(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	_, _ = runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir)
	out, err := runOKF(t, "bundle", "info", dir)
	if err != nil {
		t.Fatalf("bundle info: %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("bundle info should report 1 node; got:\n%s", out)
	}
}
```

(Add `"strings"` to `cmd/bundle_test.go` imports.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run 'Config|BundleInfo' -v`
Expected: FAIL — no `config`/`info`.

- [ ] **Step 3: Write minimal implementations**

`cmd/config.go` (file-backed layered config; env `OKFCTL_CONFIG_HOME` overrides location):

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func configPath() string {
	home := os.Getenv("OKFCTL_CONFIG_HOME")
	if home == "" {
		if h, err := os.UserConfigDir(); err == nil {
			home = filepath.Join(h, "okfctl")
		} else {
			home = ".okfctl"
		}
	}
	return filepath.Join(home, "config.json")
}

func loadConfig() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func saveConfig(m map[string]string) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(p, data, 0o644)
}

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{Use: "config", Short: "Get and set okfctl configuration"}
	c.AddCommand(&cobra.Command{
		Use: "set <key> <value>", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			m[args[0]] = args[1]
			return saveConfig(m)
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "get <key>", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			v, ok := m[args[0]]
			if !ok {
				return fmt.Errorf("no such key: %s", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			for k, v := range m {
				fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", k, v)
			}
			return nil
		},
	})
	return c
}
```

`cmd/completion.go`:

```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [bash|zsh|fish]",
		Short:                 "Generate shell completion script",
		Args:                  cobra.ExactValidArgs(1),
		ValidArgs:             []string{"bash", "zsh", "fish"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			}
			return nil
		},
	}
}
```

Add `bundle info` to `cmd/bundle.go` (inside `newBundleCmd`, before `return`):

```go
	bundle.AddCommand(&cobra.Command{
		Use:   "info [dir]",
		Short: "Summarize a bundle (node count, spec version)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "nodes: %d\nreserved: %d\nokf_version: %s\n",
				len(b.Nodes), len(b.Reserved), okf.SpecVersion)
			return nil
		},
	})
```

Register both in `cmd/root.go` inside `NewRootCmd()` (completion needs `root`):

```go
	root.AddCommand(newConfigCmd())
	root.AddCommand(newCompletionCmd(root))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ -run 'Config|BundleInfo' -v`
Expected: PASS.

- [ ] **Step 5: Write README.md** — a real quickstart (init → node new → validate → list), the command tree, and a "built from docs/PRD.md; this is increment 1 of the roadmap" pointer. (No AI attribution.)

- [ ] **Step 6: Full verification (HARD GATE — do not claim done until green)**

```bash
gofmt -l .            # must print nothing
go vet ./...          # must be clean
go build ./...        # must succeed
go test ./... -race   # ALL packages green
CGO_ENABLED=0 go build -o /tmp/okfctl . && /tmp/okfctl bundle init /tmp/kb-demo && /tmp/okfctl node new demo.md --type Reference --bundle /tmp/kb-demo && /tmp/okfctl validate /tmp/kb-demo
```

Expected: end-to-end demo prints "Initialized…", "Created…", "OK: bundle conforms…".

- [ ] **Step 7: Commit**

```bash
git add cmd/config.go cmd/completion.go cmd/config_test.go cmd/bundle.go cmd/bundle_test.go cmd/root.go README.md
git commit -S -m "feat(cmd): completion, config, and bundle info; add README"
```

---

## Self-Review (run before requesting code review)

1. **Spec coverage vs `docs/specs/2026-07-22-walking-skeleton.md`:** foundation
   (T1,T9) · model+loader (T2,T3) · `bundle init` (T6) · `validate` floor incl.
   managed-`type` presence + unknown-type-passes (T4,T5) · `node new/show/list`
   (T7,T8). All success criteria mapped.
2. **Placeholder scan:** every code step has complete code; no TBD/TODO.
3. **Type consistency:** `Node`, `Bundle`, `Finding`, `Load`, `Validate`,
   `Scaffold`, `NewNode`, `NewRootCmd`, `SpecVersion` are defined once and reused
   with identical signatures across tasks. `--bundle` flag name is consistent
   across `node` subcommands.

## Execution handoff

Two options: **(1) Subagent-driven** (fresh subagent per task, review between) —
recommended for a plan this size; **(2) Inline** (executing-plans, batched with
checkpoints). Await Casey's choice.
