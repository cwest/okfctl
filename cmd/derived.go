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
	"time"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// nowUTCcmd is the cmd-layer clock (real UTC wall time). It exists as a var so
// timestamp-dependent command behavior stays consistent with the model's clock
// seam; production reads real time.
var nowUTCcmd = func() time.Time { return time.Now().UTC() }

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
