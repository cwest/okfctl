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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okfconfig"
	"github.com/cwest/okfctl/internal/search"
)

// isolateConfig points okfconfig at a temp dir so these tests never read or
// write the developer's real ~/.config/okfctl/config.json.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", dir)
	return dir
}

func TestResolveModel2vec_ErrorWhenUnset(t *testing.T) {
	isolateConfig(t)
	_, err := resolveEmbedder("model2vec", "")
	if err == nil {
		t.Fatal("want an error when no model path is configured, got nil")
	}
	// The error must TELL the user how to fix it, not just that it failed.
	for _, want := range []string{"model_path", "--model-path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestResolveModel2vec_ConfigFallback(t *testing.T) {
	isolateConfig(t)
	writeConfig(t, `{"model_path":"/nonexistent-model-dir"}`)
	_, err := resolveEmbedder("model2vec", "")
	if err == nil {
		t.Fatal("want a load error for a bogus configured dir, got nil")
	}
	// It got PAST resolution and tried to load the configured dir — proving the
	// config fallback was consulted rather than the "unset" branch firing.
	if strings.Contains(err.Error(), "--model-path") {
		t.Errorf("configured path should be used, not the unset error: %v", err)
	}
}

func TestResolveModel2vec_FlagWins(t *testing.T) {
	isolateConfig(t)
	writeConfig(t, `{"model_path":"/from-config"}`)
	_, err := resolveEmbedder("model2vec", "/from-flag")
	if err == nil {
		t.Fatal("want a load error for a bogus flag dir, got nil")
	}
	if !strings.Contains(err.Error(), "/from-flag") {
		t.Errorf("--model-path must win over config, got %v", err)
	}
	if strings.Contains(err.Error(), "/from-config") {
		t.Errorf("config path should have been overridden, got %v", err)
	}
}

func TestResolveHash_UnaffectedByModelPath(t *testing.T) {
	isolateConfig(t)
	e, err := resolveEmbedder("hash", "")
	if err != nil {
		t.Fatalf("hash must stay the zero-config default: %v", err)
	}
	if _, ok := e.(*search.HashEmbedder); !ok {
		t.Errorf("want *search.HashEmbedder, got %T", e)
	}
}

func TestResolveModel2vec_LoadsEmbedder(t *testing.T) {
	dir := os.Getenv("OKFCTL_TEST_MODEL_DIR")
	if dir == "" {
		t.Skip("set OKFCTL_TEST_MODEL_DIR to a potion-base-8M dir to run this test")
	}
	isolateConfig(t)
	e, err := resolveEmbedder("model2vec", dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.(*search.Model2VecEmbedder); !ok {
		t.Fatalf("want *search.Model2VecEmbedder, got %T", e)
	}
	if e.Dim() != 256 {
		t.Errorf("Dim() = %d, want 256", e.Dim())
	}
}

func writeConfig(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(okfconfig.Path()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(okfconfig.Path(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
