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
	case plan.VersionAction == okf.VersionHold:
		action = fmt.Sprintf("hold (%d prose-citation/missing-actor item(s) still rely on the §12.1 fallback "+
			"that declaring v%s turns off; resolve them and re-plan, or re-plan with --force-declare)",
			plan.FallbackItems(), plan.TargetVersion)
	case plan.VersionAction != okf.VersionKeep && plan.ForceDeclare:
		action = fmt.Sprintf("declare v%s (forced: %d prose-citation/missing-actor item(s) lose the §12.1 fallback)",
			plan.TargetVersion, plan.FallbackItems())
	case plan.VersionAction != okf.VersionKeep:
	case plan.DeclaredVersion == plan.TargetVersion:
		action = "keep (already v" + plan.TargetVersion + ")"
	default:
		action = "keep (§12: a version this build does not migrate from; consumed best-effort, never rewritten)"
	}
	var gen, src, titles, links, fields, stamps int
	for _, nm := range plan.Nodes {
		if nm.Generated != nil {
			gen++
		}
		src += len(nm.Sources)
		titles += len(nm.Titles)
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
	fmt.Fprintf(out, "edits: %d file(s); timestamp->generated %d, citations->sources %d, citation titles->sources %d, root-relative links->/ %d, root-relative path fields->/ %d, timestamps->RFC 3339 %d\n",
		len(plan.Nodes), gen, src, titles, links, fields, stamps)
	var ids, notes, refs int
	for _, nm := range plan.Nodes {
		notes += len(nm.Footnotes)
		ids += len(nm.IDs)
		for _, s := range nm.Sources {
			if s.ID != "" {
				ids++
			}
		}
		for _, r := range nm.Refs {
			refs += r.Count
		}
	}
	if ids+notes+refs > 0 {
		fmt.Fprintf(out, "attribution: source ids %d, footnote definitions %d, body references->footnotes %d\n", ids, notes, refs)
	}
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

// maxLCSCells and maxLCSLines bound the LCS table unifiedDiff builds for the
// changed middle of a file. Past either, the middle is shown as one replaced
// block: still a correct diff, only less minimal.
const (
	maxLCSCells = 4 << 20
	maxLCSLines = 1 << 16
)

// diffOp is one line of a unified diff: ' ' kept, '-' removed, '+' added. ai
// and bi are the 0-based line numbers in the old and new text.
type diffOp struct {
	kind   byte
	text   string
	ai, bi int
}

// unifiedDiff returns a unified diff (3 lines of context) between two texts,
// or "" when they are equal. The common prefix and suffix are matched first,
// so the LCS table covers only the changed middle: migrate's edits touch a
// few lines per file.
func unifiedDiff(path, a, b string) string {
	if a == b {
		return ""
	}
	al, bl := splitLines(a), splitLines(b)
	pre := 0
	for pre < len(al) && pre < len(bl) && al[pre] == bl[pre] {
		pre++
	}
	suf := 0
	for suf < len(al)-pre && suf < len(bl)-pre && al[len(al)-1-suf] == bl[len(bl)-1-suf] {
		suf++
	}
	var ops []diffOp
	for k := 0; k < pre; k++ {
		ops = append(ops, diffOp{' ', al[k], k, k})
	}
	ops = append(ops, middleOps(al[pre:len(al)-suf], bl[pre:len(bl)-suf], pre)...)
	for k := 0; k < suf; k++ {
		ai, bi := len(al)-suf+k, len(bl)-suf+k
		ops = append(ops, diffOp{' ', al[ai], ai, bi})
	}
	return renderHunks(path, ops)
}

// middleOps diffs the changed middle of two texts that both start at line
// offset off. It uses an LCS table when the table is small enough, otherwise
// it reports the whole middle as removed then added.
func middleOps(am, bm []string, off int) []diffOp {
	n, m := len(am), len(bm)
	var ops []diffOp
	if n == 0 || m == 0 || n >= maxLCSLines || m >= maxLCSLines || n*m > maxLCSCells {
		for k, s := range am {
			ops = append(ops, diffOp{'-', s, off + k, off})
		}
		for k, s := range bm {
			ops = append(ops, diffOp{'+', s, off + n, off + k})
		}
		return ops
	}
	// One flat table of (n+1)*(m+1) cells, grown by append inside loops that
	// the guard above bounds (both dimensions below maxLCSLines, the product
	// within maxLCSCells), so no allocation size is computed arithmetically.
	w := len(bm) + 1
	var cells []int
	for i := 0; i <= n; i++ {
		for j := 0; j <= m; j++ {
			cells = append(cells, 0)
		}
	}
	at := func(i, j int) int { return cells[i*w+j] }
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if am[i] == bm[j] {
				cells[i*w+j] = at(i+1, j+1) + 1
			} else {
				cells[i*w+j] = max(at(i+1, j), at(i, j+1))
			}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && am[i] == bm[j]:
			ops = append(ops, diffOp{' ', am[i], off + i, off + j})
			i++
			j++
		case i < n && (j == m || at(i+1, j) >= at(i, j+1)):
			ops = append(ops, diffOp{'-', am[i], off + i, off + j})
			i++
		default:
			ops = append(ops, diffOp{'+', bm[j], off + i, off + j})
			j++
		}
	}
	return ops
}

// renderHunks groups diff lines into unified-diff hunks with 3 lines of
// context.
func renderHunks(path string, ops []diffOp) string {
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
