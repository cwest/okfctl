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

// theme-toggle.test.mjs exercises the PURE tri-state theme logic in
// src/lib/theme.ts. That module is the single source of truth for the toggle's
// behavior; two copies of the same functions ride along in index.astro (a
// FOUC-safe pre-paint copy and a runtime copy) and MUST stay in lockstep with
// it. Because those inline copies cannot be imported, this suite pins the
// contract on the importable module so a drift in the algorithm reddens here.
//
// The load-bearing interop constraint (per the card): the homepage shares the
// `starlight-theme` localStorage key with Starlight's own tri-state
// ThemeSelect, which encodes "auto/follow the system" as the EMPTY STRING and
// writes only 'light'/'dark' literally. So:
//   - normalizeChoice MUST fold '', null, undefined, and any unknown value to
//     'system' (never invent a literal 'system' in that key).
//   - choiceToStorage MUST round-trip: 'system' -> '' (matching Starlight),
//     'light' -> 'light', 'dark' -> 'dark'.
// These two directions are what makes a choice set on a docs page carry to the
// homepage and back.

import { test } from "node:test";
import assert from "node:assert/strict";
import {
  normalizeChoice,
  resolveEffectiveTheme,
  nextChoice,
  ariaLabelFor,
  choiceToStorage,
} from "../src/lib/theme.ts";

// --- normalizeChoice: the interop fold ---

test("normalizeChoice: '' (Starlight's auto encoding) -> system", () => {
  assert.equal(normalizeChoice(""), "system");
});

test("normalizeChoice: null / undefined -> system (first visit, nothing stored)", () => {
  assert.equal(normalizeChoice(null), "system");
  assert.equal(normalizeChoice(undefined), "system");
});

test("normalizeChoice: any unknown value -> system (best-effort tolerance)", () => {
  assert.equal(normalizeChoice("auto"), "system");
  assert.equal(normalizeChoice("SYSTEM"), "system");
  assert.equal(normalizeChoice("garbage"), "system");
});

test("normalizeChoice: the literal 'system' -> system (idempotent)", () => {
  assert.equal(normalizeChoice("system"), "system");
});

test("normalizeChoice: 'light' and 'dark' pass through unchanged", () => {
  assert.equal(normalizeChoice("light"), "light");
  assert.equal(normalizeChoice("dark"), "dark");
});

// --- resolveEffectiveTheme: all three choices against both OS preferences ---

test("resolveEffectiveTheme: system follows the OS (dark preference -> dark)", () => {
  assert.equal(resolveEffectiveTheme("system", true), "dark");
});

test("resolveEffectiveTheme: system follows the OS (light preference -> light)", () => {
  assert.equal(resolveEffectiveTheme("system", false), "light");
});

test("resolveEffectiveTheme: explicit light ignores the OS in both directions", () => {
  assert.equal(resolveEffectiveTheme("light", true), "light");
  assert.equal(resolveEffectiveTheme("light", false), "light");
});

test("resolveEffectiveTheme: explicit dark ignores the OS in both directions", () => {
  assert.equal(resolveEffectiveTheme("dark", true), "dark");
  assert.equal(resolveEffectiveTheme("dark", false), "dark");
});

test("resolveEffectiveTheme: '' resolves through the system fold, following the OS", () => {
  assert.equal(resolveEffectiveTheme("", true), "dark");
  assert.equal(resolveEffectiveTheme("", false), "light");
});

// --- nextChoice: the system -> light -> dark -> system cycle ---

test("nextChoice: cycles system -> light -> dark -> system", () => {
  assert.equal(nextChoice("system"), "light");
  assert.equal(nextChoice("light"), "dark");
  assert.equal(nextChoice("dark"), "system");
});

test("nextChoice: normalizes first, so '' cycles as system -> light", () => {
  assert.equal(nextChoice(""), "light");
  assert.equal(nextChoice(null), "light");
});

// --- ariaLabelFor: announce current AND next state ---

test("ariaLabelFor: announces current and next for each state", () => {
  assert.equal(ariaLabelFor("system"), "Theme: system (tap for light)");
  assert.equal(ariaLabelFor("light"), "Theme: light (tap for dark)");
  assert.equal(ariaLabelFor("dark"), "Theme: dark (tap for system)");
});

test("ariaLabelFor: normalizes '' to the system label", () => {
  assert.equal(ariaLabelFor(""), "Theme: system (tap for light)");
});

// --- choiceToStorage: the write side of the interop contract ---

test("choiceToStorage: system -> '' (matches Starlight's auto encoding)", () => {
  assert.equal(choiceToStorage("system"), "");
});

test("choiceToStorage: light and dark are written literally", () => {
  assert.equal(choiceToStorage("light"), "light");
  assert.equal(choiceToStorage("dark"), "dark");
});

test("choiceToStorage: normalizes an unknown value to '' before writing", () => {
  assert.equal(choiceToStorage("garbage"), "");
});

// --- round-trip: what we write, we read back as the same choice ---

test("round-trip: choiceToStorage then normalizeChoice is identity for all three", () => {
  for (const choice of ["system", "light", "dark"]) {
    assert.equal(normalizeChoice(choiceToStorage(choice)), choice);
  }
});
