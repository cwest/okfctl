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

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
)

// printMigrateDryRun renders every file the plan would change as a unified
// diff, through the same renderer the real apply writes with, then a summary.
// It writes nothing to the bundle.
func printMigrateDryRun(out io.Writer, dir string, b *okf.Bundle, plan okf.MigratePlan) error {
	type change struct {
		path          string
		before, after []byte
	}
	var changes []change
	for _, nm := range plan.Nodes {
		before, after, err := okf.RenderMigration(dir, b, nm)
		if err != nil {
			return err
		}
		if !bytes.Equal(before, after) {
			changes = append(changes, change{nm.Path, before, after})
		}
	}
	markers, err := okf.RenderVersionMarkers(dir, plan)
	if err != nil {
		return err
	}
	for p, ba := range markers {
		changes = append(changes, change{p, ba[0], ba[1]})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].path < changes[j].path })
	for _, c := range changes {
		fmt.Fprint(out, unifiedDiff(c.path, string(c.before), string(c.after)))
	}
	writeMigrateSummary(out, plan)
	fmt.Fprintf(out, "%d file(s) would change (dry run; nothing written).\n", len(changes))
	return nil
}

// writeMigrateSummary prints the plan's edit and judgment counts by kind.
func writeMigrateSummary(out io.Writer, plan okf.MigratePlan) {
	declared := plan.DeclaredVersion
	if declared == "" {
		declared = "undeclared"
	}
	action := "declare v" + plan.TargetVersion
	switch {
	case plan.VersionAction != okf.VersionKeep:
	case plan.DeclaredVersion == plan.TargetVersion:
		action = "keep (already v" + plan.TargetVersion + ")"
	default:
		action = "keep (§12: a version this build does not migrate from; consumed best-effort, never rewritten)"
	}
	var gen, src, links, fields, stamps int
	for _, nm := range plan.Nodes {
		if nm.Generated != nil {
			gen++
		}
		src += len(nm.Sources)
		stamps += len(nm.Timestamps)
		for _, p := range nm.Paths {
			if p.Field == "link" {
				links++
			} else {
				fields++
			}
		}
	}
	fmt.Fprintf(out, "version: %s -> %s\n", declared, action)
	fmt.Fprintf(out, "edits: %d file(s); timestamp->generated %d, citations->sources %d, root-relative links->/ %d, root-relative path fields->/ %d, timestamps->RFC 3339 %d\n",
		len(plan.Nodes), gen, src, links, fields, stamps)
	byKind := map[okf.JudgmentKind]int{}
	for _, j := range plan.Judgment {
		byKind[j.Kind]++
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, fmt.Sprintf("%s %d", k, byKind[k]))
	}
	sort.Strings(kinds)
	if len(kinds) == 0 {
		kinds = []string{"none"}
	}
	fmt.Fprintf(out, "judgment: %d item(s); %s\n", len(plan.Judgment), strings.Join(kinds, ", "))
}

// unifiedDiff returns a unified diff (3 lines of context) between two texts,
// or "" when they are equal. It is a plain LCS line diff: migrate's edits
// touch few lines per file, and the files are small.
func unifiedDiff(path, a, b string) string {
	if a == b {
		return ""
	}
	al, bl := splitLines(a), splitLines(b)
	n, m := len(al), len(bl)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type op struct {
		kind byte // ' ', '-', '+'
		text string
		ai   int
		bi   int
	}
	var ops []op
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && al[i] == bl[j]:
			ops = append(ops, op{' ', al[i], i, j})
			i++
			j++
		case i < n && (j == m || lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, op{'-', al[i], i, j})
			i++
		default:
			ops = append(ops, op{'+', bl[j], i, j})
			j++
		}
	}
	const ctx = 3
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)
	for k := 0; k < len(ops); {
		if ops[k].kind == ' ' {
			k++
			continue
		}
		start := max(k-ctx, 0)
		end := k
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := end
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run == len(ops) || run-end > 2*ctx {
				end = min(end+ctx, len(ops))
				break
			}
			end = run
		}
		var aLen, bLen int
		for _, o := range ops[start:end] {
			if o.kind != '+' {
				aLen++
			}
			if o.kind != '-' {
				bLen++
			}
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", ops[start].ai+1, aLen, ops[start].bi+1, bLen)
		for _, o := range ops[start:end] {
			sb.WriteByte(o.kind)
			sb.WriteString(o.text)
			sb.WriteByte('\n')
		}
		k = end
	}
	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
