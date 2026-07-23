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

func TestLogAppendThenShow(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "log", "append", dir, "--message", "added tannin node"); err != nil {
		t.Fatalf("log append: %v", err)
	}
	out, err := runOKF(t, "log", "show", dir)
	if err != nil {
		t.Fatalf("log show: %v", err)
	}
	if !strings.Contains(out, "added tannin node") {
		t.Errorf("log show missing the appended entry; got:\n%s", out)
	}
}

func TestLogAppend_RequiresMessage(t *testing.T) {
	dir := t.TempDir()
	_, _ = runOKF(t, "bundle", "init", dir)
	if _, err := runOKF(t, "log", "append", dir); err == nil {
		t.Fatal("log append without --message must error")
	}
}
