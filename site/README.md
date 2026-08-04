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

## Version numbers derive, never copied

`src/lib/version.ts` fetches
`https://api.github.com/repos/cwest/okfctl/releases/latest` at build time. No
version string is hand-typed anywhere in this site.

## Local development

```sh
npm ci                # reproducible install from package-lock.json (Node 22)
npm run dev           # runs prebuild, then astro dev
npm run build         # runs prebuild, then astro build -> dist/
```

## Theming seam

Tailwind v4 is wired via `@astrojs/starlight-tailwind` + `@tailwindcss/vite`.
This ships the **default Starlight look**; the bespoke visual design lands as a
separate change that restyles through this seam. Do not restyle here.

## Custom domain

The custom domain (`okfctl.dev`) is a **server-side GitHub Pages repo setting**,
not a committed `CNAME` file. For Actions-sourced Pages, GitHub ignores a CNAME
file in the artifact, so committing one is misleading. There is deliberately no
`CNAME` file here.
