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
	"fmt"
	"os/exec"
	"strings"
)

// gitsource.go wires okfctl to remote bundle sources over plain git. It shells
// out to the git binary (like gitmeta.go) rather than taking a git library
// dependency, keeping the core stdlib-only. It performs no authentication of
// its own: reaching a private URL is git's concern (ssh agent, credential
// helper), exactly as with any git remote.

// GitAvailable reports whether the git binary is on PATH.
func GitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Clone runs `git clone <url> <dir>`, materializing a remote bundle source into
// a local directory. It returns an error that includes git's own stderr so a
// failure (bad url, unreachable host, auth) is actionable.
func Clone(url, dir string) error {
	if !GitAvailable() {
		return fmt.Errorf("git is required to connect a remote bundle source but was not found on PATH")
	}
	// Fixed git subcommand; url is passed after "--" so it cannot be an option,
	// and cloning the user-named remote is the connect command's purpose.
	cmd := exec.Command("git", "clone", "--", url, dir) //nolint:gosec // G204: fixed git subcommand, url guarded by "--"
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s: %w\n%s", url, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PullFastForward runs `git -C <dir> pull --ff-only`, updating an existing
// checkout without ever rewriting local history. A divergence (local commits
// that are not on the remote) fails rather than merging.
func PullFastForward(dir string) error {
	if !GitAvailable() {
		return fmt.Errorf("git is required to update a remote bundle source but was not found on PATH")
	}
	// Fixed git subcommand over the user's own checkout dir.
	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only") //nolint:gosec // G204: fixed git subcommand over the user's checkout
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull --ff-only in %s: %w\n%s", dir, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsGitWorkTree reports whether dir is inside a git work tree. It degrades to
// false (never an error) when git is absent or dir is not a repo, mirroring the
// "git as an optional source of truth" discipline in gitmeta.go.
func IsGitWorkTree(dir string) bool {
	if !GitAvailable() {
		return false
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree") //nolint:gosec // G204: fixed git subcommand over the user's own checkout
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
