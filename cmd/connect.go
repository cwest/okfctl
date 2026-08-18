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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// newConnectCmd builds `okfctl connect`, which materializes a remote bundle
// source into a local directory over plain git: `git clone` on first use, a
// fast-forward `git pull` on subsequent use. The source is a registered remote
// name (see `okfctl registry`) or an ad-hoc git URL.
func newConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <name|git-url> [dir]",
		Short: "Clone or update a remote bundle source into a local directory",
		Long: "Materialize a remote OKF bundle source into a local directory over git.\n" +
			"The first argument is a remote name registered with `okfctl registry`, " +
			"or an ad-hoc git URL. The optional second argument is the destination " +
			"directory (default: a directory named after the source).\n\n" +
			"A fresh destination is cloned; an existing checkout of the same source " +
			"is fast-forwarded (never a history-rewriting merge). A non-empty " +
			"directory that isn't that git checkout is left untouched.",
		Example: "  # Clone a registered remote into a directory named after the source\n" +
			"  okfctl connect knowledge\n\n" +
			"  # Clone an ad-hoc git URL into a chosen directory\n" +
			"  okfctl connect https://github.com/acme/kb.git ./kb\n\n" +
			"  # Re-run to fast-forward an existing checkout\n" +
			"  okfctl connect knowledge ./kb",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]

			// Resolve the source to a git URL: a registered name wins; otherwise
			// the argument must look like a git URL (a bare unknown word is a
			// mistake, not an ad-hoc URL, and cloning it would produce a cryptic
			// git error).
			url, registered, err := resolveRemoteURL(source)
			if err != nil {
				return err
			}
			if !registered {
				if !looksLikeGitURL(source) {
					return fmt.Errorf("no such remote %q, and it is not a git URL; register it with `okfctl registry add %s <git-url>` or pass a URL", source, source)
				}
				url = source
			}

			dir := ""
			if len(args) == 2 {
				dir = args[1]
			} else {
				dir = defaultDirForURL(url)
			}
			if dir == "" {
				return fmt.Errorf("could not derive a destination directory from %q; pass one explicitly", url)
			}

			out := cmd.OutOrStdout()

			// Existing checkout of the source: fast-forward it.
			if okf.IsGitWorkTree(dir) {
				if err := okf.PullFastForward(dir); err != nil {
					return err
				}
				fmt.Fprintf(out, "Updated %s from %s\n", dir, url)
				return nil
			}

			// A non-empty, non-repo directory must not be clobbered.
			if nonEmpty, err := dirIsNonEmpty(dir); err != nil {
				return err
			} else if nonEmpty {
				return fmt.Errorf("destination %s exists, is not empty, and is not a checkout of this source; refusing to overwrite", dir)
			}

			if err := okf.Clone(url, dir); err != nil {
				return err
			}
			fmt.Fprintf(out, "Connected %s -> %s\n", url, dir)
			return nil
		},
	}
}

// looksLikeGitURL reports whether s is plausibly a git URL rather than a bare
// (mistyped) remote name: a scheme (https://, git://, ssh://, file://), scp-like
// syntax (user@host:path), or an absolute/relative path to a local repo.
func looksLikeGitURL(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	if strings.Contains(s, "@") && strings.Contains(s, ":") {
		return true // scp-like: git@github.com:owner/repo.git
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~") {
		return true // local path (a local bare repo is a valid source)
	}
	return false
}

// defaultDirForURL derives a destination directory name from a git URL's final
// path segment, dropping a trailing .git — matching `git clone`'s own default.
func defaultDirForURL(url string) string {
	s := strings.TrimRight(url, "/")
	// scp-like syntax: everything after the last ':' is the path.
	if i := strings.LastIndex(s, ":"); i >= 0 && !strings.Contains(s, "://") {
		s = s[i+1:]
	}
	s = strings.TrimRight(s, "/")
	base := filepath.Base(s)
	base = strings.TrimSuffix(base, ".git")
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}

// dirIsNonEmpty reports whether dir exists and contains at least one entry. A
// missing directory is not an error (it is a valid clone target).
func dirIsNonEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}
