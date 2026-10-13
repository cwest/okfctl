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
	"runtime"
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

// IsCheckoutOf reports whether dir is itself the top level of a git work tree
// whose origin remote is url. Being inside a work tree is not enough: a
// subdirectory of an unrelated repo, or a checkout of a different origin, is
// not a checkout of url, and fast-forwarding it would pull the wrong repo. It
// degrades to false (never an error) when git is absent or either probe fails.
func IsCheckoutOf(dir, url string) bool {
	if !GitAvailable() {
		return false
	}
	top, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(top, dir) {
		return false
	}
	// The stored origin URL, not `git remote get-url`: get-url applies
	// url.<base>.insteadOf rewrites, so it would never equal the URL the
	// checkout was cloned from when such a rewrite is configured.
	origin, err := gitOutput(dir, "config", "--get", "remote.origin.url")
	if err != nil || origin == "" {
		return false
	}
	// git stores a relative local origin relative to the work tree.
	return NormalizeSourceURL(origin, top) == NormalizeSourceURL(url, "")
}

// gitOutput runs a fixed git subcommand in dir and returns its trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec // G204: fixed git subcommands over the user's own checkout
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// HasWindowsPathPrefix reports whether s starts like a Windows path git accepts
// as a local source: a drive letter (`C:\repos\kb`, `C:/repos/kb`) or a UNC
// share (`\\server\share\kb`).
func HasWindowsPathPrefix(s string) bool {
	if strings.HasPrefix(s, `\\`) {
		return true
	}
	return len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') &&
		(('a' <= s[0] && s[0] <= 'z') || ('A' <= s[0] && s[0] <= 'Z'))
}

// IsLocalSource reports whether a git source string names a local path rather
// than a remote URL, following git's own rule: a scheme (`://`) is a URL; a
// Windows drive or UNC prefix is a path; otherwise a colon with no slash before
// it is scp-like syntax (`host:path`, `git@host:path`).
func IsLocalSource(s string) bool {
	if strings.Contains(s, "://") {
		return strings.HasPrefix(strings.ToLower(s), "file://")
	}
	if HasWindowsPathPrefix(s) {
		return true
	}
	colon := strings.Index(s, ":")
	if colon < 0 {
		return true
	}
	slash := strings.IndexAny(s, `/\`)
	return slash >= 0 && slash < colon
}

// NormalizeSourceURL reduces a git source to a comparable form, so the URL a
// checkout was cloned from matches the one connect is given even when spelled
// differently. A local path (or file:// URL) becomes an absolute, cleaned,
// symlink-resolved path, resolved against base when relative (the cwd when
// base is empty), and case-folded on Windows. A remote URL has its scheme and
// host lower-cased and a trailing "/" or ".git" dropped.
func NormalizeSourceURL(s, base string) string {
	s = strings.TrimSpace(s)
	if IsLocalSource(s) {
		if len(s) >= len("file://") && strings.EqualFold(s[:len("file://")], "file://") {
			s = s[len("file://"):]
			if len(s) >= 3 && s[0] == '/' && HasWindowsPathPrefix(s[1:]) {
				s = s[1:] // file:///C:/repos/kb
			}
		}
		if base != "" && !filepath.IsAbs(s) && !HasWindowsPathPrefix(s) {
			s = filepath.Join(base, s)
		}
		return canonicalPath(s)
	}
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		// scheme://[user@]host[:port]/path
		rest := s[i+3:]
		hostEnd := strings.Index(rest, "/")
		if hostEnd < 0 {
			hostEnd = len(rest)
		}
		host := rest[:hostEnd]
		user := ""
		if at := strings.LastIndex(host, "@"); at >= 0 {
			user, host = host[:at+1], host[at+1:]
		}
		return strings.ToLower(s[:i]) + "://" + user + strings.ToLower(host) + rest[hostEnd:]
	}
	// scp-like: [user@]host:path
	colon := strings.Index(s, ":")
	host := s[:colon]
	user := ""
	if at := strings.LastIndex(host, "@"); at >= 0 {
		user, host = host[:at+1], host[at+1:]
	}
	return user + strings.ToLower(host) + s[colon:]
}

// samePath reports whether two filesystem paths name the same location.
func samePath(a, b string) bool {
	return canonicalPath(a) == canonicalPath(b)
}

// canonicalPath returns an absolute, cleaned path with symlinks (and, on
// Windows, 8.3 short names) resolved where the path exists, case-folded on
// Windows where the filesystem is case-insensitive.
func canonicalPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}
