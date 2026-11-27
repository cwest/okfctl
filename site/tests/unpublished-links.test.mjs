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

// unpublished-links.test.mjs proves every reader-facing doc only links to docs
// the site actually publishes.
//
// prepare-content.mjs holds the list of docs okfctl.dev renders. A doc missing
// from that list exists only on GitHub, and nothing failed when a published doc
// linked to it: docs/README.md sent new readers to install.md first while
// https://okfctl.dev/install/ returned 404. This test walks the relative
// Markdown links in every published doc (plus docs/README.md, the docs index)
// and fails when one points at a docs/ Markdown file the site does not publish.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, posix } from "node:path";
import { fileURLToPath } from "node:url";

import { pages } from "../scripts/prepare-content.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const docsRoot = join(here, "..", "..", "docs");

// Docs that are deliberately GitHub-only: internal design records, not
// reader-facing pages. A link to one of these is intentional and never needs a
// page on the site. Paths are relative to docs/; a trailing "/" covers a tree.
const GITHUB_ONLY = ["PRD.md", "adr/", "plans/", "specs/"];

// The docs index is not rendered on the site (the homepage plays that role),
// but it is the page that points readers at everything else, so its links are
// checked too.
const EXTRA_SOURCES = ["README.md"];

// relativeDocLinks returns the relative Markdown link targets in a doc,
// skipping fenced code blocks, inline code, and HTML comments (example output
// such as `[tannin](tannin.md)` and the hidden transcript-setup blocks are
// illustration, not links that render), absolute and external
// URLs, and in-page anchors. Fragments are dropped from the returned target.
export function relativeDocLinks(markdown) {
  const prose = markdown
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/^(```|~~~)[^\n]*\n[\s\S]*?^\1[^\n]*$/gm, "")
    .replace(/`[^`\n]*`/g, "");
  const targets = [];
  for (const m of prose.matchAll(/\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)/g)) {
    const target = m[1];
    if (/^[a-z][a-z0-9+.-]*:/i.test(target) || target.startsWith("/") || target.startsWith("#")) {
      continue;
    }
    targets.push(target.replace(/#.*$/, ""));
  }
  return targets;
}

// unpublishedLinks checks the given sources against the published page list
// and returns one "docs/<from> -> <target> (...)" line per offending link.
export function unpublishedLinks(sources, publishedSrcs) {
  const published = new Set(publishedSrcs);
  const problems = [];
  for (const { src, markdown } of sources) {
    for (const target of relativeDocLinks(markdown)) {
      if (!target.endsWith(".md")) continue;
      const resolved = posix.normalize(posix.join(posix.dirname(src), target));
      if (resolved.startsWith("../")) continue; // outside docs/, not a site page
      if (published.has(resolved)) continue;
      if (GITHUB_ONLY.some((p) => (p.endsWith("/") ? resolved.startsWith(p) : resolved === p))) {
        continue;
      }
      const why = existsSync(join(docsRoot, resolved))
        ? "not published by site/scripts/prepare-content.mjs"
        : "no such file";
      problems.push(`docs/${src} -> ${target} (docs/${resolved}: ${why})`);
    }
  }
  return problems;
}

async function readSources() {
  const srcs = [...EXTRA_SOURCES, ...pages.map((p) => p.src)];
  return Promise.all(
    srcs.map(async (src) => ({ src, markdown: await readFile(join(docsRoot, src), "utf8") })),
  );
}

test("relativeDocLinks skips code, comments, external URLs, absolute paths, and anchors", () => {
  const md = [
    "Start with [Install](install.md) and [Search](guides/search.md#flags).",
    "See [spec](https://example.com/SPEC.md), [root](/wine/acidity.md), [here](#top).",
    "Inline `[tannin](tannin.md)` is an example.",
    "```text",
    "* [Oak](oak.md)",
    "```",
    "<!-- transcript: setup",
    "$ printf 'Sharpens [tannin](tannin.md).' > old/wine/acidity.md",
    "-->",
    "Back to [concepts](../concepts.md).",
  ].join("\n");
  assert.deepEqual(relativeDocLinks(md), ["install.md", "guides/search.md", "../concepts.md"]);
});

test("a published doc linking to an unpublished doc is reported (positive control)", async () => {
  const sources = await readSources();
  const withoutInstall = pages.map((p) => p.src).filter((s) => s !== "install.md");
  const problems = unpublishedLinks(sources, withoutInstall);
  assert.ok(
    problems.some((p) => p.startsWith("docs/README.md -> install.md ")),
    `expected docs/README.md -> install.md to be reported, got:\n${problems.join("\n")}`,
  );
});

test("GitHub-only docs (PRD, ADRs, plans, specs) do not trip the check", () => {
  const md = "[PRD](PRD.md) [ADRs](adr/README.md) [plan](plans/x.md) [spec](specs/y.md)";
  assert.deepEqual(unpublishedLinks([{ src: "README.md", markdown: md }], []), []);
});

test("every published doc links only to published or GitHub-only docs", async () => {
  const problems = unpublishedLinks(await readSources(), pages.map((p) => p.src));
  assert.deepEqual(
    problems,
    [],
    `these links point at docs okfctl.dev does not publish; add the target to the ` +
      `page list in site/scripts/prepare-content.mjs or to GITHUB_ONLY:\n${problems.join("\n")}`,
  );
});
