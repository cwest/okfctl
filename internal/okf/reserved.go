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
)

// SpecVersion is the OKF spec version this build targets (OKF v0.3 §12). It is
// what `bundle init` declares and what `migrate` upgrades a bundle to. A
// bundle that declares no version is READ as this version for display, but
// the §12.1 compatibility fallbacks key on the declared version
// (Bundle.DeclaredOkfVersion), never on this constant.
const SpecVersion = "0.3"

// logHeader is the leading header of a reserved log.md; logPlaceholder is the
// empty-log hint the scaffold writes beneath it. AppendLog reconciles against
// both so the first real entry replaces the placeholder rather than pinning it
// below every entry. Keep the scaffold body and AppendLog in sync via these.
const (
	logHeader      = "# Change Log\n\n"
	logPlaceholder = "_No entries yet. Record changes with `okfctl log append`._\n"
)

// Scaffold writes a minimal conformant bundle into dir: a reserved index.md and
// log.md. The result passes Validate with zero findings (it has no concept
// nodes yet, so the type floor is vacuously satisfied).
//
// The scaffolded bundle-root index.md declares the bundle's version with a
// quoted okf_version frontmatter key, the spec-defined declaration (OKF v0.3
// §8.1: "Producers SHOULD put a bundle-root index.md with okf_version in each
// bundle"; §4: quote number-like values). No .okf sidecar is written: it is a
// legacy fallback read only when the root index.md declares nothing.
func Scaffold(dir string) error {
	// A scaffolded bundle dir holds shareable knowledge documents; 0o755 is intended.
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: shareable bundle content dir
		return err
	}
	files := map[string]string{
		"index.md": "---\nokf_version: \"" + SpecVersion + "\"\n---\n\n" +
			"# Knowledge Base\n\n_Progressive-disclosure entry point. Add \"start here\" links as nodes land._\n",
		"log.md": logHeader + logPlaceholder,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil { //nolint:gosec // G306: a scaffolded reserved file is shareable bundle content; 0o644 is intended
			return err
		}
	}
	return nil
}
