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
	"path/filepath"

	"github.com/cwest/okfctl/internal/clock"
	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// nowUTCcmd is the cmd-layer clock. It reads the single process-wide source
// (internal/clock), so a pinned SOURCE_DATE_EPOCH flows through here to the
// modified timestamp `node edit` writes. It stays a var so tests can pin it
// independently of the model seam; production delegates to the shared clock.
var nowUTCcmd = clock.Now

// bundleRel converts an absolute node path (as returned by okf.NewNode) into a
// bundle-relative slash path rooted at dir.
func bundleRel(dir, abs string) (string, error) {
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// maintainOnCreate records a node creation in the derived artifacts. Failures
// are reported to stderr but never fatal: the node is already written, and a
// derived-artifact hiccup must not make the create look like it failed.
func maintainOnCreate(cmd *cobra.Command, dir, rel string) {
	if err := okf.AppendLog(dir, "created "+rel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
	maintainIndex(cmd, dir)
}

// maintainOnEdit records a node edit in the derived artifacts (see
// maintainOnCreate for the best-effort contract).
func maintainOnEdit(cmd *cobra.Command, dir, rel string) {
	if err := okf.AppendLog(dir, "edited "+rel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
	maintainIndex(cmd, dir)
}

// maintainOnDelete records a node removal in the derived artifacts.
func maintainOnDelete(cmd *cobra.Command, dir, rel string) {
	if err := okf.AppendLog(dir, "removed "+rel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
	maintainIndex(cmd, dir)
}

// maintainOnMove records a node move in the derived artifacts.
func maintainOnMove(cmd *cobra.Command, dir, oldRel, newRel string) {
	if err := okf.AppendLog(dir, "moved "+oldRel+" -> "+newRel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
	maintainIndex(cmd, dir)
}

// logOnRefresh records a single node's timestamp refresh in log.md. Unlike the
// maintainOn* helpers it does NOT regenerate index.md — a bulk refresh appends
// one log line per node and regenerates the index once at the end, so the index
// is not rebuilt N times. Best-effort: reported, never fatal.
func logOnRefresh(cmd *cobra.Command, dir, rel string) {
	if err := okf.AppendLog(dir, "refreshed "+rel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
}

// logOnPromote records a single directory-concept promotion in log.md. Like
// logOnRefresh it does NOT regenerate index.md — a bulk promote appends one log
// line per node and regenerates the index once at the end. Best-effort:
// reported, never fatal.
func logOnPromote(cmd *cobra.Command, dir, oldRel, newRel string) {
	if err := okf.AppendLog(dir, "promoted "+oldRel+" -> "+newRel); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not update log.md: %v\n", err)
	}
}

// maintainIndex regenerates index.md from the current bundle so it never
// silently drifts after a node create/edit/delete/rename. A build step a human
// must remember is a build step that drifts; okfctl maintains it automatically.
// Best-effort: reported, never fatal.
func maintainIndex(cmd *cobra.Command, dir string) {
	b, err := okf.Load(dir)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not reload bundle to refresh index.md: %v\n", err)
		return
	}
	if err := okf.WriteIndex(b); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not refresh index.md: %v\n", err)
	}
}
