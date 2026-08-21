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

package clock

import (
	"strings"
	"testing"
	"time"
)

// A valid SOURCE_DATE_EPOCH resolves to a constant clock pinned to that exact
// Unix second in UTC, and two calls return the identical instant (the property
// the whole reproducibility feature rests on).
func TestResolve_ValidEpochPinsInstant(t *testing.T) {
	t.Setenv(EnvSourceDateEpoch, "1700000000")
	fn, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	want := time.Unix(1700000000, 0).UTC()
	got1 := fn()
	got2 := fn()
	if !got1.Equal(want) {
		t.Fatalf("pinned instant = %v, want %v", got1, want)
	}
	if !got1.Equal(got2) {
		t.Fatalf("pinned clock is not constant: %v != %v", got1, got2)
	}
	if got1.Location() != time.UTC {
		t.Fatalf("pinned instant is not UTC: %v", got1.Location())
	}
}

// With the variable unset, Resolve returns the real-wall-time clock and no
// error. This is the load-bearing negative control at the resolver level: the
// default path must be byte-for-byte the pre-existing behaviour.
func TestResolve_UnsetIsWallClock(t *testing.T) {
	// t.Setenv followed by Unsetenv guarantees the var is absent even if the
	// developer's shell exported it.
	t.Setenv(EnvSourceDateEpoch, "")
	fn, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	before := time.Now().UTC().Add(-time.Second)
	got := fn()
	after := time.Now().UTC().Add(time.Second)
	if got.Before(before) || got.After(after) {
		t.Fatalf("wall clock %v not within [%v, %v]", got, before, after)
	}
}

// A malformed value is a hard error whose message NAMES the variable, and the
// returned clock func is nil so a caller cannot accidentally proceed to write.
func TestResolve_MalformedIsHardError(t *testing.T) {
	for _, bad := range []string{"not-a-number", "17.5", "0x10", "12three", "  ", "-", "nan"} {
		t.Run(bad, func(t *testing.T) {
			// "  " (all-whitespace) is treated as UNSET, not malformed — assert
			// that separately below; skip it here.
			if strings.TrimSpace(bad) == "" {
				return
			}
			t.Setenv(EnvSourceDateEpoch, bad)
			fn, err := Resolve()
			if err == nil {
				t.Fatalf("Resolve(%q): want error, got nil", bad)
			}
			if fn != nil {
				t.Fatalf("Resolve(%q): want nil clock on error, got non-nil", bad)
			}
			if !strings.Contains(err.Error(), EnvSourceDateEpoch) {
				t.Fatalf("Resolve(%q): error %q must name %s", bad, err, EnvSourceDateEpoch)
			}
		})
	}
}

// A negative value is rejected: SOURCE_DATE_EPOCH is defined as seconds since
// the epoch, so a negative count is a mis-set variable, not "before 1970".
func TestResolve_NegativeIsHardError(t *testing.T) {
	t.Setenv(EnvSourceDateEpoch, "-1")
	fn, err := Resolve()
	if err == nil {
		t.Fatalf("Resolve(-1): want error, got nil")
	}
	if fn != nil {
		t.Fatalf("Resolve(-1): want nil clock on error")
	}
	if !strings.Contains(err.Error(), EnvSourceDateEpoch) {
		t.Fatalf("error %q must name %s", err, EnvSourceDateEpoch)
	}
}

// All-whitespace and empty are treated as UNSET (wall clock, no error): a
// present-but-blank variable is common in CI shells and must not hard-fail.
func TestResolve_BlankIsUnset(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t"} {
		t.Setenv(EnvSourceDateEpoch, blank)
		fn, err := Resolve()
		if err != nil {
			t.Fatalf("Resolve(%q): want wall clock, got error %v", blank, err)
		}
		if fn == nil {
			t.Fatalf("Resolve(%q): want non-nil wall clock", blank)
		}
	}
}

// Install wires a resolved clock into the process-wide Now; Reset undoes it.
func TestInstallAndReset(t *testing.T) {
	t.Cleanup(Reset)
	pinned := time.Unix(1700000000, 0).UTC()
	Install(func() time.Time { return pinned })
	if got := Now(); !got.Equal(pinned) {
		t.Fatalf("after Install, Now() = %v, want %v", got, pinned)
	}
	Reset()
	// After Reset the clock is wall time again (not the pinned instant).
	if got := Now(); got.Equal(pinned) {
		t.Fatalf("after Reset, Now() still pinned to %v", pinned)
	}
}

// Install(nil) is a no-op: it must not blank the clock.
func TestInstallNilIsNoOp(t *testing.T) {
	t.Cleanup(Reset)
	pinned := time.Unix(1700000000, 0).UTC()
	Install(func() time.Time { return pinned })
	Install(nil)
	if got := Now(); !got.Equal(pinned) {
		t.Fatalf("Install(nil) changed the clock: Now() = %v, want %v", got, pinned)
	}
}
