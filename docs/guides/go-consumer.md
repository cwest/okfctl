# Consuming a bundle from Go with `pkg/okf`

`pkg/okf` is the stable, read-only Go front door for an OKF bundle. If your
program is written in Go, you import this package and call the loader directly
instead of shelling out to the `okfctl` binary and parsing its text.

It is a pure delegation layer over the tool's own implementation: every function
forwards to the exact code the CLI runs, and every domain type is a Go type
alias, so your program and the CLI can never disagree about what a bundle
contains or whether it conforms.

## Install

```sh
go get github.com/cwest/okfctl/pkg/okf
```

The package depends on the Go standard library and okfctl alone. Everything runs
in process—the loader, validator, linter, search, and analysis each execute
directly, so a consumer stands up nothing external before calling them.

## Load, then read

Everything starts with `Load`, which walks a bundle root, parses every `.md`
file, and builds the in-memory graph. The returned `*okf.Bundle` is then the
argument to every read function.

```go
package main

import (
	"fmt"
	"log"

	"github.com/cwest/okfctl/pkg/okf"
)

func main() {
	bundle, err := okf.Load("./bundles/knowledge")
	if err != nil {
		log.Fatal(err)
	}

	// Spec floor (§6.2, §7.1): an empty slice means the bundle conforms.
	if findings := okf.Validate(bundle); len(findings) != 0 {
		for _, f := range findings {
			fmt.Printf("FAIL %s: %s\n", f.Path, f.Message)
		}
		log.Fatalf("%d conformance finding(s)", len(findings))
	}

	// Curation guidance (never a spec failure).
	for _, f := range okf.Lint(bundle, okf.LintOptions{}) {
		fmt.Println(f.Message)
	}

	// Lexical search across all surfaces.
	for _, r := range okf.Search(bundle, "income statement", okf.FieldAny) {
		fmt.Printf("%s [%s]\n", r.Path, r.Type)
	}
}
```

By default the walk skips vendored and derived directories (a virtualenv or a
build tree under the bundle root should not become knowledge). Pass
`okf.WithNoIgnore()` to `Load` to restore the full walk.

## The read surface

| Function | Returns | Purpose |
|---|---|---|
| `Load(root, opts...)` | `*Bundle, error` | parse a bundle into memory |
| `Validate(b)` | `[]Finding` | the OKF spec floor (§6.2, §7.1); empty == conforms |
| `Lint(b, opts)` | `[]LintFinding` | deterministic structural curation guidance |
| `BuildGraph(b)` | `Graph` | the serializable concept-node link graph |
| `Search(b, query, field)` | `[]SearchResult` | case-insensitive lexical query |
| `Neighborhood(b, start, depth)` | `[]NeighborResult, bool` | graph-structural (undirected) traversal |
| `Analyze(b, opts)` | `AnalyzeReport` | the read-only five-dimension curation report |

`Validate` and `Lint` return exactly what `okfctl validate` and `okfctl lint`
print, because the CLI renders these same slices verbatim—the facade and the CLI
share one implementation.

## Read-only by design

There is deliberately no write path here. `pkg/okf` exports no `New`, `Move`,
`Touch`, index build, or log append—exercising the entire package over a bundle
never mutates a single file. A stable read contract is what a consumer can depend
on today; the write path stays inside the tool until its contract is frozen.

## Type aliases, not copies

`okf.Bundle` is `type Bundle = internal.Bundle`, so a value you obtain here IS
the type the tool uses internally. There is one `Bundle` in your program, not two
that can drift, and a bundle from the facade is assignable wherever the internal
type is expected. This is what lets the read surface stay a thin delegation
rather than a second dialect of the same logic.

## Runnable examples

Every function on the package's [pkg.go.dev
page](https://pkg.go.dev/github.com/cwest/okfctl/pkg/okf) carries a runnable
`Example`. They build a tiny bundle in a temp dir, so you can copy one and run it
without a bundle of your own on hand.
