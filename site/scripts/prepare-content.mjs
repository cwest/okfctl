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

// prepare-content.mjs is the build-time TEMPLATING step for the site. It takes
// the repository's already-authored, user-facing Markdown under ../docs and
// copies it into src/content/docs/, prepending Starlight frontmatter (title,
// sidebar) so Starlight can render each page.
//
// This is deliberately NOT a generator. It imports no cobra, runs no okfctl,
// and invents no fact. The command reference in particular is consumed verbatim
// from ../docs/commands/README.md — the file that cmd.TestCommandReference_NoDrift
// already proves is byte-for-byte in sync with the live command tree. A second
// doc generator here could drift from that first one; this script exists so that
// cannot happen. The site's only job is to render a file CI has already proven
// fresh.
//
// Every page written here is derived from an existing source file with only a
// frontmatter header added. INTERNAL docs (docs/PRD.md, docs/plans/, docs/specs/,
// docs/adr/) are never touched — only user-facing content is published.

import { readFile, writeFile, mkdir, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const siteRoot = join(here, "..");
const repoRoot = join(siteRoot, "..");
const docsRoot = join(repoRoot, "docs");
const outRoot = join(siteRoot, "src", "content", "docs");
const genRoot = join(outRoot, "_generated");

// YAML-escape a scalar we control (titles are ours, not user input), but stay
// safe against colons and quotes so the emitted frontmatter always parses.
function yamlString(s) {
  return JSON.stringify(String(s));
}

// Build a Starlight frontmatter block. `body` is appended verbatim — no fact is
// added, only the header Starlight needs to render the page.
//
// `slug` sets the page's PUBLISHED route, decoupling it from the on-disk path.
// Starlight uses the frontmatter `slug` as the canonical URL, so a file written
// under _generated/concepts.md publishes at /concepts/ — the build-artifact
// prefix never reaches the public URL space.
function withFrontmatter({ title, slug, sidebarLabel, sidebarOrder, description }, body) {
  const lines = ["---", `title: ${yamlString(title)}`];
  if (slug) lines.push(`slug: ${yamlString(slug)}`);
  if (description) lines.push(`description: ${yamlString(description)}`);
  if (sidebarLabel || sidebarOrder !== undefined) {
    lines.push("sidebar:");
    if (sidebarLabel) lines.push(`  label: ${yamlString(sidebarLabel)}`);
    if (sidebarOrder !== undefined) lines.push(`  order: ${sidebarOrder}`);
  }
  lines.push("---", "");
  // Body starts with a top-level "# Heading"; Starlight renders `title` as the
  // page H1, so drop the source's leading H1 to avoid a duplicate heading.
  const trimmed = body.replace(/^\uFEFF?#[^\n]*\n+/, "");
  return lines.join("\n") + trimmed;
}

// Each entry maps ONE user-facing source doc to ONE published page. The source
// must exist; a missing source is a hard error (better a red build than a
// silently missing page). No entry points at docs/PRD.md, docs/plans/,
// docs/specs/, or docs/adr/ — those are internal and stay unpublished.
//
// `out` is where the page is WRITTEN on disk — under the private _generated/
// staging dir (gitignored). `slug` is where the page is PUBLISHED — the clean,
// permanent URL. The two are decoupled on purpose: keep the templated files out
// of the tracked tree, but never leak the _generated/ build-artifact prefix into
// a public URL. The slug is derived from `out` (drop the ".md") so the two
// cannot drift; a bespoke slug can still be set explicitly if ever needed.
const pages = [
  {
    src: "concepts.md",
    out: "concepts.md",
    title: "Concepts",
    sidebarLabel: "Concepts",
    sidebarOrder: 1,
    description: "The core OKF ideas okfctl works with: bundles, nodes, the link graph, and the reserved files.",
  },
  {
    src: "guides/authoring.md",
    out: "guides/authoring.md",
    title: "Starting and authoring a bundle",
    sidebarLabel: "Authoring a bundle",
    sidebarOrder: 1,
  },
  {
    src: "guides/index-and-freshness.md",
    out: "guides/index-and-freshness.md",
    title: "Keeping the index current and fixing freshness drift",
    sidebarLabel: "Index & freshness",
    sidebarOrder: 2,
  },
  {
    src: "guides/curation-health.md",
    out: "guides/curation-health.md",
    title: "Curation health: lint, analyze, and --strict in CI",
    sidebarLabel: "Curation health",
    sidebarOrder: 3,
  },
  {
    src: "guides/search.md",
    out: "guides/search.md",
    title: "Search: core lexical/graph and the semantic plugin",
    sidebarLabel: "Search",
    sidebarOrder: 4,
  },
  {
    src: "guides/migrating.md",
    out: "guides/migrating.md",
    title: "Migrating a v0.1 bundle to v0.2",
    sidebarLabel: "Migrating",
    sidebarOrder: 5,
  },
  {
    src: "guides/remote-sources.md",
    out: "guides/remote-sources.md",
    title: "Remote sources: registry and connect",
    sidebarLabel: "Remote sources",
    sidebarOrder: 6,
  },
  {
    src: "guides/plugins.md",
    out: "guides/plugins.md",
    title: "Extending okfctl with plugins",
    sidebarLabel: "Plugins",
    sidebarOrder: 7,
  },
  {
    // THE anti-drift page. Consumed verbatim from the CI-proven generated file.
    src: "commands/README.md",
    out: "reference/commands.md",
    title: "Command reference",
    sidebarLabel: "Command reference",
    sidebarOrder: 1,
    description: "Every okfctl command, its flags, and a runnable example — rendered from the generated command reference that CI keeps in sync with the binary.",
  },
];

// slugFor derives the clean, published route from the on-disk `out` path by
// dropping the trailing ".md". This is the single source of truth for a page's
// URL: prepare-content stamps it into frontmatter, and astro.config imports the
// same mapping to build the legacy-path redirects, so the two cannot drift.
export function slugFor(out) {
  return out.replace(/\.md$/, "");
}

// LEGACY_TO_CLEAN maps each old _generated/* route (paths people may already
// have shared) to its clean replacement. Consumed by astro.config.mjs to emit
// redirect stubs so a shared /_generated/... link never 404s.
export const LEGACY_TO_CLEAN = Object.fromEntries(
  pages.map((p) => {
    const clean = slugFor(p.out);
    return [`/_generated/${clean}`, `/${clean}`];
  }),
);

async function main() {
  // Start clean so a removed/renamed source cannot leave a stale page behind.
  await rm(genRoot, { recursive: true, force: true });
  await mkdir(genRoot, { recursive: true });

  for (const page of pages) {
    const srcPath = join(docsRoot, page.src);
    if (!existsSync(srcPath)) {
      throw new Error(
        `prepare-content: source doc not found: ${srcPath}. ` +
          `This step only copies existing docs; it never invents content.`,
      );
    }
    const body = await readFile(srcPath, "utf8");
    const outPath = join(genRoot, page.out);
    await mkdir(dirname(outPath), { recursive: true });
    // Publish at the clean slug derived from `out`; the file stays under the
    // private _generated/ staging dir but its URL drops that prefix.
    const slug = slugFor(page.out);
    await writeFile(outPath, withFrontmatter({ ...page, slug }, body), "utf8");
    console.log(`prepared ${page.src} -> _generated/${page.out} (published at /${slug}/)`);
  }

  console.log(`prepare-content: wrote ${pages.length} pages to ${genRoot}`);
}

// Only run the copy step when invoked directly (`node prepare-content.mjs`).
// astro.config.mjs imports this module for LEGACY_TO_CLEAN and must not trigger
// a rebuild as a side effect of the import.
if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
