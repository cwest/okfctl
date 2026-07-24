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

// mkExecStub writes an executable okfctl-<name> stub with an explicit script body.
func mkExecStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, "okfctl-"+name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}
	return p
}

func TestDispatch_ExecsPluginWithArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "out.txt")
	// Stub records its args and whether OKFCTL is set, into marker.
	mkExecStub(t, dir, "demo", `printf '%s\n' "$*" > "`+marker+`"; printf 'OKFCTL=%s\n' "${OKFCTL:-UNSET}" >> "`+marker+`"`)

	code, err := dispatch("demo", []string{"hello", "--flag"}, dir)
	if err != nil {
		t.Fatalf("dispatch returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("dispatch exit = %d, want 0", code)
	}
	data, rerr := os.ReadFile(marker)
	if rerr != nil {
		t.Fatalf("plugin did not write marker: %v", rerr)
	}
	got := string(data)
	if !strings.Contains(got, "hello --flag") {
		t.Errorf("plugin args = %q, want to contain 'hello --flag'", got)
	}
	if strings.Contains(got, "OKFCTL=UNSET") || !strings.Contains(got, "OKFCTL=") {
		t.Errorf("plugin should see OKFCTL set; marker=%q", got)
	}
}

func TestDispatch_ExitCodeFidelity(t *testing.T) {
	dir := t.TempDir()
	mkExecStub(t, dir, "boom", "exit 7")

	code, err := dispatch("boom", nil, dir)
	if err != nil {
		t.Fatalf("dispatch should not error on nonzero child exit: %v", err)
	}
	if code != 7 {
		t.Errorf("dispatch exit = %d, want 7 (child fidelity)", code)
	}
}

func TestDispatch_MissingPlugin(t *testing.T) {
	dir := t.TempDir()
	code, err := dispatch("ghost", nil, dir)
	if err == nil {
		t.Fatalf("dispatch of missing plugin should error")
	}
	if code == 0 {
		t.Errorf("missing plugin exit = %d, want non-zero", code)
	}
}

func TestExecute_UnknownNoPluginSuggests(t *testing.T) {
	// No plugin on an empty PATH; an unknown near-miss command should surface a
	// did-you-mean suggestion for the closest built-in.
	stdout, stderr, err := runOKFSplit(t, "valdate", "somewhere")
	_ = stdout
	if err == nil {
		t.Fatalf("unknown command with no plugin should error")
	}
	combined := stderr + err.Error()
	if !strings.Contains(strings.ToLower(combined), "valdate") {
		t.Errorf("error should name the unknown command; got %q", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "validate") {
		t.Errorf("error should suggest 'validate' (did-you-mean); got %q", combined)
	}
}

func TestExecute_BuiltinNotShadowed(t *testing.T) {
	// Even with an okfctl-validate on PATH, the built-in `validate` must run.
	dir := t.TempDir()
	mkExecStub(t, dir, "validate", "echo PLUGIN_RAN; exit 3")
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)

	good := filepath.Join("..", "testdata", "good-bundle")
	out, err := runOKF(t, "validate", good)
	if err != nil {
		t.Fatalf("built-in validate should run and pass, got err: %v (out=%q)", err, out)
	}
	if strings.Contains(out, "PLUGIN_RAN") {
		t.Errorf("plugin shadowed the built-in; out=%q", out)
	}
}
