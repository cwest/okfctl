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

// Command gendocs regenerates the per-command markdown reference at
// docs/commands/README.md from the live okfctl cobra tree. It is the single
// sanctioned way to update that file — the committed copy is validated against a
// fresh regeneration by cmd.TestCommandReference_NoDrift, so hand-editing it only
// gets reverted by the next `go generate`.
//
// Usage:
//
//	go generate ./cmd     # via the //go:generate directive in cmd/docs.go
//	go run ./cmd/gendocs  # direct
//
// The output path is resolved relative to the module root (the parent of this
// program's directory), so it works from any working directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cwest/okfctl/cmd"
)

func main() {
	out := referencePath()
	content := cmd.GenerateCommandReference(cmd.NewRootCmd())
	// Generated docs are shareable reference files; 0o755/0o644 is intended.
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil { //nolint:gosec // G301: shareable generated-docs dir
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil { //nolint:gosec // G306: shareable generated-docs file
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}

// referencePath returns the absolute path of docs/commands/README.md, resolved
// from this source file's location so it does not depend on the caller's cwd.
// This file lives at <module>/cmd/gendocs/main.go, so the module root is three
// directories up.
func referencePath() string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "gendocs: cannot resolve source path")
		os.Exit(1)
	}
	moduleRoot := filepath.Dir(filepath.Dir(filepath.Dir(self)))
	return filepath.Join(moduleRoot, "docs", "commands", "README.md")
}
