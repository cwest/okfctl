/**
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

// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";

// https://astro.build/config
export default defineConfig({
  // GitHub Pages serves this project from the custom domain okfctl.dev (a
  // server-side repo setting, NOT a committed CNAME file — see .github/workflows/
  // docs.yml). The apex domain means the site is served from the root, so no
  // `base` path is needed.
  site: "https://okfctl.dev",
  vite: {
    // Tailwind v4 is wired via its first-party Vite plugin. This is the theming
    // SEAM for the follow-up design card: starlightTailwind() below tells
    // Starlight to defer to Tailwind's design tokens. We do NOT restyle here —
    // the default Starlight look ships as-is.
    plugins: [tailwindcss()],
  },
  integrations: [
    starlight({
      title: "okfctl",
      description:
        "A command-line tool for authoring and maintaining Open Knowledge Format (OKF) bundles.",
      // The SVG mark shipped in public/favicon.svg (a placeholder typographic
      // mark until the bespoke-design change lands a wordmark). Starlight emits
      // the <link rel="icon"> for this; the PNG fallback + apple-touch-icon and
      // the social-card tags are added via `head` below because Starlight has no
      // first-party option for them.
      favicon: "/favicon.svg",
      head: [
        // PNG favicon fallback for clients that do not render SVG icons, plus
        // the iOS home-screen icon. Both are generated at build time by
        // scripts/generate-share-assets.mjs from public/favicon.svg.
        {
          tag: "link",
          attrs: { rel: "icon", href: "/favicon.png", type: "image/png", sizes: "180x180" },
        },
        {
          tag: "link",
          attrs: { rel: "apple-touch-icon", href: "/apple-touch-icon.png", sizes: "180x180" },
        },
        // Social share card. Starlight already emits og:title/description/url and
        // twitter:card=summary_large_image; it does NOT emit an image. Absolute
        // URLs are required so the card resolves when a link is unfurled off-site.
        // The 1200x630 PNG is generated at build time from the brand tokens.
        {
          tag: "meta",
          attrs: { property: "og:image", content: "https://okfctl.dev/og.png" },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:width", content: "1200" },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:height", content: "630" },
        },
        {
          tag: "meta",
          attrs: {
            property: "og:image:alt",
            content: "okfctl — author and maintain Open Knowledge Format bundles.",
          },
        },
        {
          tag: "meta",
          attrs: { name: "twitter:image", content: "https://okfctl.dev/og.png" },
        },
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/cwest/okfctl",
        },
      ],
      // Tailwind v4 is wired via its first-party Vite plugin (see `vite` above)
      // plus the Starlight Tailwind preset, imported from src/styles/global.css.
      // This is the theming SEAM for the follow-up design pass; intentionally no
      // custom palette or component CSS here — the default Starlight look ships.
      customCss: ["./src/styles/global.css"],
      // Pages are sourced from src/content/docs/. The user-facing docs under
      // ../docs are copied in at build time by scripts/prepare-content.mjs
      // (into _generated/) with Starlight frontmatter prepended — templating,
      // not generation. Authored site pages (the homepage) live directly under
      // src/content/docs/.
      sidebar: [
        {
          label: "Start here",
          items: [{ label: "Concepts", slug: "_generated/concepts" }],
        },
        {
          label: "Guides",
          items: [{ autogenerate: { directory: "_generated/guides" } }],
        },
        {
          label: "Reference",
          items: [
            { label: "Command reference", slug: "_generated/reference/commands" },
          ],
        },
      ],
    }),
  ],
});
