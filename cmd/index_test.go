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
	"testing"
)

func TestIndexBuildThenCheck_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass right after build: %v", err)
	}
}

func TestIndexCheck_StaleExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	_, _ = runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir)
	_, _ = runOKF(t, "index", "build", dir)
	_, _ = runOKF(t, "node", "new", "b.md", "--type", "Reference", "--bundle", dir)
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Fatal("index check must exit nonzero on a stale index")
	}
}
