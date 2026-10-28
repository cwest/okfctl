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
// so tests can pin it to a fixed instant; production reads real LOCAL wall time.
//
// The name is historical: it returns time.Now() in the machine's local location,
// NOT a UTC-normalized instant. The local offset is load-bearing — the write
// path stamps created/modified in that offset (see stampValue) so the recorded
// calendar day matches the day git records for the authoring commit (which keeps
// the author's local offset). Stripping to UTC here re-introduced the evening
// false-positive drift class: an edit after 20:00 US-Eastern recorded tomorrow's
// UTC date while git kept today's local date, so okfctl's own writer produced a
// node its own drift check rejected. SPEC v0.2 §5 requires only "an explicit UTC
// offset" (e.g. 2026-06-30T14:00:00Z) — a local offset like -04:00 is an explicit
// UTC offset and valid ISO 8601, so writing local-offset RFC3339 stays on the
// spec floor.
var nowUTC = func() time.Time { return time.Now() }

// timestampLayout is the frontmatter timestamp form. The real corpus stamps
// created/modified as RFC3339 (e.g. "2026-06-26T00:00:00Z"), so okfctl writes
// the same layout for consistency with hand-authored history.
const timestampLayout = time.RFC3339

// stampValue renders `at` as RFC3339 in `at`'s OWN location (it does NOT force
// UTC). This is the single formatter every write site uses, so the recorded
// calendar day is the author's local day — the frame git records for the commit,
// which is the frame drift's sameCalendarDay (drift.go §5) compares against. A
// UTC-pinned `at` still renders with a Z offset, so pinned-clock tests are
// unaffected; the local offset only matters for a real local-zone clock.
func stampValue(at time.Time) string { return at.Format(timestampLayout) }

// stampCreated records both created and modified as `at` (RFC3339 in at's own
// location) on a frontmatter map at node birth. created is the immutable birth
// marker; both start equal. Use touchModified for every subsequent write.
func stampCreated(fm map[string]any, at time.Time) {
	stamp := stampValue(at)
	fm["created"] = stamp
	fm["modified"] = stamp
}

// touchModified advances modified to `at` (RFC3339 in at's own location). It
// NEVER invents a created value: a node authored outside okfctl (in $EDITOR) may
// lack created, and fabricating one would lie about the node's birth. created is
// left exactly as found — present-and-unchanged, or absent.
func touchModified(fm map[string]any, at time.Time) {
	fm["modified"] = stampValue(at)
}
