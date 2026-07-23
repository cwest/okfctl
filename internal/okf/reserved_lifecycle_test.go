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
	"strings"
	"testing"
)

func writeNode(t *testing.T, dir, rel, typ, title string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderIndex_GroupsByNeighborhoodDeterministically(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	writeNode(t, dir, "lifting/squat.md", "Playbook", "Squat")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	if !strings.HasPrefix(got, "---\ntype: Index\n") {
		t.Errorf("index missing Index frontmatter; got:\n%s", got)
	}
	li := strings.Index(got, "lifting")
	wi := strings.Index(got, "wine")
	if li < 0 || wi < 0 || li > wi {
		t.Errorf("neighborhoods not sorted (lifting before wine); got:\n%s", got)
	}
	ai := strings.Index(got, "acidity.md")
	ti := strings.Index(got, "tannin.md")
	if ai < 0 || ti < 0 || ai > ti {
		t.Errorf("nodes not sorted within neighborhood; got:\n%s", got)
	}
	if !strings.Contains(got, "[Tannin](wine/tannin.md)") {
		t.Errorf("node not rendered as a titled link; got:\n%s", got)
	}
	if !strings.Contains(got, "Reference") {
		t.Errorf("node type not surfaced; got:\n%s", got)
	}
}

func TestRenderIndex_Deterministic(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "a/one.md", "Reference", "One")
	writeNode(t, dir, "b/two.md", "Reference", "Two")
	b, _ := Load(dir)
	if RenderIndex(b) != RenderIndex(b) {
		t.Fatal("RenderIndex is not deterministic across calls")
	}
}

func TestIndexInSync_TrueAfterBuildFalseAfterChange(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	b, _ := Load(dir)
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(RenderIndex(b)), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, _ := Load(dir)
	if ok, diff := IndexInSync(b2); !ok {
		t.Errorf("index should be in sync right after build; diff:\n%s", diff)
	}
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	b3, _ := Load(dir)
	if ok, _ := IndexInSync(b3); ok {
		t.Error("index should be STALE after adding a node")
	}
}

func TestAppendLog_CreatesAndAccumulates(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if err := AppendLog(dir, "first change"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLog(dir, "second change"); err != nil {
		t.Fatal(err)
	}
	body, err := ReadLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "first change") || !strings.Contains(body, "second change") {
		t.Errorf("log lost an entry; got:\n%s", body)
	}
	if strings.Index(body, "second change") > strings.Index(body, "first change") {
		t.Errorf("log not newest-first; got:\n%s", body)
	}
	if !strings.Contains(body, "20") {
		t.Errorf("log entry missing a timestamp; got:\n%s", body)
	}
}

func TestAppendLog_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	if err := AppendLog(dir, "   "); err == nil {
		t.Fatal("AppendLog must reject an empty/whitespace message")
	}
}
