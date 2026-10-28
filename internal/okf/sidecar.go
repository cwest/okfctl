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

package okf

import (
	"os"
	"path/filepath"
	"strings"
)

// readOkfSidecar parses the bundle-root .okf sidecar into a map of its top-level
// scalar keys. The .okf is a small flat YAML document (e.g.
// "okf_version: 0.2\ntemplates: ../tpl"); this is the generalized reader that
// both readOkfVersion (§12) and readTemplatesRef (§9.2) build on, so the sidecar
// is no longer okf_version-only. A missing or unreadable file yields an empty
// map — the sidecar is always optional, and Load stays lenient. Keys and values
// are trimmed; blank lines and lines with no colon are skipped. The last
// occurrence of a duplicated key wins (matching readOkfVersion's historical
// first-non-empty-wins would require extra state and no caller needs it).
func readOkfSidecar(root string) map[string]string {
	out := map[string]string{}
	// root is the user's bundle root; reading its .okf sidecar is intended.
	data, err := os.ReadFile(filepath.Join(root, ".okf")) //nolint:gosec // G304: reading the user's own bundle sidecar
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(val)
	}
	return out
}

// readTemplatesRef returns the bundle-relative path declared by the .okf
// sidecar's `templates` key (§9.2), or the empty string when the key — or the
// sidecar — is absent. An empty string is the silent direction of the feature:
// no referenced template bundle. The path is returned verbatim (bundle-relative,
// e.g. "../tpl"); the resolver joins it to the bundle root.
func readTemplatesRef(root string) string {
	return readOkfSidecar(root)["templates"]
}
