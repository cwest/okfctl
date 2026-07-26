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
	"path/filepath"
	"strings"
	"testing"
)

// node rm keeps index.md in sync and records the removal in log.md.
func TestNodeRm_MaintainsIndexAndLog(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "b.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "rm", "b.md", "--bundle", dir); err != nil {
		t.Fatalf("node rm: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass after node rm without a manual build: %v", err)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, "b.md") || !strings.Contains(log, "removed") {
		t.Fatalf("log.md should record the removal; got:\n%s", log)
	}
}

// A dry-run rm touches nothing: no index rebuild, no log entry.
func TestNodeRm_DryRunDoesNotMaintain(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "rm", "a.md", "--bundle", dir, "--dry-run"); err != nil {
		t.Fatalf("dry-run rm: %v", err)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if strings.Contains(log, "removed a.md") {
		t.Fatalf("dry-run must not append a log entry; got:\n%s", log)
	}
}

// node mv keeps index.md in sync and records the move in log.md.
func TestNodeMv_MaintainsIndexAndLog(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/foo.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "mv", "wine/foo.md", "cellar/foo.md", "--bundle", dir); err != nil {
		t.Fatalf("node mv: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass after node mv without a manual build: %v", err)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if !strings.Contains(log, "wine/foo.md") || !strings.Contains(log, "cellar/foo.md") || !strings.Contains(log, "moved") {
		t.Fatalf("log.md should record the move; got:\n%s", log)
	}
}

// A dry-run mv touches nothing.
func TestNodeMv_DryRunDoesNotMaintain(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/foo.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "mv", "wine/foo.md", "cellar/foo.md", "--bundle", dir, "--dry-run"); err != nil {
		t.Fatalf("dry-run mv: %v", err)
	}
	log := readFileStr(t, filepath.Join(dir, "log.md"))
	if strings.Contains(log, "moved") {
		t.Fatalf("dry-run must not append a log entry; got:\n%s", log)
	}
}
