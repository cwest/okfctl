// Copyright 2026 Casey West
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

func TestBundleInit_CreatesValidatableBundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatalf("bundle init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err != nil {
		t.Fatalf("index.md not created: %v", err)
	}
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("freshly-init bundle failed validate: %v", err)
	}
}

func TestBundleInfo_ReportsCounts(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "bundle", "info", dir)
	if err != nil {
		t.Fatalf("bundle info: %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("bundle info should report 1 node; got:\n%s", out)
	}
}
