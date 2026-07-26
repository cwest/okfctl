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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginInstall_RealSearchPluginRoundTrip exercises the full contract with
// the in-repo okfctl-search plugin: build it, install it via `plugin install`,
// prove `plugin list` discovers it, and dispatch invokes the real binary. This
// is the acceptance round-trip from the task's "Done when" list. Skipped under
// -short because it shells out to `go build`.
func TestPluginInstall_RealSearchPluginRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build-based round trip in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}

	// Build the real okfctl-search plugin binary from the in-repo main.
	buildDir := t.TempDir()
	built := filepath.Join(buildDir, "okfctl-search")
	build := exec.Command("go", "build", "-o", built, "github.com/cwest/okfctl/cmd/okfctl-search")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build okfctl-search: %v\n%s", err, out)
	}

	// Install it into a fresh managed dir via the command.
	dst := t.TempDir()
	if _, _, err := runOKFSplit(t, "plugin", "install", built, "--dir", dst); err != nil {
		t.Fatalf("plugin install okfctl-search: %v", err)
	}
	installed := filepath.Join(dst, "okfctl-search")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("okfctl-search not installed: %v", err)
	}

	// plugin list discovers it.
	listOut, _, err := runOKFSplit(t, "plugin", "list", "--path", dst)
	if err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	if !strings.Contains(listOut, "okfctl-search\t") {
		t.Errorf("plugin list should show okfctl-search, got %q", listOut)
	}

	// dispatch invokes the real binary; --help is a clean, side-effect-free run.
	code, derr := dispatch("search", []string{"--help"}, dst)
	if derr != nil {
		t.Fatalf("dispatch okfctl-search --help: %v", derr)
	}
	if code != 0 {
		t.Errorf("okfctl-search --help exit code = %d, want 0", code)
	}
}
