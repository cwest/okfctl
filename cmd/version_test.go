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
	"runtime/debug"
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

// buildInfoOf builds a minimal *debug.BuildInfo whose Main.Version is v, for
// exercising the go-install fallback without depending on the test binary's
// own module metadata.
func buildInfoOf(v string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: v}}
}

func TestResolveVersion_PrefersInjectedLdflags(t *testing.T) {
	// A goreleaser build injects a real tag; that always wins over build info,
	// even when module metadata is also present.
	got := resolveVersion("v1.2.3", func() (*debug.BuildInfo, bool) {
		return buildInfoOf("v9.9.9"), true
	})
	if got != "v1.2.3" {
		t.Fatalf("resolveVersion = %q, want the injected ldflags value %q", got, "v1.2.3")
	}
}

func TestResolveVersion_FallsBackToModuleVersionOnGoInstall(t *testing.T) {
	// A `go install github.com/cwest/okfctl@latest` build injects nothing, so
	// the ldflags value is the "dev" default; the module version from
	// debug.ReadBuildInfo() must fill in instead of reporting "dev".
	got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
		return buildInfoOf("v0.2.0"), true
	})
	if got != "v0.2.0" {
		t.Fatalf("resolveVersion = %q, want the module version %q", got, "v0.2.0")
	}
}

func TestResolveVersion_KeepsDevWhenModuleVersionUnusable(t *testing.T) {
	// When the Go toolchain cannot derive a real module version it records
	// "(devel)" (or leaves it empty) for the main module — e.g. a build with no
	// usable VCS/module metadata. That is not a real release, so the "dev"
	// default is kept rather than surfacing the useless placeholder. (A build
	// inside a git checkout instead gets a real pseudo-version, which is a
	// usable value the fallback happily reports.)
	for _, mv := range []string{"", "(devel)"} {
		got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
			return buildInfoOf(mv), true
		})
		if got != "dev" {
			t.Fatalf("resolveVersion with module version %q = %q, want %q", mv, got, "dev")
		}
	}
}

func TestResolveVersion_KeepsDevWhenBuildInfoUnavailable(t *testing.T) {
	// debug.ReadBuildInfo() can fail (ok=false); the "dev" default stands.
	got := resolveVersion("dev", func() (*debug.BuildInfo, bool) {
		return nil, false
	})
	if got != "dev" {
		t.Fatalf("resolveVersion = %q, want %q when build info is unavailable", got, "dev")
	}
}
