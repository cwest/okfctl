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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runOKFSplit runs the root with separate stdout/stderr buffers.
func runOKFSplit(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRootCmd()
	var o, e bytes.Buffer
	root.SetOut(&o)
	root.SetErr(&e)
	root.SetArgs(args)
	err = root.Execute()
	return o.String(), e.String(), err
}

// mkPluginStub writes an executable okfctl-<name> stub into dir.
func mkPluginStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, "okfctl-"+name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
	return p
}

func TestPluginList_PrintsDiscoveredSorted(t *testing.T) {
	dir := t.TempDir()
	pBeta := mkPluginStub(t, dir, "beta", "echo beta")
	pAlpha := mkPluginStub(t, dir, "alpha", "echo alpha")

	stdout, _, err := runOKFSplit(t, "plugin", "list", "--path", dir)
	if err != nil {
		t.Fatalf("plugin list returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), stdout)
	}
	if !strings.HasPrefix(lines[0], "okfctl-alpha\t") || !strings.Contains(lines[0], pAlpha) {
		t.Errorf("line 0 = %q, want okfctl-alpha\\t%s", lines[0], pAlpha)
	}
	if !strings.HasPrefix(lines[1], "okfctl-beta\t") || !strings.Contains(lines[1], pBeta) {
		t.Errorf("line 1 = %q, want okfctl-beta\\t%s", lines[1], pBeta)
	}
}

func TestPluginList_EmptyIsFriendly(t *testing.T) {
	empty := t.TempDir()
	stdout, stderr, err := runOKFSplit(t, "plugin", "list", "--path", empty)
	if err != nil {
		t.Fatalf("plugin list (empty) returned error: %v", err)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("want empty stdout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "no okfctl plugins") {
		t.Errorf("want friendly stderr note, got %q", stderr)
	}
}
