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

import (
	"testing"
	"time"

	"github.com/cwest/okfctl/internal/clock"
)

// The okf write-clock seam (nowUTC) must resolve from the single process-wide
// source (internal/clock), so a pinned SOURCE_DATE_EPOCH reaches created/
// modified and the log.md date heading. This asserts the seam is not an
// independent clock: installing a pinned instant on clock makes nowUTC return
// it. Paired with the apiserver and cmd seam tests, this proves all seams agree
// under one epoch (#143).
func TestNowUTCReadsSharedClock(t *testing.T) {
	t.Cleanup(clock.Reset)
	pinned := time.Unix(1700000000, 0).UTC()
	clock.Install(func() time.Time { return pinned })
	if got := nowUTC(); !got.Equal(pinned) {
		t.Fatalf("nowUTC() = %v, want shared clock instant %v", got, pinned)
	}
	if got := nowUTC(); !got.Equal(clock.Now()) {
		t.Fatalf("nowUTC() %v disagrees with clock.Now() %v", got, clock.Now())
	}
}
