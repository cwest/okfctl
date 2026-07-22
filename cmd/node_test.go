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
	"strings"
	"testing"
)

func TestNodeNew_RequiresType(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "node", "new", "x.md", "--bundle", dir); err == nil {
		t.Fatal("node new without --type must error")
	}
}

func TestNodeNew_CreatesNode(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatalf("node new: %v", err)
	}
}

func TestNodeList_SurfacesType(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "node", "list", "--bundle", dir)
	if err != nil {
		t.Fatalf("node list: %v", err)
	}
	if !strings.Contains(out, "wine/tannin.md") || !strings.Contains(out, "Reference") {
		t.Errorf("node list must surface path and type; got:\n%s", out)
	}
}

func TestNodeShow_SurfacesType(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Playbook", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runOKF(t, "node", "show", "a.md", "--bundle", dir)
	if err != nil {
		t.Fatalf("node show: %v", err)
	}
	if !strings.Contains(out, "Playbook") {
		t.Errorf("node show must surface type; got:\n%s", out)
	}
}
