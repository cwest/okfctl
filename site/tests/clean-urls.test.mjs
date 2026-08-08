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

// clean-urls.test.mjs proves the site ships its docs at clean, permanent URLs
// (/concepts/, /guides/<slug>/, /reference/commands/) and NOT under the private
// _generated/ staging path that prepare-content.mjs writes to on disk.
//
// It asserts on the BUILT output in dist/ — the bytes GitHub Pages actually
// serves — because a link is only correct if it survives the build, not just
// the config. Run `npm run build` before this suite (see package.json test
// script, which chains them).

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir, stat } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const siteRoot = join(here, "..");
const dist = join(siteRoot, "dist");

// The clean, permanent routes each doc must publish at. These are the URLs that
// go into issues, blog posts, and the sitemap; they must never carry the
// _generated build-artifact prefix.
const CLEAN_ROUTES = [
  "concepts",
  "guides/authoring",
  "guides/index-and-freshness",
  "guides/curation-health",
  "guides/search",
  "guides/migrating",
  "guides/remote-sources",
  "guides/plugins",
  "reference/commands",
];

// The old _generated/* paths people may already have shared. They must keep
// working via a redirect, not 404.
const LEGACY_ROUTES = CLEAN_ROUTES.map((r) => `_generated/${r}`);

async function walk(dir) {
  const out = [];
  for (const ent of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, ent.name);
    if (ent.isDirectory()) out.push(...(await walk(p)));
    else out.push(p);
  }
  return out;
}

test("dist/ exists (build ran first)", () => {
  assert.ok(
    existsSync(dist),
    "dist/ not found — run `npm run build` before the test suite",
  );
});

test("every doc is served at its clean URL", async () => {
  for (const route of CLEAN_ROUTES) {
    const page = join(dist, route, "index.html");
    assert.ok(
      existsSync(page),
      `expected clean route /${route}/ to build to ${relative(siteRoot, page)}`,
    );
  }
});

test("no real doc is served under the _generated/ prefix (only redirect stubs)", async () => {
  const genDir = join(dist, "_generated");
  if (!existsSync(genDir)) return; // nothing under _generated/ at all is fine
  // Anything that remains under _generated/ must be a redirect stub, never a
  // full rendered doc page. A stub is a tiny meta-refresh document; a real doc
  // carries the Starlight chrome (sidebar/pagefind markup).
  const files = (await walk(genDir)).filter((f) => f.endsWith(".html"));
  for (const f of files) {
    const html = await readFile(f, "utf8");
    assert.match(
      html,
      /http-equiv=["']?refresh/i,
      `${relative(dist, f)} under _generated/ is a real page, not a redirect stub`,
    );
    assert.ok(
      !html.includes("data-has-sidebar"),
      `${relative(dist, f)} under _generated/ rendered full doc chrome — it should be a redirect stub`,
    );
  }
});

test("legacy _generated/* paths redirect to the clean URL (not 404)", async () => {
  for (let i = 0; i < LEGACY_ROUTES.length; i++) {
    const legacy = LEGACY_ROUTES[i];
    const clean = CLEAN_ROUTES[i];
    const page = join(dist, legacy, "index.html");
    assert.ok(
      existsSync(page),
      `expected redirect stub for legacy /${legacy}/ at ${relative(siteRoot, page)}`,
    );
    const html = await readFile(page, "utf8");
    assert.match(
      html,
      /http-equiv=["']?refresh/i,
      `legacy /${legacy}/ must be a redirect page (meta refresh)`,
    );
    assert.ok(
      html.includes(`/${clean}`),
      `legacy /${legacy}/ must redirect to the clean /${clean}/`,
    );
  }
});

test("sitemap lists ONLY clean URLs — no _generated path", async () => {
  const sitemap = join(dist, "sitemap-0.xml");
  assert.ok(existsSync(sitemap), "sitemap-0.xml not found in dist/");
  const xml = await readFile(sitemap, "utf8");
  assert.ok(
    !xml.includes("/_generated/"),
    "sitemap-0.xml contains a /_generated/ URL",
  );
  for (const route of CLEAN_ROUTES) {
    assert.ok(
      xml.includes(`https://okfctl.dev/${route}/`),
      `sitemap-0.xml is missing the clean URL https://okfctl.dev/${route}/`,
    );
  }
});

test("no published HTML links to a _generated path (redirect stubs excepted)", async () => {
  const files = (await walk(dist)).filter((f) => f.endsWith(".html"));
  const offenders = [];
  for (const f of files) {
    const rel = relative(dist, f);
    // Redirect stubs under _generated/ legitimately reference their own path.
    if (rel.startsWith("_generated/")) continue;
    const html = await readFile(f, "utf8");
    // Match an href/src pointing at the _generated build-artifact space.
    if (/(?:href|src)=["'][^"']*\/_generated\//.test(html)) {
      offenders.push(rel);
    }
  }
  assert.deepEqual(
    offenders,
    [],
    `these published pages still link to a _generated path: ${offenders.join(", ")}`,
  );
});
