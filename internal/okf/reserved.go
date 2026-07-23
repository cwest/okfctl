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

package okf

import (
	"os"
	"path/filepath"
)

// SpecVersion is the OKF spec version this build targets.
const SpecVersion = "0.1"

// logHeader is the leading header of a reserved log.md; logPlaceholder is the
// empty-log hint the scaffold writes beneath it. AppendLog reconciles against
// both so the first real entry replaces the placeholder rather than pinning it
// below every entry. Keep the scaffold body and AppendLog in sync via these.
const (
	logHeader      = "# Change Log\n\n"
	logPlaceholder = "_No entries yet. Record changes with `okfctl log append`._\n"
)

// Scaffold writes a minimal conformant bundle into dir: a reserved index.md and
// log.md and an .okf spec pin. The result passes Validate with zero findings
// (it has no concept nodes yet, so the type floor is vacuously satisfied).
func Scaffold(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]string{
		"index.md": "---\ntype: Index\n---\n\n# Knowledge Base\n\n_Progressive-disclosure entry point. Add \"start here\" links as nodes land._\n",
		"log.md":   logHeader + logPlaceholder,
		".okf":     "okf_version: " + SpecVersion + "\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
