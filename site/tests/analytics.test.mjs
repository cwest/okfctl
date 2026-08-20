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

// analytics.test.mjs proves the Google Analytics 4 tag is emitted on BOTH site
// render paths under a production build with a configured measurement id, and on
// NEITHER path when the id is unset — the load-bearing gate that keeps local
// `astro dev` and secret-less fork builds clean.
//
// The two render paths are the same trap social-tags.test.mjs guards: docs pages
// inherit the tag from the `head` array in astro.config.mjs, while the homepage
// is a standalone Astro route (src/pages/index.astro) that Starlight never
// renders and must carry the tag itself. Instrumenting one and forgetting the
// other produces numbers that look fine and are wrong. Neither omission breaks
// the build or reddens a linter, so the only place it is observable is the built
// HTML — that is what this asserts on.
//
// Both controls are proven here, per AGENTS.md:
//   - Positive: a fresh production build WITH PUBLIC_GA_MEASUREMENT_ID set emits
//     gtag on the homepage, a docs page, and 404.
//   - Negative (load-bearing): the default suite build (run by `npm test` with
//     the env var unset) emits gtag on ZERO pages.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir, stat, mkdtemp, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { tmpdir } from "node:os";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const execFileP = promisify(execFile);
const here = dirname(fileURLToPath(import.meta.url));
const siteRoot = join(here, "..");
const dist = join(siteRoot, "dist");

// The two-line gtag.js snippet leaves an unambiguous fingerprint no other tag on
// the site produces: the loader src and the gtag() bootstrap call.
const GTAG_LOADER = /googletagmanager\.com\/gtag\/js/;
const GTAG_CALL = /gtag\('config'/;

const DUMMY_ID = "G-TESTONLY0";

async function walk(dir) {
  const out = [];
  for (const entry of await readdir(dir)) {
    const full = join(dir, entry);
    if ((await stat(full)).isDirectory()) out.push(...(await walk(full)));
    else out.push(full);
  }
  return out;
}

function countGtagPages(root, files) {
  let n = 0;
  for (const { html } of files) if (GTAG_LOADER.test(html) || GTAG_CALL.test(html)) n++;
  return n;
}

async function readHtml(root) {
  const files = (await walk(root))
    .map((f) => relative(root, f))
    .filter((rel) => rel.endsWith(".html"));
  const out = [];
  for (const rel of files) out.push({ rel, html: await readFile(join(root, rel), "utf8") });
  return out;
}

// --- Negative control (load-bearing): the default suite build has NO id set. ---

test("dist/ exists (build ran first)", () => {
  assert.ok(existsSync(dist), "dist/ is missing — run `npm run build` first");
});

test("negative control: with PUBLIC_GA_MEASUREMENT_ID unset, ZERO pages carry the gtag tag", async () => {
  const pages = await readHtml(dist);
  assert.ok(pages.length > 0, "found no HTML pages in dist/");
  const tagged = pages
    .filter(({ html }) => GTAG_LOADER.test(html) || GTAG_CALL.test(html))
    .map((p) => p.rel);
  assert.deepEqual(
    tagged,
    [],
    `expected no gtag tag in a secret-less build, found it on: ${tagged.join(", ")}`,
  );
});

// --- Positive control: a fresh production build WITH the id set. ---

test("positive control: with PUBLIC_GA_MEASUREMENT_ID set, the homepage, a docs page, and 404 all carry the gtag tag", async () => {
  const outDir = await mkdtemp(join(tmpdir(), "okfctl-ga-"));
  try {
    await execFileP("npx", ["astro", "build", "--outDir", outDir], {
      cwd: siteRoot,
      env: { ...process.env, PUBLIC_GA_MEASUREMENT_ID: DUMMY_ID },
      maxBuffer: 64 * 1024 * 1024,
    });

    const required = ["index.html", join("concepts", "index.html"), "404.html"];
    for (const rel of required) {
      const p = join(outDir, rel);
      assert.ok(existsSync(p), `expected ${rel} in the production build`);
      const html = await readFile(p, "utf8");
      assert.ok(GTAG_LOADER.test(html), `${rel} is missing the gtag.js loader`);
      assert.ok(GTAG_CALL.test(html), `${rel} is missing the gtag('config', …) call`);
      assert.ok(html.includes(DUMMY_ID), `${rel} does not carry the configured measurement id`);
    }
  } finally {
    await rm(outDir, { recursive: true, force: true });
  }
});
