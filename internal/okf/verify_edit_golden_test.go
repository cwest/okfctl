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
	"testing"
	"time"
)

// A byte-exact golden proving the writer's output shape: existing frontmatter
// keys keep their order (type, title, created, modified, verified), the prior
// `verified` entry is preserved as element 0 of the list, the new event is
// appended as element 1, created/modified are byte-identical to the input, and
// the Markdown body (blank separator included) is reproduced verbatim. This is
// the regression fence for §5.2 append-only behavior through the shared,
// order-preserving frontmatter writer.
func TestAppendVerifiedFile_Golden(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "g.md")
	orig := "---\n" +
		"type: Concept\n" +
		"title: Tannin\n" +
		"created: 2026-01-01T00:00:00Z\n" +
		"modified: 2026-07-01T00:00:00Z\n" +
		"verified:\n" +
		"  - { by: process:nightly, at: 2026-06-01T00:00:00Z }\n" +
		"---\n" +
		"\n" +
		"# Tannin\n" +
		"\n" +
		"A phenolic compound.\n"
	if err := os.WriteFile(abs, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := AppendVerifiedFile(abs, "human:casey", time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("AppendVerifiedFile: %v", err)
	}

	want := "---\n" +
		"type: Concept\n" +
		"title: Tannin\n" +
		"created: 2026-01-01T00:00:00Z\n" +
		"modified: 2026-07-01T00:00:00Z\n" +
		"verified:\n" +
		"  - {by: 'process:nightly', at: '2026-06-01T00:00:00Z'}\n" +
		"  - by: human:casey\n" +
		"    at: 2026-08-20T12:00:00Z\n" +
		"---\n" +
		"\n" +
		"# Tannin\n" +
		"\n" +
		"A phenolic compound.\n"

	got := readNodeStr(t, abs)
	if got != want {
		t.Fatalf("golden mismatch:\n--- want ---\n%q\n--- got ---\n%q", want, got)
	}
}
