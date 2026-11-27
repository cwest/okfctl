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

// page-titles.test.mjs proves every published docs page's title matches the
// first H1 of the doc it is rendered from.
//
// prepare-content.mjs drops the source doc's leading H1 and renders its own
// hard-coded `title` in its place (as <title>, og:title, and the page <h1>).
// That title is a hand-copied second copy of the heading, so it drifts the
// moment someone edits the doc: docs/guides/migrating.md was retitled for OKF
// v0.3 while the site kept publishing "Migrating a v0.1 bundle to v0.2" above
// the new body. Nothing else catches this: the build passes and the body is
// correct.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { pages } from "../scripts/prepare-content.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const docsRoot = join(here, "..", "..", "docs");

// A site title may differ from its source H1 only when listed here, with the
// reason. Keep this list short: every entry is a title nobody re-checks.
const ALLOWED_DIVERGENCE = {
  // The doc's H1 is a bare "Search"; in a browser tab, a search result, or a
  // shared link the page needs to say what it covers.
  "guides/search.md": "Search: core lexical/graph and the semantic plugin",
};

// firstH1 returns the doc's first top-level heading with inline-code
// backticks stripped, since a <title> renders them as literal characters.
export function firstH1(markdown) {
  const m = markdown.replace(/^\uFEFF/, "").match(/^# (.+)$/m);
  return m ? m[1].replace(/`/g, "").trim() : null;
}

test("firstH1 strips inline code and finds the first heading", () => {
  assert.equal(firstH1("# Keeping `index.md` current\n\n# Later\n"), "Keeping index.md current");
  assert.equal(firstH1("intro\n## Sub\n# Real\n"), "Real");
  assert.equal(firstH1("no heading\n"), null);
});

for (const page of pages) {
  test(`site title for ${page.src} matches its source H1`, async () => {
    const h1 = firstH1(await readFile(join(docsRoot, page.src), "utf8"));
    assert.ok(h1, `${page.src} has no top-level "# " heading`);
    if (page.src in ALLOWED_DIVERGENCE) {
      assert.equal(
        page.title,
        ALLOWED_DIVERGENCE[page.src],
        `${page.src} is allow-listed for a divergent title; update the allow-entry if the title changed`,
      );
      return;
    }
    assert.equal(
      page.title,
      h1,
      `site/scripts/prepare-content.mjs title for ${page.src} drifted from the doc's H1`,
    );
  });
}

test("every allow-listed divergence names a published page", () => {
  const published = new Set(pages.map((p) => p.src));
  for (const src of Object.keys(ALLOWED_DIVERGENCE)) {
    assert.ok(published.has(src), `ALLOWED_DIVERGENCE entry ${src} is not a published page`);
  }
});
