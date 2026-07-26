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

package okfconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPath_HonorsEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", dir)
	want := filepath.Join(dir, "config.json")
	if got := Path(); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoad_MissingIsEmpty(t *testing.T) {
	t.Setenv("OKFCTL_CONFIG_HOME", t.TempDir()) // empty dir, no config.json
	m, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file err = %v, want nil", err)
	}
	if len(m) != 0 {
		t.Errorf("Load() = %v, want empty map", m)
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"model_path":"/x/model"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if m["model_path"] != "/x/model" {
		t.Errorf("model_path = %q, want /x/model", m["model_path"])
	}
}
