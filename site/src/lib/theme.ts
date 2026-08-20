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

// theme.ts is the PURE, unit-tested source of truth for the homepage's
// tri-state (system / light / dark) theme toggle. It holds no DOM access and no
// browser globals so it can be exercised in Node.
//
// The homepage (src/pages/index.astro) carries TWO inline copies of this same
// algorithm — a FOUC-safe pre-paint copy that runs before first paint, and a
// runtime copy that wires the cycling button and the live-OS listener. Inline
// `is:inline` scripts cannot import a module, so those copies are hand-mirrored
// and MUST stay in lockstep with the functions here; tests/theme-toggle.test.mjs
// pins the contract so a drift reddens the suite.
//
// THE INTEROP CONSTRAINT (load-bearing). The homepage shares the
// `starlight-theme` localStorage key with the Starlight docs pages, and that
// sharing is deliberate. Starlight's own ThemeSelect is already tri-state
// (auto / light / dark) and encodes "auto / follow the OS" as the EMPTY STRING:
//
//     localStorage.setItem(key, theme === 'light' || theme === 'dark' ? theme : '')
//
// So this module NEVER invents a literal 'system' in that key. It reads any of
// '', null, undefined, or an unknown value as the "system" choice
// (normalizeChoice), and writes the "system" choice back as '' (choiceToStorage)
// — matching exactly what the docs pages read and write. That round-trip is what
// carries a visitor's choice across the homepage and the docs in both
// directions.

// The user-facing tri-state choice. Distinct from the EFFECTIVE theme
// ('light' | 'dark') that actually paints: 'system' resolves to one of those via
// the OS preference.
export type ThemeChoice = "system" | "light" | "dark";
export type EffectiveTheme = "light" | "dark";

// The cycle order for the button: system -> light -> dark -> system.
const THEME_CHOICES: ThemeChoice[] = ["system", "light", "dark"];

// Fold any stored value into a tri-state choice. '' (Starlight's auto encoding),
// null, undefined, and any unrecognized string all mean "system". Only the exact
// literals 'light' and 'dark' are explicit choices.
export function normalizeChoice(raw: unknown): ThemeChoice {
  return raw === "light" || raw === "dark" ? raw : "system";
}

// Resolve the choice to the theme that actually paints. 'system' follows the OS;
// an explicit 'light'/'dark' ignores it.
export function resolveEffectiveTheme(
  choice: unknown,
  systemPrefersDark: boolean,
): EffectiveTheme {
  const normalized = normalizeChoice(choice);
  if (normalized === "system") {
    return systemPrefersDark ? "dark" : "light";
  }
  return normalized;
}

// The next choice in the cycle. Normalizes first, so a stored '' cycles from
// 'system' to 'light'.
export function nextChoice(current: unknown): ThemeChoice {
  const idx = THEME_CHOICES.indexOf(normalizeChoice(current));
  return THEME_CHOICES[(idx + 1) % THEME_CHOICES.length];
}

// The aria-label announcing the CURRENT state and the NEXT one, e.g.
// "Theme: system (tap for light)". Matches caseywest.com's UX copy.
export function ariaLabelFor(choice: unknown): string {
  const current = normalizeChoice(choice);
  return `Theme: ${current} (tap for ${nextChoice(current)})`;
}

// The write side of the interop contract: turn a choice into the value stored
// under `starlight-theme`. 'system' -> '' (matching Starlight); 'light'/'dark'
// written literally. An unknown value normalizes to 'system' first, so it too
// stores ''.
export function choiceToStorage(choice: unknown): string {
  const normalized = normalizeChoice(choice);
  return normalized === "system" ? "" : normalized;
}
