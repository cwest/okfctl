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

// author-credit.test.mjs proves the site carries an author credit for Casey
// West, linking to caseywest.com, on EVERY indexable page — the homepage AND
// the Starlight docs pages.
//
// It exists because the credit reaches pages by two different routes that break
// independently: the homepage is a hand-written Astro route (src/pages/
// index.astro) that carries its own footer, while every docs page inherits a
// Starlight footer component override (src/components/Footer.astro wired via
// astro.config.mjs `components.Footer`). A regression on either route is
// invisible in the build, the linter, and a diff of the markup — the only place
// it is observable is the built HTML, so that is what this asserts on.
//
// Run `npm run build` before this suite (see package.json test script, which
// chains them).

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir, stat } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const dist = join(here, "..", "dist");

// The external destination the credit must link to. Kept separate from the copy
// assertion so a copy edit that keeps the link, or a link change that keeps the
// copy, both fail loudly.
const CREDIT_HREF = "https://caseywest.com/";

async function walk(dir) {
  const out = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...(await walk(full)));
    else out.push(full);
  }
  return out;
}

// Redirect stubs are a meta-refresh and nothing else; they are never shared or
// indexed, so requiring a footer credit on them would be wrong. Detect them by
// their meta-refresh signature (matches the social-tags suite's convention).
function isIndexable(rel, html) {
  if (!rel.endsWith(".html")) return false;
  if (rel.startsWith("_generated/")) return false;
  if (/http-equiv=["']?refresh/i.test(html)) return false;
  return true;
}

// The credit's link is present on a page when there is an anchor to
// caseywest.com that also carries rel="noopener" (external-link hygiene the
// card requires, consistent with the other external anchors in the footer).
function hasCreditLink(html) {
  const anchors = html.match(/<a\b[^>]*>/gi) ?? [];
  return anchors.some(
    (a) =>
      /href=["']https:\/\/caseywest\.com\/?["']/i.test(a) &&
      /rel=["'][^"']*noopener[^"']*["']/i.test(a),
  );
}

// The visible copy names Casey West, so a link with empty or wrong anchor text
// still fails.
function hasCreditCopy(html) {
  return /Casey West/.test(html);
}

test("dist/ exists (build ran first)", () => {
  assert.ok(existsSync(dist), "dist/ is missing — run `npm run build` first");
});

test("every indexable page carries the author credit linking to caseywest.com", async () => {
  const htmlFiles = (await walk(dist))
    .map((f) => relative(dist, f))
    .filter((rel) => rel.endsWith(".html"))
    .sort();

  const pages = [];
  for (const rel of htmlFiles) {
    const html = await readFile(join(dist, rel), "utf8");
    if (isIndexable(rel, html)) pages.push({ rel, html });
  }

  assert.ok(pages.length > 0, "found no indexable pages in dist/");

  const failures = [];
  for (const { rel, html } of pages) {
    if (!hasCreditLink(html))
      failures.push(`${rel} is missing the credit link to ${CREDIT_HREF} (with rel=noopener)`);
    if (!hasCreditCopy(html)) failures.push(`${rel} is missing the "Casey West" credit copy`);
  }

  assert.deepEqual(failures, [], `\n${failures.join("\n")}\n`);
});

test("the homepage carries the author credit", async () => {
  // Called out separately because the homepage carries its own footer, distinct
  // from the Starlight component override every docs page inherits — a failure
  // here means the standalone route regressed, not the shared component.
  const html = await readFile(join(dist, "index.html"), "utf8");
  assert.ok(hasCreditLink(html), "homepage is missing the credit link to caseywest.com");
  assert.ok(hasCreditCopy(html), 'homepage is missing the "Casey West" credit copy');
});

test("the homepage carries the credit exactly once (no duplicate)", async () => {
  // The docs footer is a Starlight override; the homepage is standalone. A
  // common mistake is to also render the override on the homepage, doubling the
  // credit. The homepage must show it once.
  const html = await readFile(join(dist, "index.html"), "utf8");
  const anchorCount = (html.match(/href=["']https:\/\/caseywest\.com\/?["']/gi) ?? []).length;
  assert.equal(
    anchorCount,
    1,
    `expected exactly 1 caseywest.com link on the homepage, found ${anchorCount}`,
  );
});

test("a docs page carries the author credit (Starlight footer override)", async () => {
  // /concepts/ and /reference/commands/ are the two pages the card names as the
  // must-verify docs routes. They reach the credit via the Starlight Footer
  // override, a different path than the homepage's own footer.
  for (const route of ["concepts", join("reference", "commands")]) {
    const file = join(dist, route, "index.html");
    assert.ok(existsSync(file), `${route}/index.html is missing from dist/`);
    const html = await readFile(file, "utf8");
    assert.ok(hasCreditLink(html), `${route}/ is missing the credit link to caseywest.com`);
    assert.ok(hasCreditCopy(html), `${route}/ is missing the "Casey West" credit copy`);
  }
});
