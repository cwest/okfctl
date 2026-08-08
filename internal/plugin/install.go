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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstallDir returns the okfctl-managed plugins directory, the default
// destination for `plugin install`. It mirrors the okfconfig home resolution so
// there is a single config-home convention: $OKFCTL_CONFIG_HOME/plugins if set,
// else <user config dir>/okfctl/plugins, else ./.okfctl/plugins. This directory
// must be on PATH for Discover/Lookup (and thus `plugin list` and dispatch) to
// find installed plugins.
func InstallDir() string {
	home := os.Getenv("OKFCTL_CONFIG_HOME")
	if home == "" {
		if h, err := os.UserConfigDir(); err == nil {
			home = filepath.Join(h, "okfctl")
		} else {
			home = ".okfctl"
		}
	}
	return filepath.Join(home, "plugins")
}

// Install copies the plugin executable at source into destDir, creating destDir
// if needed, and returns the installed Plugin. source must be an existing
// regular file whose base name follows the okfctl-<name> convention; the copy
// keeps that name and is written with execute bits so Discover finds it. An
// existing plugin of the same name in destDir is overwritten.
func Install(source, destDir string) (Plugin, error) {
	base := filepath.Base(source)
	if !strings.HasPrefix(base, prefix) || strings.TrimPrefix(base, prefix) == "" {
		return Plugin{}, fmt.Errorf("plugin source %q must be named %s<name>", base, prefix)
	}
	name := strings.TrimPrefix(base, prefix)

	info, err := os.Stat(source) // Stat follows symlinks
	if err != nil {
		return Plugin{}, fmt.Errorf("plugin source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Plugin{}, fmt.Errorf("plugin source %q is not a regular file", source)
	}

	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return Plugin{}, fmt.Errorf("create plugin dir: %w", err)
	}
	dest := filepath.Join(destDir, base)
	if err := copyExecutable(source, dest); err != nil {
		return Plugin{}, err
	}
	return Plugin{Name: name, Path: dest}, nil
}

// copyExecutable copies source to dest atomically (write to a temp file in the
// same dir, then rename) with 0o755 permissions so the result is executable.
func copyExecutable(source, dest string) error {
	// source is the user-supplied plugin path (`okfctl plugin install <path>`);
	// reading the file the user named is the command's whole purpose, and it was
	// verified to be a regular file above.
	in, err := os.Open(source) //nolint:gosec // G304: user-supplied plugin path is the intended input
	if err != nil {
		return fmt.Errorf("open plugin source: %w", err)
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".okfctl-install-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close() // best-effort; the copy error is the one that matters
		return fmt.Errorf("copy plugin: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	// The installed artifact is an executable plugin binary; 0o755 is required
	// so it can be exec'd. The source was verified to be a regular file above.
	if err := os.Chmod(tmpName, 0o755); err != nil { //nolint:gosec // G302: executable plugin must be 0o755
		return fmt.Errorf("chmod plugin: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("install plugin: %w", err)
	}
	return nil
}
