/*
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// The SINGLE source of truth for the Google Analytics 4 tag.
//
// The site has two independent `<head>` render paths, and instrumenting one
// silently leaves the other untracked: docs pages inherit the Starlight `head`
// array in astro.config.mjs, while the homepage (src/pages/index.astro) is a
// standalone Astro route Starlight never renders and carries its own head. Both
// paths draw the tag from buildAnalyticsHeadTags() below, so the tag SHAPE is
// defined exactly once. tests/analytics.test.mjs proves the tag lands on both
// paths (positive control) and on neither when unconfigured (negative control).
//
// The two paths resolve the GATE INPUTS differently — and must, because they run
// in different environments:
//   - The component path (analyticsHeadTags, used by src/components/
//     Analytics.astro) is compiled into Vite's app graph, where PUBLIC_* env vars
//     are injected into import.meta.env and import.meta.env.PROD correctly tells
//     dev from build. So it reads import.meta.env.
//   - The config path (astro.config.mjs) is NOT in the app graph: Vite never
//     injects PUBLIC_* into its import.meta.env and import.meta.env.PROD is a
//     constant `true` there. So it supplies enabled/measurementId from process.*
//     and calls buildAnalyticsHeadTags directly.
// Both funnel through the same builder, so the emitted tag cannot diverge.

// The GA4 head-array entry shape: `{ tag, attrs?, content? }`, matching what
// Starlight's `head` array accepts and what Analytics.astro iterates.
export type AnalyticsHeadTag =
  | { tag: "script"; attrs: { async: true; src: string }; content?: undefined }
  | { tag: "script"; content: string; attrs?: undefined };

/**
 * The head entries for the GA4 tag. Pure: returns the entries when `enabled` and
 * a non-empty `measurementId` are given, else an empty array — so both render
 * paths can spread it unconditionally. A missing/empty id degrades to no-tag,
 * never a broken script or a build failure.
 *
 * Emits the standard two-line gtag.js snippet inline; no npm dependency is added
 * (the operator's other site's `@astrolib/analytics` is deliberately not pulled
 * in here).
 */
export function buildAnalyticsHeadTags(opts: {
  enabled: boolean;
  measurementId: string | undefined;
}): AnalyticsHeadTag[] {
  if (!opts.enabled || !opts.measurementId) return [];
  const id = String(opts.measurementId);
  return [
    {
      tag: "script",
      attrs: {
        async: true,
        src: `https://www.googletagmanager.com/gtag/js?id=${id}`,
      },
    },
    {
      tag: "script",
      content:
        `window.dataLayer = window.dataLayer || [];\n` +
        `function gtag(){dataLayer.push(arguments);}\n` +
        `gtag('js', new Date());\n` +
        `gtag('config', '${id}');`,
    },
  ];
}

/**
 * Convenience wrapper for the component render path, which runs inside Vite's app
 * graph. Reads the production flag and the measurement id from import.meta.env
 * (Vite injects PUBLIC_* there), then delegates to buildAnalyticsHeadTags. Local
 * `astro dev` (import.meta.env.PROD === false) and secret-less builds (no id)
 * both emit nothing.
 */
export function analyticsHeadTags(): AnalyticsHeadTag[] {
  return buildAnalyticsHeadTags({
    enabled: Boolean(import.meta.env.PROD),
    measurementId: import.meta.env.PUBLIC_GA_MEASUREMENT_ID,
  });
}
