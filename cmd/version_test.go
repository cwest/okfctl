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
	"strings"
	"testing"
)

// runVersion executes `okfctl version` against a fresh root command and returns
// its stdout.
func runVersion(t *testing.T) string {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	return out.String()
}

func TestVersion_DefaultsToDev(t *testing.T) {
	// A plain `go build` injects nothing, so the reported version degrades
	// gracefully to the "dev" default.
	defer restoreVersionInfo(SetVersionInfo("", "", ""))
	got := runVersion(t)
	if !strings.Contains(got, "dev") {
		t.Fatalf("version output = %q, want it to contain %q", got, "dev")
	}
}

func TestVersion_ReportsInjectedValue(t *testing.T) {
	// A release build injects the tag via ldflags; the command must report it.
	defer restoreVersionInfo(SetVersionInfo("v1.2.3", "abc1234", "2026-07-26T00:00:00Z"))
	got := runVersion(t)
	if !strings.Contains(got, "v1.2.3") {
		t.Fatalf("version output = %q, want it to contain %q", got, "v1.2.3")
	}
	if !strings.Contains(got, "abc1234") {
		t.Fatalf("version output = %q, want it to contain commit %q", got, "abc1234")
	}
	if !strings.Contains(got, "2026-07-26T00:00:00Z") {
		t.Fatalf("version output = %q, want it to contain date", got)
	}
}

func TestVersion_FlagReportsSameValue(t *testing.T) {
	// `okfctl --version` reports the same injected version string as the
	// subcommand, wired to the identical source.
	defer restoreVersionInfo(SetVersionInfo("v9.9.9", "deadbee", "2026-01-01T00:00:00Z"))
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if !strings.Contains(out.String(), "v9.9.9") {
		t.Fatalf("--version output = %q, want it to contain %q", out.String(), "v9.9.9")
	}
}

// restoreVersionInfo re-applies the version info captured before a test mutated
// it, keeping package state clean across tests.
func restoreVersionInfo(prev [3]string) {
	SetVersionInfo(prev[0], prev[1], prev[2])
}
