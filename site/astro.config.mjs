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
