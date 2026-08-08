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
import { LEGACY_TO_CLEAN } from "./scripts/prepare-content.mjs";

// https://astro.build/config
export default defineConfig({
  // GitHub Pages serves this project from the custom domain okfctl.dev (a
  // server-side repo setting, NOT a committed CNAME file — see .github/workflows/
  // docs.yml). The apex domain means the site is served from the root, so no
  // `base` path is needed.
  site: "https://okfctl.dev",
  // Redirect the old _generated/* URLs (the build-artifact paths the docs were
  // briefly published under) to their clean, permanent routes. Anything already
  // shared keeps working instead of 404ing. The mapping is derived from the same
  // page list prepare-content.mjs uses, so a new page's redirect cannot be
  // forgotten. Astro emits these as static meta-refresh stubs — the only
  // redirect mechanism GitHub Pages (no server) supports.
  redirects: LEGACY_TO_CLEAN,
  vite: {
    // Tailwind v4 is wired via its first-party Vite plugin. This is the theming
    // SEAM the bespoke design restyles through: starlightTailwind() below tells
    // Starlight to defer to Tailwind's design tokens, and src/styles/global.css
    // maps Starlight's --sl-* variables onto the shared tokens (src/styles/
    // tokens.css). See site/README.md for how the palette is changed in one place.
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
        // The bespoke web fonts (Newsreader/Inter/JetBrains Mono) load via <link>
        // in the docs <head> — the same proven path the standalone homepage uses
        // (src/pages/index.astro). A CSS `@import url(...)` in customCss is dropped
        // by the Tailwind v4 optimizer: a font @import that follows the Tailwind
        // rule-emitting @imports is an invalid non-leading @import per the CSS
        // spec, so the docs would otherwise fall back to system fonts.
        {
          tag: "link",
          attrs: { rel: "preconnect", href: "https://fonts.googleapis.com" },
        },
        {
          tag: "link",
          attrs: {
            rel: "preconnect",
            href: "https://fonts.gstatic.com",
            crossorigin: true,
          },
        },
        {
          tag: "link",
          attrs: {
            rel: "stylesheet",
            href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500;700&family=Newsreader:ital,opsz,wght@0,6..72,300..600;1,6..72,300..500&display=swap",
          },
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
      // global.css also imports the bespoke design tokens and the Starlight token
      // mapping, so the docs pages inherit the homepage's design language.
      customCss: ["./src/styles/global.css"],
      // Pages are sourced from src/content/docs/. The user-facing docs under
      // ../docs are copied in at build time by scripts/prepare-content.mjs
      // (into _generated/) with Starlight frontmatter prepended — templating,
      // not generation. Authored site pages (the homepage) live directly under
      // src/content/docs/.
      // Sidebar entries reference the pages' PUBLISHED slugs (clean URLs), not
      // their on-disk _generated/ path. `autogenerate` keys off the on-disk
      // directory and would fight the slug override, so the guides are listed
      // explicitly in their intended order instead.
      sidebar: [
        {
          label: "Start here",
          items: [{ label: "Concepts", slug: "concepts" }],
        },
        {
          label: "Guides",
          items: [
            { label: "Authoring a bundle", slug: "guides/authoring" },
            { label: "Index & freshness", slug: "guides/index-and-freshness" },
            { label: "Curation health", slug: "guides/curation-health" },
            { label: "Search", slug: "guides/search" },
            { label: "Migrating", slug: "guides/migrating" },
            { label: "Remote sources", slug: "guides/remote-sources" },
            { label: "Plugins", slug: "guides/plugins" },
          ],
        },
        {
          label: "Reference",
          items: [
            { label: "Command reference", slug: "reference/commands" },
          ],
        },
      ],
    }),
  ],
});
