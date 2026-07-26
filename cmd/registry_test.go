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
	"strings"
	"testing"
)

func TestRegistry_AddListShowRemove_RoundTrips(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())

	if _, err := runOKF(t, "registry", "add", "kb", "https://example.com/kb.git"); err != nil {
		t.Fatalf("registry add kb: %v", err)
	}
	if _, err := runOKF(t, "registry", "add", "office", "git@github.com:cwest/office.git"); err != nil {
		t.Fatalf("registry add office: %v", err)
	}

	out, err := runOKF(t, "registry", "list")
	if err != nil {
		t.Fatalf("registry list: %v", err)
	}
	if !strings.Contains(out, "kb") || !strings.Contains(out, "https://example.com/kb.git") {
		t.Fatalf("list missing kb entry:\n%s", out)
	}
	if !strings.Contains(out, "office") {
		t.Fatalf("list missing office entry:\n%s", out)
	}
	// sorted: kb before office
	if strings.Index(out, "kb") > strings.Index(out, "office") {
		t.Fatalf("list not sorted by name:\n%s", out)
	}

	show, err := runOKF(t, "registry", "show", "kb")
	if err != nil {
		t.Fatalf("registry show: %v", err)
	}
	if !strings.Contains(show, "https://example.com/kb.git") {
		t.Fatalf("show kb = %q, want url", show)
	}

	if _, err := runOKF(t, "registry", "remove", "kb"); err != nil {
		t.Fatalf("registry remove: %v", err)
	}
	if _, err := runOKF(t, "registry", "show", "kb"); err == nil {
		t.Fatalf("show of removed remote must error")
	}
}

func TestRegistry_AddIsIdempotentRepoint(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	if _, err := runOKF(t, "registry", "add", "kb", "https://old.example/kb.git"); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "registry", "add", "kb", "https://new.example/kb.git"); err != nil {
		t.Fatal(err)
	}
	show, err := runOKF(t, "registry", "show", "kb")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(show, "new.example") {
		t.Fatalf("re-add did not repoint: %q", show)
	}
}

func TestRegistry_ListEmpty(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	out, err := runOKF(t, "registry", "list")
	if err != nil {
		t.Fatalf("registry list (empty): %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("empty registry should print a friendly message, got empty output")
	}
}

func TestRegistry_RejectsBadName(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	for _, bad := range []string{"has space", "has/slash", "", "a:b"} {
		if _, err := runOKF(t, "registry", "add", bad, "https://example.com/x.git"); err == nil {
			t.Fatalf("registry add with bad name %q must error", bad)
		}
	}
}

func TestRegistry_ShowUnknownErrors(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir())
	if _, err := runOKF(t, "registry", "show", "nope"); err == nil {
		t.Fatalf("show of unknown remote must error")
	}
	if _, err := runOKF(t, "registry", "remove", "nope"); err == nil {
		t.Fatalf("remove of unknown remote must error")
	}
}
