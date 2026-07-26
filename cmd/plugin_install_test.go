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
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginInstall_RoundTrip installs a plugin via the command, then proves the
// same card of the contract: plugin list discovers it and dispatch invokes it.
func TestPluginInstall_RoundTrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// A real okfctl-greet plugin stub that prints its args.
	srcPath := filepath.Join(src, "okfctl-greet")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho \"greet:$*\"\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// install --dir <dst>
	stdout, _, err := runOKFSplit(t, "plugin", "install", srcPath, "--dir", dst)
	if err != nil {
		t.Fatalf("plugin install returned error: %v", err)
	}
	installed := filepath.Join(dst, "okfctl-greet")
	if !strings.Contains(stdout, installed) {
		t.Errorf("install stdout should name the installed path %q, got %q", installed, stdout)
	}
	if fi, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("installed plugin missing: %v", statErr)
	} else if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed plugin not executable: %v", fi.Mode())
	}

	// plugin list --path <dst> now shows it.
	listOut, _, err := runOKFSplit(t, "plugin", "list", "--path", dst)
	if err != nil {
		t.Fatalf("plugin list returned error: %v", err)
	}
	if !strings.Contains(listOut, "okfctl-greet\t") || !strings.Contains(listOut, installed) {
		t.Errorf("plugin list should show installed plugin, got %q", listOut)
	}

	// dispatch executes it with args passed through.
	code, derr := dispatch("greet", []string{"world"}, dst)
	if derr != nil {
		t.Fatalf("dispatch error: %v", derr)
	}
	if code != 0 {
		t.Errorf("dispatch exit code = %d, want 0", code)
	}
}

func TestPluginInstall_DefaultsToInstallDir(t *testing.T) {
	// With OKFCTL_CONFIG_HOME set and no --dir, install lands under the managed
	// plugins dir and reports that path.
	home := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", home)

	src := t.TempDir()
	srcPath := filepath.Join(src, "okfctl-demo")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	stdout, _, err := runOKFSplit(t, "plugin", "install", srcPath)
	if err != nil {
		t.Fatalf("plugin install returned error: %v", err)
	}
	want := filepath.Join(home, "plugins", "okfctl-demo")
	if !strings.Contains(stdout, want) {
		t.Errorf("install stdout should name default dir path %q, got %q", want, stdout)
	}
	if _, statErr := os.Stat(want); statErr != nil {
		t.Fatalf("expected plugin installed at %q: %v", want, statErr)
	}
}

func TestPluginInstall_RejectsBadSource(t *testing.T) {
	src := t.TempDir()
	srcPath := filepath.Join(src, "notaplugin")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if _, _, err := runOKFSplit(t, "plugin", "install", srcPath, "--dir", t.TempDir()); err == nil {
		t.Errorf("plugin install should error on a source not named okfctl-<name>")
	}
}
