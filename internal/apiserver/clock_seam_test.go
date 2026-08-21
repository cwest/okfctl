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

package apiserver

import (
	"testing"
	"time"

	"github.com/cwest/okfctl/internal/clock"
)

// The apiserver response-clock seam (the package `now`) must resolve from the
// single process-wide source (internal/clock), so a pinned SOURCE_DATE_EPOCH
// reaches /stats generated_at and the search-decay clock. Paired with the okf
// and cmd seam tests, this proves all three seams agree under one epoch (#143).
func TestNowReadsSharedClock(t *testing.T) {
	t.Cleanup(clock.Reset)
	pinned := time.Unix(1700000000, 0).UTC()
	clock.Install(func() time.Time { return pinned })
	if got := now(); !got.Equal(pinned) {
		t.Fatalf("now() = %v, want shared clock instant %v", got, pinned)
	}
	if got := now(); !got.Equal(clock.Now()) {
		t.Fatalf("now() %v disagrees with clock.Now() %v", got, clock.Now())
	}
}
