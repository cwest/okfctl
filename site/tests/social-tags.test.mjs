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

// social-tags.test.mjs proves every indexable page carries the social share
// card and icon tags.
//
// This exists because the tags reach pages by two different routes that are
// easy to break independently: docs pages inherit them from the `head` array in
// astro.config.mjs, while the homepage is a standalone Astro route that
// Starlight never renders and must carry them itself. Both have silently
// regressed — once when a merge left two `head:` keys on the starlight() object
// (the later one wins in a JS object literal, dropping the tags from every docs
// page), and once when the homepage moved from a Starlight-rendered index.mdx
// to a hand-written index.astro that didn't reproduce them.
//
// Neither failure breaks the build, reddens a linter, or changes a visible
// pixel. The only place they are observable is the built HTML, so that is what
// this asserts on.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir, stat } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, "..", "dist");

// Every tag below must appear on every indexable page. og:image and
// twitter:image are what make a shared link unfurl as a card instead of a bare
// URL; the icon tags are what a browser tab and an iOS home screen use.
const REQUIRED_TAGS = [
  { name: "og:image", pattern: /property="og:image"/ },
  { name: "og:image:width", pattern: /property="og:image:width"/ },
  { name: "og:image:height", pattern: /property="og:image:height"/ },
  { name: "og:image:alt", pattern: /property="og:image:alt"/ },
  { name: "twitter:card", pattern: /name="twitter:card"/ },
  { name: "twitter:image", pattern: /name="twitter:image"/ },
  { name: "apple-touch-icon", pattern: /rel="apple-touch-icon"/ },
  { name: "favicon.png", pattern: /href="\/favicon\.png"/ },
];

async function walk(dir) {
  const out = [];
  for (const entry of await readdir(dir)) {
    const full = join(dir, entry);
    if ((await stat(full)).isDirectory()) out.push(...(await walk(full)));
    else out.push(full);
  }
  return out;
}

// Redirect stubs under _generated/ are intentionally minimal: a meta-refresh
// and a noindex, nothing else. They are never shared or indexed, so requiring
// social tags on them would be wrong.
function isIndexable(rel) {
  return rel.endsWith(".html") && !rel.startsWith("_generated/");
}

test("dist/ exists (build ran first)", () => {
  assert.ok(existsSync(dist), "dist/ is missing — run `npm run build` first");
});

test("every indexable page carries the social card and icon tags", async () => {
  const pages = (await walk(dist))
    .map((f) => relative(dist, f))
    .filter(isIndexable)
    .sort();

  assert.ok(pages.length > 0, "found no indexable pages in dist/");

  const failures = [];
  for (const rel of pages) {
    const html = await readFile(join(dist, rel), "utf8");
    const missing = REQUIRED_TAGS.filter((t) => !t.pattern.test(html)).map((t) => t.name);
    if (missing.length) failures.push(`${rel} is missing: ${missing.join(", ")}`);
  }

  assert.deepEqual(failures, [], `\n${failures.join("\n")}\n`);
});

test("the homepage carries the social card (it renders outside Starlight)", async () => {
  // Called out separately from the sweep above because the homepage is the
  // most-shared URL on the site and reaches its tags by a different path than
  // every other page — a sweep failure here means something quite different
  // than a docs-page failure, and the distinction is worth preserving in the
  // test name a future reader sees.
  const html = await readFile(join(dist, "index.html"), "utf8");
  const missing = REQUIRED_TAGS.filter((t) => !t.pattern.test(html)).map((t) => t.name);
  assert.deepEqual(missing, [], `the homepage is missing: ${missing.join(", ")}`);
});

test("the starlight() config declares exactly one head key", async () => {
  // A duplicate `head:` on the same object literal is valid JS that silently
  // discards the earlier value, so it survives the build, the linter, and code
  // review. The built-HTML assertions above catch the consequence; this catches
  // the cause and names it, so the next person to hit it reads a sentence
  // instead of a diff of missing meta tags.
  const config = await readFile(join(here, "..", "astro.config.mjs"), "utf8");
  const heads = config.match(/^\s*head:\s*\[/gm) ?? [];
  assert.equal(
    heads.length,
    1,
    `expected exactly 1 \`head:\` key in astro.config.mjs, found ${heads.length} — ` +
      `a duplicate key keeps only the last, silently dropping the earlier tags`,
  );
});
