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

// Package clock is okfctl's single source of the current instant for every
// timestamp the tool WRITES (node created/modified, log.md date headings) and
// every response-time instant it reports (the apiserver /stats generated_at,
// search recency decay). Resolving the clock in exactly one place is what makes
// okfctl's output reproducible: with SOURCE_DATE_EPOCH set, every seam reads the
// SAME pinned instant, so two runs with different real wall clocks produce
// byte-identical bundles.
//
// SOURCE_DATE_EPOCH is the cross-ecosystem reproducible-builds convention
// (https://reproducible-builds.org/docs/source-date-epoch/): a decimal count of
// seconds since the Unix epoch (UTC). okfctl honours it exactly — a valid value
// pins the clock; an absent value falls back to real wall time; a malformed
// value is a hard error (see Resolve), never a silent fallback, because a build
// that silently ignores a mis-set pin is not reproducible and does not announce
// that it isn't.
package clock

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EnvSourceDateEpoch is the reproducible-builds environment variable okfctl
// honours. Exported so error messages and tests name it from one constant.
const EnvSourceDateEpoch = "SOURCE_DATE_EPOCH"

// mu guards nowFn so a concurrent apiserver read of the clock cannot race the
// one-time Install at startup.
var (
	mu    sync.RWMutex
	nowFn = wall
)

// wall is the real-wall-time clock, always UTC. It is the fallback when
// SOURCE_DATE_EPOCH is unset and the value every seam reads by default so that
// behaviour with the variable UNSET is byte-identical to okfctl before this
// package existed.
func wall() time.Time { return time.Now().UTC() }

// Now returns the current instant per okfctl's single clock: the pinned
// SOURCE_DATE_EPOCH instant when one was installed, else real UTC wall time.
// Every write-path and response-path seam funnels through here.
func Now() time.Time {
	mu.RLock()
	fn := nowFn
	mu.RUnlock()
	return fn()
}

// Resolve reads SOURCE_DATE_EPOCH from the process environment and returns the
// clock okfctl should use for this invocation, WITHOUT installing it:
//
//   - unset (or empty)  -> real UTC wall time, ok.
//   - valid decimal int -> a constant clock pinned to that Unix second (UTC).
//   - anything else      -> a non-nil error naming the variable; the caller
//     MUST abort before writing anything.
//
// Resolve is pure with respect to package state (it does not mutate nowFn), so
// a caller can validate the environment and fail closed BEFORE any file is
// touched, then Install the returned clock only on success. This ordering is
// the contract that makes a malformed value write nothing.
func Resolve() (func() time.Time, error) {
	raw, set := os.LookupEnv(EnvSourceDateEpoch)
	if !set || strings.TrimSpace(raw) == "" {
		return wall, nil
	}
	// SOURCE_DATE_EPOCH is a decimal count of seconds since the Unix epoch. It
	// is deliberately strict: no whitespace, no sign games beyond a leading '-',
	// no float, no hex. ParseInt base 10 enforces exactly that. A leading/
	// trailing space is a mis-set variable, so trim only for the empty check
	// above and parse the raw value.
	secs, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return nil, fmt.Errorf(
			"%s=%q is not a valid Unix timestamp (expected a decimal number of seconds since 1970-01-01T00:00:00Z): %w",
			EnvSourceDateEpoch, raw, err)
	}
	if secs < 0 {
		return nil, fmt.Errorf(
			"%s=%q is negative; expected a non-negative number of seconds since 1970-01-01T00:00:00Z",
			EnvSourceDateEpoch, raw)
	}
	pinned := time.Unix(secs, 0).UTC()
	return func() time.Time { return pinned }, nil
}

// Install sets the process-wide clock to fn. It is called once, at root-command
// startup, with the clock Resolve returned. Passing nil is a no-op (keeps the
// current clock) so a defensive caller cannot accidentally blank the clock.
func Install(fn func() time.Time) {
	if fn == nil {
		return
	}
	mu.Lock()
	nowFn = fn
	mu.Unlock()
}

// Reset restores the real-wall-time clock. It exists for tests that need to
// undo an Install; production never calls it.
func Reset() {
	mu.Lock()
	nowFn = wall
	mu.Unlock()
}
