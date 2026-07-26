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

import "time"

// nowUTC is the package clock. It is a package var (not a direct time.Now call)
// so tests can pin it to a fixed instant; production reads real UTC wall time.
var nowUTC = func() time.Time { return time.Now().UTC() }

// timestampLayout is the frontmatter timestamp form. The real corpus stamps
// created/modified as RFC3339 (e.g. "2026-06-26T00:00:00Z"), so okfctl writes
// the same layout for consistency with hand-authored history.
const timestampLayout = time.RFC3339

// stampCreated records both created and modified as `at` (RFC3339 UTC) on a
// frontmatter map at node birth. created is the immutable birth marker; both
// start equal. Use touchModified for every subsequent write.
func stampCreated(fm map[string]any, at time.Time) {
	stamp := at.UTC().Format(timestampLayout)
	fm["created"] = stamp
	fm["modified"] = stamp
}

// touchModified advances modified to `at` (RFC3339 UTC). It NEVER invents a
// created value: a node authored outside okfctl (in $EDITOR) may lack created,
// and fabricating one would lie about the node's birth. created is left exactly
// as found — present-and-unchanged, or absent.
func touchModified(fm map[string]any, at time.Time) {
	fm["modified"] = at.UTC().Format(timestampLayout)
}
