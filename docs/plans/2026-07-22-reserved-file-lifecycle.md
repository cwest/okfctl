# okfctl Increment 2a — Reserved-file Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Deliver `index build`, `index check`, `log append`, `log show` — the reserved-file engine that keeps `index.md` and `log.md` regenerable and verifiable.

**Architecture:** New cobra-free model code in `internal/okf` renders a deterministic neighborhood-grouped index, compares it to disk, and appends log entries. `cmd/index.go` + `cmd/log.go` are thin adapters registered in `NewRootCmd()`.

**Tech Stack:** Go 1.26; existing deps (cobra v1.10.2, yaml.v3). No new deps.

**Ground truth (existing API this builds on):** `okf.Load(root)(*Bundle,error)`; `Bundle{Nodes map[string]*Node, Reserved map[string]*Node, OkfVersion string}` with `OutboundLinks`; `Node{Path, Frontmatter, Body}` + `Type() string`; `okf.Validate(b)[]Finding`; `okf.ReservedFiles` map; `okf.SpecVersion`. Nodes are keyed by bundle-relative slash-path. The cmd test helper `runOKF(t, args...)(string,error)` exists in `cmd/validate_test.go` — REUSE it.

**Constraints for the implementer:** commits signed (`-S`), repo-local identity is already `Casey West <casey@geeknest.com>` (never change; never @google); NO AI attribution anywhere; the pre-commit addlicense hook stamps `Copyright 2026 Casey West` on .go files (expected — if a commit is blocked "files were modified by this hook", run `addlicense -l apache -c "Casey West" -y 2026 <files>` then re-add + commit). Verify by ground truth; never fabricate output.

---

## Task 1: RenderIndex — deterministic neighborhood-grouped index

**Files:**
- Create: `internal/okf/reserved_lifecycle.go`
- Test: `internal/okf/reserved_lifecycle_test.go`

- [ ] **Step 1: Write the failing test** — `internal/okf/reserved_lifecycle_test.go`

```go
package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNode(t *testing.T, dir, rel, typ, title string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderIndex_GroupsByNeighborhoodDeterministically(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	writeNode(t, dir, "lifting/squat.md", "Playbook", "Squat")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	// Frontmatter present and type Index.
	if !strings.HasPrefix(got, "---\ntype: Index\n") {
		t.Errorf("index missing Index frontmatter; got:\n%s", got)
	}
	// Neighborhoods sorted: "lifting" before "wine".
	li := strings.Index(got, "lifting")
	wi := strings.Index(got, "wine")
	if li < 0 || wi < 0 || li > wi {
		t.Errorf("neighborhoods not sorted (lifting before wine); got:\n%s", got)
	}
	// Nodes sorted within a neighborhood: acidity before tannin.
	ai := strings.Index(got, "acidity.md")
	ti := strings.Index(got, "tannin.md")
	if ai < 0 || ti < 0 || ai > ti {
		t.Errorf("nodes not sorted within neighborhood; got:\n%s", got)
	}
	// Each node is a markdown link carrying its type.
	if !strings.Contains(got, "[Tannin](wine/tannin.md)") {
		t.Errorf("node not rendered as a titled link; got:\n%s", got)
	}
	if !strings.Contains(got, "Reference") {
		t.Errorf("node type not surfaced; got:\n%s", got)
	}
}

func TestRenderIndex_Deterministic(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "a/one.md", "Reference", "One")
	writeNode(t, dir, "b/two.md", "Reference", "Two")
	b, _ := Load(dir)
	if RenderIndex(b) != RenderIndex(b) {
		t.Fatal("RenderIndex is not deterministic across calls")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/okf/ -run TestRenderIndex -v`
Expected: FAIL — `RenderIndex` undefined.

- [ ] **Step 3: Write minimal implementation** — `internal/okf/reserved_lifecycle.go`

