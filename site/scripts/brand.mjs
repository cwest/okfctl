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

// brand.mjs is the SINGLE SOURCE OF TRUTH for the site's brand tokens used by
// the generated share assets (favicon + og:image). The values here are the
// SAME tokens Starlight + the Tailwind preset ship as the default theme
// (indigo accent), so the favicon and social card cannot drift from the look
// the pages actually render. When the bespoke-design change lands a wordmark
// and palette, this is the one file to update and both assets follow.
//
// Verified against the built CSS (dist/_astro/common.*.css):
//   --sl-color-accent (accent-600)   = #4f46e5  (indigo-600)
//   accent-950 (dark surface)        = #1e1b4b  (indigo-950)
//   accent-200 (light accent)        = #c7d2fe  (indigo-200)

export const brand = {
  // The site's identity is a plain typographic mark until the bespoke-design
  // change lands a real wordmark. Ship a good default rather than nothing.
  wordmark: "okfctl",
  tagline: "Author and maintain Open Knowledge Format bundles.",
  domain: "okfctl.dev",

  // Site tokens (indigo accent — the shipped Starlight default).
  accent: "#4f46e5", // accent-600
  accentDark: "#1e1b4b", // accent-950 — dark surface
  accentLight: "#c7d2fe", // accent-200
  ink: "#ffffff",
  inkMuted: "#c7d2fe",
  mono: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
  sans: "system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif",
};
