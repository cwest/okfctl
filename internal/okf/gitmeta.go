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
	"os/exec"
	"strings"
	"time"
)

// GitLastCommitDate returns the committer date of the most recent commit that
// touched the bundle-relative file relPath, resolving git from within root.
//
// The three return values distinguish "no answer" from "error": ok=false with a
// nil error means git could not answer (git binary absent, root is not a git
// repo, or the file is untracked / has no commits) — a normal, non-fatal state
// that callers degrade on. A non-nil error is a genuine failure (git present and
// invoked but failing for an unexpected reason). This keeps the drift check
// working the same way as `index check`: git is a source of truth when present,
// and silently unavailable when not, never a crash.
func GitLastCommitDate(root, relPath string) (time.Time, bool, error) {
	ts, _, ok, err := GitLastCommitDateIgnoring(root, relPath, nil)
	return ts, ok, err
}

// GitLastCommitDateIgnoring returns the committer date AND SHA of the most
// recent commit that touched relPath whose SHA is NOT in the ignore set,
// resolving git from within root. It is the drift-comparison primitive that lets
// a bulk mechanical commit opt out: when the file's last-touching commit is
// listed in `.okf-drift-ignore-revs` (loaded via LoadDriftIgnoreRevs), the
// comparison walks back to the prior real commit instead of collapsing the
// node's authoring history into the migration date. This mirrors the established
// `git blame --ignore-revs-file` convention.
//
// An ignore entry matches a commit when it equals the full 40-char SHA or is a
// prefix of it (>= 7 chars), so an abbreviated SHA in the file still opts the
// commit out — the same spelling tolerance git blame gives.
//
// The four return values distinguish "no answer" from "error": ok=false with a
// nil error means git could not answer (git binary absent, root is not a git
// repo, the file is untracked, or EVERY commit that touched it is ignored) — all
// normal, non-fatal states callers degrade on. A non-nil error is a genuine
// failure. When ignore is nil or empty this behaves exactly like a `git log -1`.
func GitLastCommitDateIgnoring(root, relPath string, ignore map[string]bool) (time.Time, string, bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return time.Time{}, "", false, nil // git unavailable: no answer, not an error
	}
	// %H is the full commit SHA, %cI the committer date in strict ISO-8601
	// (RFC3339). "-C root" runs git as if from the bundle root so relPath
	// resolves correctly. Without -1 we get the full history for the path,
	// newest first, so we can walk back past ignored (mechanical) commits.
	cmd := exec.Command("git", "-C", root, "log", "--format=%H %cI", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		// A non-repo directory makes git exit non-zero ("not a git repository").
		// That is "no answer", not a tool failure we should surface — degrade.
		return time.Time{}, "", false, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			// Untracked file / path with no commits: git succeeds with no output.
			continue
		}
		sha, iso, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		if revIgnored(sha, ignore) {
			continue // mechanical commit: walk back to the prior real one
		}
		ts, perr := time.Parse(time.RFC3339, strings.TrimSpace(iso))
		if perr != nil {
			return time.Time{}, "", false, perr
		}
		return ts, sha, true, nil
	}
	// No commits, or every touching commit was ignored: no source of truth.
	return time.Time{}, "", false, nil
}

// revIgnored reports whether a commit's full SHA is opted out by the ignore set.
// An entry matches when it equals the SHA or is a >=7-char prefix of it, so both
// full and abbreviated spellings in `.okf-drift-ignore-revs` opt the commit out.
func revIgnored(fullSHA string, ignore map[string]bool) bool {
	if len(ignore) == 0 {
		return false
	}
	if ignore[fullSHA] {
		return true
	}
	for entry := range ignore {
		if len(entry) >= 7 && len(entry) < len(fullSHA) && strings.HasPrefix(fullSHA, entry) {
			return true
		}
	}
	return false
}
