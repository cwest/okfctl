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

//go:build fuzzcontrol

// This file is the POSITIVE CONTROL for the CI fuzz gate. It is compiled ONLY
// under the `fuzzcontrol` build tag, so it never runs in the normal test suite
// (`go test ./...`) — it would fail it on purpose. CI runs it in a dedicated
// step that ASSERTS the fuzz command exits non-zero: if a target with a
// guaranteed crasher does NOT fail the build, the fuzz gate is broken (it would
// silently green-light real crashers), and that inverted assertion is what the
// control proves. The real fuzz targets are the negative control: they run
// untagged and must pass.

package okf

import "testing"

// FuzzPositiveControl always panics on its non-empty seed. It exists solely to
// prove `go test -fuzz` actually fails the build when a target crashes. It is
// tag-gated so it cannot leak into the normal suite.
func FuzzPositiveControl(f *testing.F) {
	f.Add([]byte("crash"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 0 {
			panic("positive control: this crash MUST fail the fuzz gate")
		}
	})
}
