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
	"path/filepath"
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

// IsCheckoutOf reports whether dir is itself the top level of a git checkout
// whose origin remote is sourceURL. Both conditions must hold:
//
//  1. `git -C dir rev-parse --show-toplevel` resolves to dir itself (compared as
//     cleaned, symlink-resolved absolute paths). A mere SUBDIRECTORY of a repo
//     is inside a work tree but is NOT the top level, so it does not qualify —
//     this is what keeps `connect` from fast-forwarding an enclosing repo.
//  2. The checkout's `origin` remote URL matches sourceURL (see sameGitURL),
//     so a checkout of a DIFFERENT source is not mistaken for this one.
//
// It degrades to (false, nil) — never an error — when git is absent, dir is not
// a repo, or there is no origin remote, mirroring the "git as an optional source
// of truth" discipline elsewhere in this package. A genuine failure to resolve
// an existing repo's metadata is returned as an error so the caller can surface
// it rather than silently clobbering.
func IsCheckoutOf(dir, sourceURL string) (bool, error) {
	if !GitAvailable() {
		return false, nil
	}
	top, ok := gitTopLevel(dir)
	if !ok {
		return false, nil // not a work tree at all
	}
	same, err := samePath(top, dir)
	if err != nil {
		return false, err
	}
	if !same {
		return false, nil // a subdirectory of a repo, not its top level
	}
	origin, ok := gitOriginURL(dir)
	if !ok {
		return false, nil // a repo with no origin remote is not a checkout of this source
	}
	return sameGitURL(origin, sourceURL), nil
}

// gitTopLevel returns the absolute path of the work-tree root containing dir, or
// ok=false when dir is not inside a git work tree.
func gitTopLevel(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel") //nolint:gosec // G204: fixed git subcommand over the user's own checkout
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		return "", false
	}
	return top, true
}

// gitOriginURL returns the fetch URL of the "origin" remote in dir, or ok=false
// when there is no origin remote.
func gitOriginURL(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin") //nolint:gosec // G204: fixed git subcommand over the user's own checkout
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", false
	}
	return url, true
}

// samePath reports whether a and b denote the same directory, comparing cleaned,
// symlink-resolved absolute paths. Symlink resolution matters because
// `rev-parse --show-toplevel` may return a path with symlinks resolved (e.g.
// macOS's /var -> /private/var) while the destination argument does not.
func samePath(a, b string) (bool, error) {
	ra, err := resolvePath(a)
	if err != nil {
		return false, err
	}
	rb, err := resolvePath(b)
	if err != nil {
		return false, err
	}
	return ra == rb, nil
}

// resolvePath returns the cleaned absolute path of p with symlinks resolved. If
// the path cannot be resolved via EvalSymlinks (e.g. it does not fully exist),
// it falls back to the cleaned absolute path so the comparison still proceeds.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return filepath.Clean(abs), nil
}

// sameGitURL reports whether two git URLs refer to the same remote. For remote
// URLs (a scheme or scp-like syntax) it normalizes a trailing ".git" and a
// trailing "/" that git treats as equivalent. For local filesystem paths — a
// local bare repo is a valid source — it compares cleaned, symlink-resolved
// absolute paths, since git may record origin as an absolute path even when the
// source was given relatively. It does not attempt deep URL canonicalization
// (host case, auth); it handles the common differences between how a source is
// registered and how git records it as origin.
func sameGitURL(a, b string) bool {
	// Local-path sources compare as cleaned, symlink-resolved absolute paths.
	if isLocalPath(a) && isLocalPath(b) {
		ra, errA := resolvePath(stripGitSuffix(a))
		rb, errB := resolvePath(stripGitSuffix(b))
		if errA == nil && errB == nil {
			return ra == rb
		}
		// Fall through to string normalization if resolution failed.
	}
	return normalizeGitURL(a) == normalizeGitURL(b)
}

// isLocalPath reports whether s looks like a local filesystem path rather than a
// remote URL: no URL scheme and no scp-like host:path syntax.
func isLocalPath(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	if strings.Contains(s, "@") && strings.Contains(s, ":") {
		return false // scp-like: git@host:owner/repo
	}
	return true
}

// stripGitSuffix removes a trailing "/" then a trailing ".git" from a path so a
// local bare repo "/srv/kb.git" and a checkout of it compare equal.
func stripGitSuffix(p string) string {
	p = strings.TrimRight(p, "/")
	return strings.TrimSuffix(p, ".git")
}

// normalizeGitURL strips a trailing "/" then a trailing ".git" so that
// "https://h/o/r", "https://h/o/r/" and "https://h/o/r.git" compare equal.
func normalizeGitURL(u string) string {
	u = strings.TrimSpace(u)
	return stripGitSuffix(u)
}