```go
package okf

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// neighborhood returns the top-level directory of a bundle-relative slash path,
// or "" for a root-level node (rendered under a "(root)" group).
func neighborhood(rel string) string {
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

// nodeTitle returns the node's title frontmatter, falling back to the file's
// base name (without .md) when absent.
func nodeTitle(n *Node) string {
	if t, ok := n.Frontmatter["title"].(string); ok && strings.TrimSpace(t) != "" {
		return t
	}
	return strings.TrimSuffix(path.Base(n.Path), ".md")
}

// RenderIndex produces a deterministic, neighborhood-grouped index.md body for
// the bundle: Index frontmatter, then one section per top-level neighborhood
// (sorted), each listing its concept nodes (sorted by path) as titled markdown
// links annotated with the node's type. Reserved files are excluded. The output
// is byte-stable across runs (all ordering is via sort), and passes Validate.
func RenderIndex(b *Bundle) string {
	groups := map[string][]string{}
	for p := range b.Nodes {
		nb := neighborhood(p)
		groups[nb] = append(groups[nb], p)
	}
	names := make([]string, 0, len(groups))
	for nb := range groups {
		names = append(names, nb)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("---\ntype: Index\n---\n\n# Knowledge Base\n\n")
	sb.WriteString("_Generated by `okfctl index build`. Do not edit by hand; run `okfctl index build` to regenerate._\n")
	if len(names) == 0 {
		sb.WriteString("\n_No nodes yet._\n")
		return sb.String()
	}
	for _, nb := range names {
		heading := nb
		if heading == "" {
			heading = "(root)"
		}
		sb.WriteString("\n## " + heading + "\n\n")
		paths := groups[nb]
		sort.Strings(paths)
		for _, p := range paths {
			n := b.Nodes[p]
			sb.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", nodeTitle(n), p, n.Type()))
		}
	}
	return sb.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/okf/ -run TestRenderIndex -v`
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./... -count=1
git add internal/okf/reserved_lifecycle.go internal/okf/reserved_lifecycle_test.go
git commit -S -m "feat(okf): render deterministic neighborhood-grouped index"
```

---

## Task 2: IndexInSync + AppendLog + ReadLog (model)

**Files:**
- Modify: `internal/okf/reserved_lifecycle.go`
- Test: `internal/okf/reserved_lifecycle_test.go` (append)

- [ ] **Step 1: Write the failing tests** — append to `internal/okf/reserved_lifecycle_test.go`

```go
func TestIndexInSync_TrueAfterBuildFalseAfterChange(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	b, _ := Load(dir)
	// Write the rendered index to disk, then it should be in sync.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(RenderIndex(b)), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, _ := Load(dir)
	if ok, diff := IndexInSync(b2); !ok {
		t.Errorf("index should be in sync right after build; diff:\n%s", diff)
	}
	// Add a node → now stale.
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	b3, _ := Load(dir)
	if ok, _ := IndexInSync(b3); ok {
		t.Error("index should be STALE after adding a node")
	}
}

