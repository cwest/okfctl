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

// Package plugin implements git/kubectl-style PATH discovery of okfctl-<name>
// plugin executables. It is stdlib-only and imports no cobra: the pure model of
// "what plugins exist and where," so the cmd layer can dispatch to them.
package plugin

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// prefix is the naming convention a plugin executable must follow, mirroring
// git-<name> and kubectl-<name>.
const prefix = "okfctl-"

// Plugin is a discovered okfctl plugin: its short name (the token after the
// prefix) and the absolute path to its executable.
type Plugin struct {
	Name string
	Path string
}

// Discover scans every directory on pathenv (split on the OS path-list
// separator) for executables named okfctl-<name>. It returns them sorted by
// Name, de-duplicated so the first occurrence on PATH wins.
func Discover(pathenv string) []Plugin {
	seen := make(map[string]bool)
	var out []Plugin
	for _, dir := range filepath.SplitList(pathenv) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // unreadable dir on PATH is not fatal
		}
		for _, e := range entries {
			base := e.Name()
			if !strings.HasPrefix(base, prefix) {
				continue
			}
			name := strings.TrimPrefix(base, prefix)
			if name == "" || seen[name] {
				continue
			}
			abs := filepath.Join(dir, base)
			if !isExecutable(abs) {
				continue
			}
			seen[name] = true
			out = append(out, Plugin{Name: name, Path: abs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup resolves okfctl-<name> to its absolute path via the same PATH scan
// Discover uses, returning ("", false) when no executable plugin is found.
func Lookup(name, pathenv string) (string, bool) {
	for _, p := range Discover(pathenv) {
		if p.Name == name {
			return p.Path, true
		}
	}
	return "", false
}

// isExecutable reports whether path is a regular file (following symlinks) with
// any execute bit set. Unix semantics; Windows execute-bit detection differs and
// is out of scope for this increment.
func isExecutable(path string) bool {
	info, err := os.Stat(path) // Stat follows symlinks
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
