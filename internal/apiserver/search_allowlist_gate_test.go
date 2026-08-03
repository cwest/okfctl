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
	"net/http"
	"testing"
)

// Regression guard: lexical_gate must be in the /api/v1/search allowlist so a
// request carrying it reaches lookupGateParam instead of being rejected by the
// closed-set validation first. The feature (#81) added the gate handler but not
// the allowlist entry, so every gated request 400'd before the handler ran. This
// test asserts the allowlist accepts lexical_gate — both separator spellings,
// since canonicalParam normalizes '-' to '_' — while a genuinely unknown param
// still 400s, so the fix cannot be a blanket "accept everything" that also drops
// the strictness #80 established.
//
// Both controls, per AGENTS.md:
//   - Positive: a valid gate value (?lexical_gate=true / ?lexical-gate=true) → 200.
//   - Negative: a genuinely unknown param → still 400.
func TestSearch_LexicalGateIsAcceptedParam(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	// Positive control: both accepted separator spellings reach the handler and
	// return 200, not the 400-before-dispatch the allowlist gap produced.
	for _, target := range []string{
		"/api/v1/search?q=wine+acidity&k=6&lexical_gate=true",
		"/api/v1/search?q=wine+acidity&k=6&lexical-gate=true",
	} {
		rec := doSearch(t, h, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200 (lexical_gate must be an accepted param; body=%s)",
				target, rec.Code, rec.Body.String())
		}
	}

	// Negative control: the allowlist stays closed — a genuinely unknown param is
	// still rejected. Proves the fix is a single allowlist entry, not a relaxation
	// of the closed-set validation.
	rec := doSearch(t, h, "/api/v1/search?q=wine&genuinely_unknown_param=true")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown param = %d, want 400 (allowlist must stay closed; body=%s)",
			rec.Code, rec.Body.String())
	}
}