func TestAppendLog_CreatesAndAccumulates(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if err := AppendLog(dir, "first change"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "second change"); err != nil {
		t.Fatal(err)
	}
	body, err := ReadLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "first change") || !strings.Contains(body, "second change") {
		t.Errorf("log lost an entry; got:\n%s", body)
	}
	// Newest-first: second change appears before first.
	if strings.Index(body, "second change") > strings.Index(body, "first change") {
		t.Errorf("log not newest-first; got:\n%s", body)
	}
	// Entries are timestamped (ISO date present).
	if !strings.Contains(body, "20") { // year prefix; good enough as a smoke check
		t.Errorf("log entry missing a timestamp; got:\n%s", body)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/okf/ -run 'IndexInSync|AppendLog' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement** — append to `internal/okf/reserved_lifecycle.go`

```go
import (
	// add to the existing import block:
	"os"
	"path/filepath"
	"time"
)

// IndexInSync reports whether the on-disk index.md matches what RenderIndex would
// generate for the current bundle. When stale, it returns a short human-readable
// report. A missing index.md counts as out of sync.
func IndexInSync(b *Bundle) (bool, string) {
	want := RenderIndex(b)
	onDisk, err := os.ReadFile(filepath.Join(b.Root, "index.md"))
	if err != nil {
		return false, "index.md is missing or unreadable; run `okfctl index build`"
	}
	if string(onDisk) == want {
		return true, ""
	}
	return false, "index.md is out of date; run `okfctl index build` to regenerate"
}

// AppendLog prepends a timestamped entry to log.md (newest-first), creating the
// file with a heading when absent. The message is written as a single bullet;
// a multi-line message is flattened to its first line to keep the log well-formed.
func AppendLog(root, message string) error {
	message = strings.TrimSpace(message)
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		message = message[:i]
	}
	if message == "" {
		return fmt.Errorf("log message must not be empty")
	}
	entry := fmt.Sprintf("- %s — %s\n", time.Now().UTC().Format("2006-01-02"), message)

	p := filepath.Join(root, "log.md")
	const header = "# Change Log\n\n"
	existing, err := os.ReadFile(p)
	if err != nil {
		// Create fresh.
		return os.WriteFile(p, []byte(header+entry), 0o644)
	}
	body := string(existing)
	rest := strings.TrimPrefix(body, header)
	return os.WriteFile(p, []byte(header+entry+rest), 0o644)
}

// ReadLog returns the log.md body (empty string if the file is absent).
func ReadLog(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "log.md"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/okf/ -run 'IndexInSync|AppendLog' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./... -count=1
git add internal/okf/reserved_lifecycle.go internal/okf/reserved_lifecycle_test.go
git commit -S -m "feat(okf): index sync check and log append/read"
```

---

## Task 3: `index build` / `index check` commands

**Files:**
- Create: `cmd/index.go`, `cmd/index_test.go`
- Modify: `cmd/root.go` (register `newIndexCmd()` after the existing AddCommand lines)

- [ ] **Step 1: Write the failing test** — `cmd/index_test.go`

```go
package cmd

import (
	"path/filepath"
	"testing"
)

func TestIndexBuildThenCheck_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass right after build: %v", err)
	}
}

func TestIndexCheck_StaleExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	_, _ = runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir)
	_, _ = runOKF(t, "index", "build", dir)
	// add a node → index now stale
	_, _ = runOKF(t, "node", "new", "b.md", "--type", "Reference", "--bundle", dir)
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Fatal("index check must exit nonzero on a stale index")
	}
	_ = filepath.Join
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/ -run TestIndex -v`
Expected: FAIL — no `index` command.

- [ ] **Step 3: Implement** — `cmd/index.go`

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	index := &cobra.Command{Use: "index", Short: "Manage the reserved index.md"}

	index.AddCommand(&cobra.Command{
		Use:   "build [dir]",
		Short: "Regenerate index.md from the current bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			out := okf.RenderIndex(b)
			if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(out), 0o644); err != nil {
				return fmt.Errorf("write index.md: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", filepath.Join(dir, "index.md"))
			return nil
		},
	})

	index.AddCommand(&cobra.Command{
		Use:   "check [dir]",
		Short: "Verify index.md is current (nonzero exit if stale)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			ok, report := okf.IndexInSync(b)
			if ok {
				fmt.Fprintln(cmd.OutOrStdout(), "OK: index.md is current")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), report)
			return fmt.Errorf("index.md is out of date")
		},
	})
	return index
}

// bundleDirArg returns the positional bundle dir or "." when omitted.
func bundleDirArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}
```

Register in `cmd/root.go` inside `NewRootCmd()` after the existing AddCommand lines, before `return root`:

```go
	root.AddCommand(newIndexCmd())
```

> Note: `bundleDirArg` is a new shared helper in `cmd/index.go`. If `cmd/log.go` (Task 4) is written first or in parallel, define `bundleDirArg` in exactly ONE file to avoid a duplicate-declaration build error.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/ -run TestIndex -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && go vet ./... && go test ./... -count=1
git add cmd/index.go cmd/index_test.go cmd/root.go
git commit -S -m "feat(cmd): index build and check commands"
```

---

## Task 4: `log append` / `log show` commands + increment README/verify

**Files:**
- Create: `cmd/log.go`, `cmd/log_test.go`
- Modify: `cmd/root.go` (register `newLogCmd()`), `README.md` (document the new verbs)

- [ ] **Step 1: Write the failing test** — `cmd/log_test.go`

```go
package cmd

import (
	"strings"
	"testing"
)

func TestLogAppendThenShow(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "log", "append", dir, "--message", "added tannin node"); err != nil {
		t.Fatalf("log append: %v", err)
	}
	out, err := runOKF(t, "log", "show", dir)
	if err != nil {
		t.Fatalf("log show: %v", err)
	}
	if !strings.Contains(out, "added tannin node") {
		t.Errorf("log show missing the appended entry; got:\n%s", out)
	}
}

func TestLogAppend_RequiresMessage(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	if _, err := runOKF(t, "log", "append", dir); err == nil {
		t.Fatal("log append without --message must error")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/ -run TestLog -v`
