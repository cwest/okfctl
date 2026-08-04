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

package cmd

//go:generate go run ./gendocs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// referencePreamble is the fixed header of the generated command reference. It
// intentionally repeats the "`okfctl <cmd> --help` is authoritative" framing so
// the page states its own contract: this file is generated from the binary, and
// the binary's --help is the always-current source.
const referencePreamble = `# Command reference

**This page is generated from the command tree. Do not edit it by hand.**
Regenerate it with ` + "`go generate ./cmd`" + ` (or ` + "`go run ./cmd/gendocs`" + `); a
CI check (` + "`TestCommandReference_NoDrift`" + `) fails when the committed file drifts
from the binary, so the reference cannot go stale.

` + "`okfctl <cmd> --help`" + ` is the authoritative, always-current form for any
command — it prints the same description, flags, and runnable example straight
from the binary. This page mirrors that surface in one place, one section per
command, so it is browsable and linkable (README links to the ` + "`#okfctl-<cmd>`" + `
anchors below).

Run ` + "`okfctl help`" + ` for the top-level list, or ` + "`okfctl <cmd> --help`" + ` for any
entry below.
`

// GenerateCommandReference renders the whole okfctl command tree as a single
// markdown reference. It is deterministic (no timestamps, tree-ordered) so a
// drift check can compare its output byte-for-byte to the committed file.
//
// Layout is chosen to keep README's links alive: each top-level command gets a
// `## okfctl <cmd>` heading, which GitHub slugifies to `#okfctl-<cmd>` — exactly
// what README.md links to. Subcommands render as `### okfctl <cmd> <sub>` under
// their parent. The framework-provided `help` command is omitted (its help text
// is not ours to author); `completion` is documented like any other command.
func GenerateCommandReference(root *cobra.Command) string {
	var b bytes.Buffer
	b.WriteString(referencePreamble)

	for _, top := range root.Commands() {
		if !isDocumentable(top) {
			continue
		}
		b.WriteString("\n")
		renderCommand(&b, top, 2)
		// Document runnable subcommands one heading level deeper.
		for _, sub := range top.Commands() {
			if !isDocumentable(sub) {
				continue
			}
			b.WriteString("\n")
			renderCommand(&b, sub, 3)
		}
	}
	// End the file with exactly one trailing newline. Each section ends with a
	// blank line, which would otherwise leave the file ending in "\n\n" — the
	// end-of-file-fixer pre-commit hook strips that to a single newline, and if
	// the generator disagreed with the hook the drift check (which compares the
	// committed, hook-fixed file byte-for-byte) would fail on every commit.
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// isDocumentable reports whether a command belongs in the reference. It filters
// the same set cobra hides from help (unavailable, additional-help-topic) plus
// the framework-generated `help` command whose text we do not author.
func isDocumentable(c *cobra.Command) bool {
	if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
		return false
	}
	if c.Name() == "help" {
		return false
	}
	return true
}

// renderCommand writes one command's section at the given markdown heading level
// (2 for a top-level command, 3 for a subcommand). It mirrors the --help surface:
// heading, short summary, long synopsis, usage line, example, and flags.
func renderCommand(b *bytes.Buffer, c *cobra.Command, level int) {
	path := c.CommandPath() // e.g. "okfctl bundle init"
	fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level), path)

	if s := strings.TrimSpace(c.Short); s != "" {
		fmt.Fprintf(b, "%s\n\n", s)
	}
	if l := strings.TrimSpace(c.Long); l != "" {
		fmt.Fprintf(b, "%s\n\n", l)
	}
	if c.Runnable() {
		fmt.Fprintf(b, "```\n%s\n```\n\n", c.UseLine())
	}
	if ex := strings.TrimSpace(c.Example); ex != "" {
		b.WriteString("Example:\n\n")
		fmt.Fprintf(b, "```\n%s\n```\n\n", trimTrailingSpaceEachLine(ex))
	}
	writeFlags(b, c)
}

// writeFlags renders a command's own (non-inherited) flags as a fenced block,
// matching how cobra prints them under --help. Commands with no local flags emit
// nothing, keeping the reference tight.
func writeFlags(b *bytes.Buffer, c *cobra.Command) {
	flags := c.NonInheritedFlags()
	if !flags.HasAvailableFlags() {
		return
	}
	var buf bytes.Buffer
	flags.SetOutput(&buf)
	b.WriteString("Flags:\n\n```\n")
	flags.PrintDefaults()
	b.WriteString(strings.TrimRight(buf.String(), "\n"))
	b.WriteString("\n```\n\n")
}

// trimTrailingSpaceEachLine strips trailing whitespace from every line. Example
// strings are hand-indented in the command definitions; trimming keeps the
// generated block gofmt-stable and free of trailing-space diffs.
func trimTrailingSpaceEachLine(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	return strings.Join(lines, "\n")
}
