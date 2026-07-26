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
	if _, err := exec.LookPath("git"); err != nil {
		return time.Time{}, false, nil // git unavailable: no answer, not an error
	}
	// %cI is the committer date in strict ISO-8601 (RFC3339). "-C root" runs git
	// as if from the bundle root so relPath resolves correctly.
	cmd := exec.Command("git", "-C", root, "log", "-1", "--format=%cI", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		// A non-repo directory makes git exit non-zero ("not a git repository").
		// That is "no answer", not a tool failure we should surface — degrade.
		return time.Time{}, false, nil
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		// Untracked file / path with no commits: git succeeds with empty output.
		return time.Time{}, false, nil
	}
	ts, perr := time.Parse(time.RFC3339, s)
	if perr != nil {
		return time.Time{}, false, perr
	}
	return ts, true, nil
}
