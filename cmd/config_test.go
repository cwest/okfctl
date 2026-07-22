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

package cmd

import (
	"strings"
	"testing"
)

func TestConfigSetGet_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OKFCTL_CONFIG_HOME", dir)
	if _, err := runOKF(t, "config", "set", "editor", "vim"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	out, err := runOKF(t, "config", "get", "editor")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if !strings.Contains(out, "vim") {
		t.Errorf("config get editor = %q, want vim", out)
	}
}
