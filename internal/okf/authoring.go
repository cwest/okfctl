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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewNode creates a conformant concept node at relPath (bundle-relative) with a
// required non-empty type (§7.2). It refuses an empty type and refuses to
// overwrite an existing file. Returns the absolute path written.
func NewNode(root, relPath, typ, title string) (string, error) {
	if strings.TrimSpace(typ) == "" {
		return "", fmt.Errorf("type is required and must be non-empty (OKF §7)")
	}
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("node already exists: %s", relPath)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: " + typ + "\n")
	if strings.TrimSpace(title) != "" {
		sb.WriteString("title: " + title + "\n")
	}
	sb.WriteString("---\n\n")
	heading := title
	if heading == "" {
		heading = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}
	sb.WriteString("# " + heading + "\n")
	if err := os.WriteFile(abs, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return abs, nil
}