Expected: FAIL — no `log` command.

- [ ] **Step 3: Implement** — `cmd/log.go`

```go
package cmd

import (
	"fmt"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	logCmd := &cobra.Command{Use: "log", Short: "Manage the reserved log.md change history"}

	var msg string
	appendC := &cobra.Command{
		Use:   "append [dir]",
		Short: "Append a timestamped change entry to log.md",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if msg == "" {
				return fmt.Errorf("--message is required")
			}
			dir := bundleDirArg(args)
			if err := okf.AppendLog(dir, msg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Appended log entry")
			return nil
		},
	}
	appendC.Flags().StringVar(&msg, "message", "", "the change entry text (required)")
	logCmd.AddCommand(appendC)

	logCmd.AddCommand(&cobra.Command{
		Use:   "show [dir]",
		Short: "Print the change history",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			body, err := okf.ReadLog(dir)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), body)
			return nil
		},
	})
	return logCmd
}
```

Register in `cmd/root.go`:

```go
	root.AddCommand(newLogCmd())
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/ -run TestLog -v`
Expected: PASS.

- [ ] **Step 5: Update README.md** — add `index build/check` and `log append/show` to the command reference and quickstart (keep it accurate to what's implemented; do NOT document 2b's node edit/mv/rm). No AI attribution.

- [ ] **Step 6: Full verification gate (HARD — paste real output)**

```bash
gofmt -l .            # empty
go vet ./...          # clean
CGO_ENABLED=0 go build -o /tmp/okfctl-2a .
go test ./... -race -count=1   # all green
# E2E against the built binary:
rm -rf /tmp/kb-2a
/tmp/okfctl-2a bundle init /tmp/kb-2a
/tmp/okfctl-2a node new wine/tannin.md --type Reference --title Tannin --bundle /tmp/kb-2a
/tmp/okfctl-2a index build /tmp/kb-2a
/tmp/okfctl-2a index check /tmp/kb-2a          # expect OK, exit 0
/tmp/okfctl-2a node new wine/acidity.md --type Reference --bundle /tmp/kb-2a
/tmp/okfctl-2a index check /tmp/kb-2a; echo "stale exit: $?"   # expect nonzero
/tmp/okfctl-2a index build /tmp/kb-2a
/tmp/okfctl-2a log append /tmp/kb-2a --message "added acidity"
/tmp/okfctl-2a log show /tmp/kb-2a
/tmp/okfctl-2a validate /tmp/kb-2a             # still OK
# determinism: build twice, diff must be empty
/tmp/okfctl-2a index build /tmp/kb-2a && cp /tmp/kb-2a/index.md /tmp/idx1
/tmp/okfctl-2a index build /tmp/kb-2a && diff /tmp/idx1 /tmp/kb-2a/index.md && echo "DETERMINISTIC"
```

- [ ] **Step 7: Commit**

```bash
git add cmd/log.go cmd/log_test.go cmd/root.go README.md
git commit -S -m "feat(cmd): log append and show; document reserved-file verbs"
```

---

## Self-Review (run before requesting review)

1. **Spec coverage** vs `docs/specs/2026-07-22-reserved-file-lifecycle.md`: RenderIndex (T1) · IndexInSync/AppendLog/ReadLog (T2) · index build/check (T3) · log append/show + README + verify (T4). All success criteria mapped.
2. **Placeholder scan:** every step has complete code; no TBD.
3. **Type consistency:** `RenderIndex`, `IndexInSync`, `AppendLog`, `ReadLog`, `bundleDirArg`, `newIndexCmd`, `newLogCmd` defined once, reused with identical signatures. `bundleDirArg` declared in exactly one file.
4. **Determinism:** all index ordering is via `sort` — no map-iteration output (the class Lamport bounced in increment 1). The verify gate proves byte-identical repeated builds.

## Execution handoff

Subagent-driven (recommended) or inline. Each task: implementer → spec review → quality review before advancing.
