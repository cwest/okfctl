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

package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallDir_HonorsConfigHomeOverride(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", "/tmp/okfctl-home")
	got := InstallDir()
	want := filepath.Join("/tmp/okfctl-home", "plugins")
	if got != want {
		t.Errorf("InstallDir() = %q, want %q", got, want)
	}
}

func TestInstallDir_DefaultsUnderUserConfigDir(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", "")
	h, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("no user config dir on this platform: %v", err)
	}
	want := filepath.Join(h, "okfctl", "plugins")
	if got := InstallDir(); got != want {
		t.Errorf("InstallDir() = %q, want %q", got, want)
	}
}

func TestInstall_CopiesExecutableAndIsDiscoverable(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// A source binary named okfctl-demo.
	srcPath := filepath.Join(src, "okfctl-demo")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	p, err := Install(srcPath, dst)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if p.Name != "demo" {
		t.Errorf("installed name = %q, want demo", p.Name)
	}
	wantPath := filepath.Join(dst, "okfctl-demo")
	if p.Path != wantPath {
		t.Errorf("installed path = %q, want %q", p.Path, wantPath)
	}
	// The copy exists, is executable, and has the source's bytes.
	if !isExecutable(wantPath) {
		t.Errorf("installed file is not executable: %s", wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	if string(data) != "#!/bin/sh\necho hi\n" {
		t.Errorf("installed bytes = %q, want source bytes", string(data))
	}
	// Discover on the install dir now finds it.
	if _, ok := Lookup("demo", dst); !ok {
		t.Errorf("Lookup(demo) after Install should succeed")
	}
}

func TestInstall_CreatesDestDir(t *testing.T) {
	src := t.TempDir()
	srcPath := filepath.Join(src, "okfctl-demo")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "nested", "plugins")

	if _, err := Install(srcPath, dst); err != nil {
		t.Fatalf("Install should create dest dir: %v", err)
	}
	if !isExecutable(filepath.Join(dst, "okfctl-demo")) {
		t.Errorf("expected installed executable under created dir %s", dst)
	}
}

func TestInstall_RejectsSourceWithoutPrefix(t *testing.T) {
	src := t.TempDir()
	srcPath := filepath.Join(src, "notaplugin")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if _, err := Install(srcPath, t.TempDir()); err == nil {
		t.Errorf("Install should reject a source not named okfctl-<name>")
	}
}

func TestInstall_RejectsMissingSource(t *testing.T) {
	if _, err := Install(filepath.Join(t.TempDir(), "okfctl-ghost"), t.TempDir()); err == nil {
		t.Errorf("Install should reject a nonexistent source")
	}
}

func TestInstall_RejectsNonRegularSource(t *testing.T) {
	// A directory named okfctl-demo is not a valid plugin source.
	src := t.TempDir()
	dirSrc := filepath.Join(src, "okfctl-demo")
	if err := os.Mkdir(dirSrc, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if _, err := Install(dirSrc, t.TempDir()); err == nil {
		t.Errorf("Install should reject a non-regular source")
	}
}
