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
	"testing"
	"time"

	"github.com/cwest/okfctl/internal/clock"
)

// The cmd write-clock seam (nowUTCcmd) must resolve from the single process-wide
// source (internal/clock). Under a pinned instant it returns exactly that
// instant and agrees with clock.Now() — the third leg of the "all seams resolve
// from one source" contract (#143). The okf and apiserver legs are asserted in
// their own packages (nowUTC / now are unexported there); together the three
// tests prove nowUTC, nowUTCcmd, and the apiserver clock agree under one epoch.
func TestNowUTCcmdReadsSharedClock(t *testing.T) {
	t.Cleanup(clock.Reset)
	pinned := time.Unix(1700000000, 0).UTC()
	clock.Install(func() time.Time { return pinned })
	if got := nowUTCcmd(); !got.Equal(pinned) {
		t.Fatalf("nowUTCcmd() = %v, want shared clock instant %v", got, pinned)
	}
	if got := nowUTCcmd(); !got.Equal(clock.Now()) {
		t.Fatalf("nowUTCcmd() %v disagrees with clock.Now() %v", got, clock.Now())
	}
}
