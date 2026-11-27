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
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gitOutput runs a git command and returns its stdout. It is a variable so
// tests can count how many git processes a scan spawns.
var gitOutput = func(c *exec.Cmd) ([]byte, error) { return c.Output() }

// gitCommit is one commit of the HEAD history, as `git log` reports it.
type gitCommit struct {
	sha     string
	unix    int64  // committer timestamp: git's walk order key
	iso     string // committer date, strict ISO-8601 (%cI)
	parents []int  // indexes into gitHistory.commits
	// changed[i] is the set of bundle-relative paths (files and every parent
	// directory of a changed file) that differ between parents[i] and this
	// commit. A root commit has a single entry, against the empty tree.
	changed []map[string]bool
}

// gitHistory answers GitLastCommitDateIgnoring for every path of a bundle from
// two git processes total, instead of one `git log` per node.
//
// It loads the HEAD commit graph once (`git log`) and the paths each commit
// changed against each of its parents once (`git diff-tree --stdin`), then
// replays, per path, the walk `git log -- <path>` performs: git's default
// history simplification (a commit TREESAME to its parent is hidden; a merge
// TREESAME to some parent follows only the first such parent; a root commit is
// shown only when it contains the path), in committer-date order with ties
// broken first-in-first-out. Replaying the same walk is what keeps the answer —
// and so every drift finding — identical to the per-path lookup.
type gitHistory struct {
	root    string
	commits []gitCommit
	head    int
	// perPath forces the per-path lookup for every path: the user's git
	// config changes what a per-path `git log` reports (log.follow follows
	// renames; log.showSignature injects signature lines) in ways this
	// replay does not model.
	perPath bool
}

// loadGitHistory reads root's HEAD history. It returns (nil, nil) when git
// cannot answer at all — git absent, root not in a repository, or no commits
// yet — the same "no answer" the per-path lookup gives for every path. A
// non-nil error means the batched read failed unexpectedly; callers fall back
// to the per-path lookup.
func loadGitHistory(root string) (*gitHistory, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil
	}
	h := &gitHistory{root: root}
	if perPathConfig(root) {
		h.perPath = true
		return h, nil
	}

	// %ct orders the walk, %cI is the date the per-path lookup returns, %P
	// the parents. No pathspec: the full graph, simplified per path below.
	out, err := gitOutput(exec.Command("git", "-C", root, "log", "--no-show-signature", "--format=%H %ct %cI %P", "HEAD")) //nolint:gosec // G204: fixed git subcommand over the user's bundle
	if err != nil {
		return nil, nil // not a repo, or no HEAD yet: no answer
	}
	index := map[string]int{}
	var parentSHAs [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		unix, perr := strconv.ParseInt(f[1], 10, 64)
		if perr != nil {
			return nil, fmt.Errorf("git log: bad committer timestamp %q", f[1])
		}
		index[f[0]] = len(h.commits)
		h.commits = append(h.commits, gitCommit{sha: f[0], unix: unix, iso: f[2]})
		parentSHAs = append(parentSHAs, f[3:])
	}
	if len(h.commits) == 0 {
		return nil, nil
	}
	h.head = 0 // git log lists HEAD first

	// One diff-tree line per (commit, parent) pair, or the bare commit for a
	// root, so every TREESAME question below is a set lookup.
	var stdin bytes.Buffer
	var order []int
	for i := range h.commits {
		c := &h.commits[i]
		if len(parentSHAs[i]) == 0 {
			fmt.Fprintf(&stdin, "%s\n", c.sha)
			order = append(order, i)
			c.changed = make([]map[string]bool, 1)
			continue
		}
		for _, p := range parentSHAs[i] {
			pi, ok := index[p]
			if !ok {
				return nil, fmt.Errorf("git log: parent %s of %s not in history", p, c.sha)
			}
			c.parents = append(c.parents, pi)
			fmt.Fprintf(&stdin, "%s %s\n", c.sha, p)
			order = append(order, i)
		}
		c.changed = make([]map[string]bool, len(c.parents))
	}
	// --relative scopes names to the bundle root, matching how the per-path
	// lookup resolves its relative pathspec from -C root. --always prints the
	// commit header even for an empty diff, so headers line up with stdin.
	cmd := exec.Command("git", "-C", root, "diff-tree", "--stdin", "-r", "-z", "--name-only", //nolint:gosec // G204: fixed git subcommand over the user's bundle
		"--no-renames", "--always", "--root", "--relative")
	cmd.Stdin = &stdin
	out, err = gitOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}
	tokens := strings.Split(string(out), "\x00")
	t := 0
	slot := map[int]int{} // commit index -> next parent slot to fill
	for k, ci := range order {
		if t >= len(tokens) || tokens[t] != h.commits[ci].sha {
			return nil, errors.New("git diff-tree: output out of step with input")
		}
		t++
		var next string
		if k+1 < len(order) {
			next = h.commits[order[k+1]].sha
		}
		set := map[string]bool{}
		for ; t < len(tokens) && tokens[t] != "" && tokens[t] != next; t++ {
			addWithParents(set, tokens[t])
		}
		h.commits[ci].changed[slot[ci]] = set
		slot[ci]++
	}
	return h, nil
}

