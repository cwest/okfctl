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
