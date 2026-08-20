# okfctl.dev site

The [Astro](https://astro.build) + [Starlight](https://starlight.astro.build)
site published to <https://okfctl.dev> by `.github/workflows/docs.yml`.

## The one rule: this site RENDERS docs, it does not GENERATE them

The command reference is authored exactly once, by `cmd/gendocs` in the Go tree,
into `../docs/commands/README.md`. A CI drift gate
(`cmd.TestCommandReference_NoDrift`) fails the build if that file falls out of
sync with the live command tree, so a new flag cannot reach `main` without the
reference already containing it.

This site's build **consumes that file verbatim**. `scripts/prepare-content.mjs`
copies the user-facing docs under `../docs` into `src/content/docs/_generated/`,
prepending Starlight frontmatter. That is *templating* — it adds no fact and
imports no cobra. A second doc generator here is the specific thing this site is
built to avoid: it could drift from the first and defeat the whole point.

`src/content/docs/_generated/` is a build artifact (gitignored) — never edit it,
never commit it. Edit the source under `../docs/` instead (and for the command
reference, run `go generate ./cmd` in the Go tree).

## Clean URLs: the `_generated/` staging dir is never a public path

The templated files live on disk under `_generated/`, but that build-artifact
prefix **never appears in a URL**. Each generated page carries an explicit
`slug:` in its frontmatter (derived from the page's `out` path in
`prepare-content.mjs`), so the docs publish at clean, permanent routes:

- `/concepts/`
- `/guides/<slug>/`
- `/reference/commands/`

`prepare-content.mjs` exports `LEGACY_TO_CLEAN`, the single source of truth
mapping every old `/_generated/...` path to its clean route; `astro.config.mjs`
imports it and feeds it to Astro's `redirects`, so any `/_generated/...` link
already shared keeps working (a static meta-refresh stub, `noindex`, kept out of
the sitemap) instead of 404ing. Add a page to the `pages` list and its slug,
sidebar-eligibility, and redirect all follow from that one entry.

`npm test` builds the site and asserts these invariants against the emitted
`dist/` — clean routes resolve, `_generated/*` redirects rather than 404s, and
the sitemap lists only clean URLs.

## Theming: the bespoke design and where the palette lives

The site serves a **bespoke visual design**, not the default Starlight look.

- **The homepage** is a standalone Astro page at `src/pages/index.astro` — it owns
  the `/` route and carries its own header, footer, and theme toggle. Its layout
  and components are styled by `src/styles/homepage.css`.
- **The docs pages** are Starlight, restyled through the CSS-custom-property seam
  by `src/styles/starlight-theme.css`, which maps Starlight's own `--sl-*` design
  variables onto the shared tokens. No Starlight component is forked.

### Change the palette in ONE place

`src/styles/tokens.css` is the **single source of truth** for the palette, type
scale, spacing, and radii. It was extracted unchanged from the design draft
(HSL ramps authored off four hue anchors; the accent is gold, `--h-accent: 41`,
chosen to avoid the AI-default violet). Both the homepage and the docs read these
tokens, so:

- To reshape the brand colour, change a hue anchor (`--h-accent`, `--h-ink`, …)
  or a ramp value in `tokens.css`. That one edit reflows the homepage and every
  docs page in both light and dark.
- **Light and dark are both authored** in `tokens.css` — the light ramp is a
  distinct design (white surfaces + shadow on a cool paper canvas, accent
  darkened to ochre for contrast), not an inversion of dark. The theme toggle
  shares Starlight's `starlight-theme` localStorage key, so a visitor's choice
  persists across the homepage and the docs.

### Every fact on the page derives — none is hand-typed

- **Version**: the nav chip, the install snippet, and the footer read
  `src/lib/version.ts` (GitHub releases API at build time). No version string is
  authored anywhere under `site/`.
- **Terminal panels / corpus scale**: the homepage shows the stable command
  *grammar* (`okfctl validate`, `lint --strict`, …), which the command-reference
  drift gate already proves matches the binary. It deliberately does NOT paste a
  version-pinned transcript or specific corpus counts — a render-only site does
  not run the binary to generate content, so those would rot. The qualitative
  "a real corpus, not a fixture" claim stays; the rotting numbers do not.

## Version numbers derive, never copied

`src/lib/version.ts` fetches
`https://api.github.com/repos/cwest/okfctl/releases/latest` at build time. No
version string is hand-typed anywhere in this site.

## Analytics

The site emits a Google Analytics 4 tag, gated so it appears **only** on a
production build with a configured measurement id:

- **Where the id comes from:** the `PUBLIC_GA_MEASUREMENT_ID` env var
  (`G-XXXXXXXXXX`), wired in `.github/workflows/docs.yml` from the repo
  **variable** of the same name (`vars.PUBLIC_GA_MEASUREMENT_ID`, not a
  secret—a measurement id ships in public client HTML and is not a credential).
  It is never hardcoded.
- **Building without it:** a missing or empty value degrades to **no tag**—the
  build stays clean. So `npm run dev` (not a production build) and any fork or PR
  preview building without the variable emit nothing. There is nothing to
  configure locally.
- **One tag, two render paths:** the snippet is defined once in
  `src/lib/analytics.ts` and injected into BOTH the standalone homepage
  (`src/components/Analytics.astro`) and the Starlight docs `head` array
  (`astro.config.mjs`). `tests/analytics.test.mjs` builds the site and greps the
  emitted HTML to prove the tag reaches both paths when the id is set and neither
  when it is unset.

## Local development

```sh
npm ci                # reproducible install from package-lock.json (Node 22)
npm run dev           # runs prebuild, then astro dev
npm run build         # runs prebuild, then astro build -> dist/
```

## Theming seam

Tailwind v4 is wired via `@astrojs/starlight-tailwind` + `@tailwindcss/vite`.
`src/styles/global.css` is the Starlight `customCss` entry point: it imports the
design tokens (`tokens.css`) and the Starlight token mapping
(`starlight-theme.css`). The web fonts load via `<link>` tags in the Starlight
`head` config in `astro.config.mjs` — a font `@import` in `customCss` follows the
Tailwind rule-emitting `@import`s and is dropped by the Tailwind v4 optimizer as
an invalid non-leading `@import`, so docs would fall back to system fonts.
Restyle by editing those files — see "Theming: the bespoke design and where the
palette lives" above.

## Custom domain

The custom domain (`okfctl.dev`) is a **server-side GitHub Pages repo setting**,
not a committed `CNAME` file. For Actions-sourced Pages, GitHub ignores a CNAME
file in the artifact, so committing one is misleading. There is deliberately no
`CNAME` file here.