// addWithParents records a changed path and each directory above it. A
// pathspec naming a directory matches every file under it, so a path that was
// once a directory still counts as touched by a change beneath it.
func addWithParents(set map[string]bool, p string) {
	set[p] = true
	for i := strings.LastIndexByte(p, '/'); i > 0; i = strings.LastIndexByte(p[:i], '/') {
		set[p[:i]] = true
	}
}

// perPathConfig reports whether git config changes what a per-path `git log
// --format=%H %cI` prints for this repository: log.follow turns on rename
// following for a single-path log, and log.showSignature prints signature
// lines between commits. Either one sends every path to the per-path lookup.
func perPathConfig(root string) bool {
	out, err := gitOutput(exec.Command("git", "-C", root, "config", "--get-regexp", `^log\.(follow|showsignature)$`)) //nolint:gosec // G204: fixed git subcommand over the user's bundle
	if err != nil {
		return false // exit 1: neither key set
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_, v, _ := strings.Cut(line, " ")
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "false", "no", "off", "0":
		default:
			return true
		}
	}
	return false
}

// pathspecMagic reports whether git would read relPath as more than a literal
// path: glob characters, a leading ':' magic signature, or an environment
// override of pathspec matching. Such a path keeps the per-path lookup.
func pathspecMagic(relPath string) bool {
	for _, env := range []string{"GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS", "GIT_LITERAL_PATHSPECS"} {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return strings.HasPrefix(relPath, ":") || strings.ContainsAny(relPath, `*?[\`)
}

// lastCommit is GitLastCommitDateIgnoring answered from the loaded history.
func (h *gitHistory) lastCommit(relPath string, ignore map[string]bool) (time.Time, string, bool, error) {
	if h.perPath || pathspecMagic(relPath) {
		return GitLastCommitDateIgnoring(h.root, relPath, ignore)
	}
	seen := make([]bool, len(h.commits))
	q := &commitQueue{h: h}
	seen[h.head] = true
	q.push(h.head)
	for q.Len() > 0 {
		ci := q.pop()
		c := &h.commits[ci]
		show, follow := h.simplify(c, relPath)
		for _, p := range follow {
			if !seen[p] {
				seen[p] = true
				q.push(p)
			}
		}
		if !show || revIgnored(c.sha, ignore) {
			continue
		}
		ts, err := time.Parse(time.RFC3339, c.iso)
		if err != nil {
			return time.Time{}, "", false, err
		}
		return ts, c.sha, true, nil
	}
	return time.Time{}, "", false, nil
}

// simplify applies git's default history simplification for one path to one
// commit: whether `git log -- <path>` shows it, and which parents the walk
// continues into.
func (h *gitHistory) simplify(c *gitCommit, relPath string) (show bool, follow []int) {
	if len(c.parents) == 0 {
		return c.changed[0][relPath], nil // root: shown iff it has the path
	}
	for i, p := range c.parents {
		if !c.changed[i][relPath] {
			// TREESAME to this parent: hide the commit and follow only it.
			return false, []int{p}
		}
	}
	return true, c.parents
}

// commitQueue pops the newest committer date first and, among equal dates, the
// commit queued first — the order git's revision walk emits.
type commitQueue struct {
	h     *gitHistory
	items []queued
	seq   int
}

type queued struct {
	ci  int
	seq int
}

func (q *commitQueue) Len() int { return len(q.items) }
func (q *commitQueue) Less(i, j int) bool {
	a, b := q.h.commits[q.items[i].ci].unix, q.h.commits[q.items[j].ci].unix
	if a != b {
		return a > b
	}
	return q.items[i].seq < q.items[j].seq
}
func (q *commitQueue) Swap(i, j int) { q.items[i], q.items[j] = q.items[j], q.items[i] }
func (q *commitQueue) Push(x any)    { q.items = append(q.items, x.(queued)) }
func (q *commitQueue) Pop() any {
	x := q.items[len(q.items)-1]
	q.items = q.items[:len(q.items)-1]
	return x
}
func (q *commitQueue) push(ci int) { heap.Push(q, queued{ci: ci, seq: q.seq}); q.seq++ }
func (q *commitQueue) pop() int    { return heap.Pop(q).(queued).ci }
