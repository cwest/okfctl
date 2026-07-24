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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runPlugin(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newSearchCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func writeSearchBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":        "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [T](wine/tannin.md)\n",
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nTannin gives structure and astringency.\n",
		"wine/acidity.md": "---\ntype: Concept\ntitle: Acidity\n---\n\n# Acidity\n\nAcidity gives freshness and lift.\n",
	}
	for rel, c := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPlugin_IndexBuildThenSemantic(t *testing.T) {
	dir := writeSearchBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	idx := filepath.Join(dir, ".okfctl", "index.db")
	if _, err := os.Stat(idx); err != nil {
		t.Fatalf("index.db not written: %v", err)
	}
	data, _ := os.ReadFile(idx)
	if !strings.Contains(string(data), "hash-test-embedder") {
		t.Errorf("index.db should record the model; got %s", data)
	}
	out, err := runPlugin(t, "--semantic", "tannin structure astringency", dir)
	if err != nil {
		t.Fatalf("semantic query: %v", err)
	}
	if !strings.Contains(out, "wine/tannin.md") {
		t.Errorf("semantic query should surface wine/tannin.md; got %q", out)
	}
}

func TestPlugin_Related(t *testing.T) {
	dir := writeSearchBundle(t)
	if _, err := runPlugin(t, "index", "build", dir); err != nil {
		t.Fatal(err)
	}
	out, err := runPlugin(t, "related", "wine/tannin.md", dir)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if strings.Contains(out, "wine/tannin.md") {
		t.Errorf("related should exclude the node itself; got %q", out)
	}
	if !strings.Contains(out, "wine/acidity.md") {
		t.Errorf("related should list the neighbor; got %q", out)
	}
}

func TestPlugin_Model2vecNotYet(t *testing.T) {
	dir := writeSearchBundle(t)
	_, err := runPlugin(t, "--embedder", "model2vec", "index", "build", dir)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not yet") {
		t.Errorf("model2vec should error 'not yet available'; got %v", err)
	}
}
