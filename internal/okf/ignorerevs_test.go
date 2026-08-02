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
)

// A bundle with no .okf-drift-ignore-revs yields an empty (non-nil) set and no
// error — the file is optional; its absence is the common case.
func TestLoadDriftIgnoreRevs_Absent(t *testing.T) {
	root := t.TempDir()
	got, err := LoadDriftIgnoreRevs(root)
	if err != nil {
		t.Fatalf("absent file must not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("absent file should yield empty set, got %v", got)
	}
}

// The file format mirrors `git blame --ignore-revs-file`: one SHA per line,
// blank lines and #-comments ignored, trailing whitespace trimmed. An inline
// comment after a SHA is also stripped.
func TestLoadDriftIgnoreRevs_ParsesFormat(t *testing.T) {
	root := t.TempDir()
	content := "# mechanical migration commits — opt these out of git drift\n" +
		"\n" +
		"abc123def456abc123def456abc123def456abcd\n" +
		"   fedcba098765fedcba098765fedcba098765fedc   # the v0.2 key sweep\n" +
		"\n" +
		"# another comment\n" +
		"1111111111111111111111111111111111111111\n"
	if err := os.WriteFile(filepath.Join(root, ".okf-drift-ignore-revs"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDriftIgnoreRevs(root)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{
		"abc123def456abc123def456abc123def456abcd",
		"fedcba098765fedcba098765fedcba098765fedc",
		"1111111111111111111111111111111111111111",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d revs, want %d: %v", len(got), len(want), got)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("missing rev %q in %v", w, got)
		}
	}
}
