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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandReference_EveryTopLevelCommandHasStableAnchor asserts the generated
// reference emits a `## okfctl <cmd>` heading for every top-level command in the
// tree. GitHub slugifies that heading to `#okfctl-<cmd>`, which is exactly the
// anchor README.md links to; a single-file reference keeps those links from
// 404ing. If a command is added or renamed and the anchor moves, this fails.
func TestCommandReference_EveryTopLevelCommandHasStableAnchor(t *testing.T) {
	root := NewRootCmd()
	ref := GenerateCommandReference(root)

	for _, c := range root.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		heading := "## okfctl " + c.Name()
		if !strings.Contains(ref, heading) {
			t.Errorf("generated reference is missing heading %q (anchor #okfctl-%s that README links to)", heading, c.Name())
		}
	}
}

// TestCommandReference_IsDeterministic guards the drift check itself: the
// generator must produce byte-identical output across runs, or the no-drift test
// would flap. No timestamps, no map iteration order.
func TestCommandReference_IsDeterministic(t *testing.T) {
	root1 := NewRootCmd()
	root2 := NewRootCmd()
	if GenerateCommandReference(root1) != GenerateCommandReference(root2) {
		t.Fatal("GenerateCommandReference is not deterministic; the drift check needs byte-stable output")
	}
}

// TestCommandReference_NoDrift is the CI drift gate. It regenerates the command
// reference from the live cobra tree and compares it to the committed
// docs/commands/README.md. When they differ, the committed reference is stale:
// the fix is to regenerate it (see the error message), never to hand-edit the
// committed file. This is what makes staleness impossible rather than merely
// discouraged.
func TestCommandReference_NoDrift(t *testing.T) {
	path := commandReferencePath(t)
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed command reference %s: %v", path, err)
	}
	generated := GenerateCommandReference(NewRootCmd())
	if string(committed) != generated {
		t.Errorf("committed command reference %s is out of date with the cobra tree.\n"+
			"Regenerate it with:\n\n    go generate ./cmd\n\n"+
			"(or: go run ./cmd/gendocs). Never hand-edit the committed file.", path)
	}
}

// commandReferencePath returns the absolute path to the committed command
// reference, resolved relative to this test file's package directory so the test
// works regardless of the process working directory.
func commandReferencePath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Tests run with the package dir (cmd/) as cwd; docs/ is one level up.
	return filepath.Join(wd, "..", "docs", "commands", "README.md")
}
