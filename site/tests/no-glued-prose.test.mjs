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

// no-glued-prose.test.mjs guards against Astro's compressHTML collapsing the
// newline that is the only separator between a prose text node and an adjacent
// inline element (<a>/<strong>/<em>). When that happens the rendered prose runs
// two words together — "from<a>releases</a>" ships as "fromreleases", "in<a>"
// as "inGoogleCloudPlatform", "it.</strong>The" as "it.The".
//
// The idiom that prevents it already lives in index.astro ({" "} between the
// text and the inline element). This test asserts on the BUILT output in dist/
// — the bytes GitHub Pages serves — so the class of bug cannot regress no matter
// which source line reintroduces it.
//
// It is deliberately NARROW. It flags a missing word-space ONLY where BOTH sides
// are prose carriers: a letter/'.' abutting an <a>/<strong>/<em> whose text
// starts with a letter, or a multi-letter </a>/</strong>/</em> abutting a
// letter. It stays silent on the legitimate adjacencies the site relies on:
//   - the wordmark        okfctl<span class="dot">.</span>dev   (span, not prose)
//   - arrow/tag badges    text<span class="arrow">→</span>      (span, not prose)
//   - acronym styling     <strong>T</strong>ransparency         (single letter)
//   - code word suffixes  <code>git clone</code>d               (code, not prose)
// The negative control below pins that silence — it is the load-bearing half.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const siteRoot = join(here, "..");
const dist = join(siteRoot, "dist");

// The prose-carrying inline elements. A run-together across one of these glues
// two real words; a span (wordmark/arrow/tag) or a code span does not.
const PROSE_EL = "(?:a|strong|em)";

// Shape A — a prose letter or sentence period immediately followed by an opening
// prose element whose text begins with a letter:  "from<a ...>releases".
const OPEN_GLUE = new RegExp(`[A-Za-z.]<${PROSE_EL}(?:\\s[^>]*)?>[A-Za-z]`, "g");

// Shape B — a closing prose element preceded by at least two non-tag characters
// (so single-letter acronym styling like <strong>T</strong> is excluded) and
// immediately followed by a prose letter:  "it.</strong>The".
const CLOSE_GLUE = new RegExp(`[^<>][^<>]</${PROSE_EL}>[A-Za-z]`, "g");

// Return every glued-prose boundary in one document, with a little context so a
// failure names the exact site to fix.
function gluedBoundaries(html) {
  const out = [];
  for (const re of [OPEN_GLUE, CLOSE_GLUE]) {
    re.lastIndex = 0;
    let m;
    while ((m = re.exec(html)) !== null) {
      out.push(html.slice(Math.max(0, m.index - 25), m.index + m[0].length + 15));
    }
  }
  return out;
}

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

test("no published page glues two prose words at an inline-element boundary", async () => {
  const files = (await walk(dist)).filter((f) => f.endsWith(".html"));
  const offenders = [];
  for (const f of files) {
    // Redirect stubs are machine documents, not prose.
    const html = await readFile(f, "utf8");
    if (/http-equiv=["']?refresh/i.test(html)) continue;
    for (const ctx of gluedBoundaries(html)) {
      offenders.push(`${relative(dist, f)}: …${ctx.replace(/\s+/g, " ")}…`);
    }
  }
  assert.deepEqual(
    offenders,
    [],
    `these built pages run two prose words together at an inline boundary ` +
      `(add {" "} in the .astro source between the text and the inline element):\n  ` +
      offenders.join("\n  "),
  );
});

// POSITIVE CONTROL — the detector must BITE on a genuinely glued fixture. If this
// ever passes silently the guard above has gone blind and the whole test is
// theatre.
test("positive control: detector fires on glued prose", () => {
  const glued = [
    `<p>Grab a binary from<a href="/releases">releases</a>.</p>`,
    `<p>maintained by Google in<a href="/x">GoogleCloudPlatform</a>.</p>`,
    `<p>does not author it.</strong>The format is</p>`,
  ];
  for (const html of glued) {
    assert.ok(
      gluedBoundaries(html).length > 0,
      `detector failed to flag glued prose: ${html}`,
    );
  }
});

// NEGATIVE CONTROL — the load-bearing half. Every one of these is a legitimate
// adjacency the live site ships; the detector MUST stay silent or it forces
// authors to insert spaces that would visibly break the wordmark, the acronym
// styling, the arrow rows, or "git cloned".
test("negative control: detector stays silent on legitimate adjacencies", () => {
  const legit = [
    // wordmark — dot and dev are meant to touch (span, not a prose element)
    `<a class="brand">okfctl<span class="dot">.</span>dev</a>`,
    // arrow badge after link text
    `<a href="/x">Concepts<span class="arrow">→</span></a>`,
    // tag badge after a filename in the tree
    `<span>tannin.md<span class="tag">Reference</span></span>`,
    // acronym styling — single-letter emphasis continues the word
    `<p><strong>T</strong>ransparency, <strong>A</strong>ccuracy</p>`,
    // code span carrying a suffixed word: renders "git cloned"
    `<li>is <code dir="auto">git clone</code>d.</li>`,
    // punctuation correctly abutting a closing link — no word runs together
    `<a href="/x">releases</a>.`,
    // an opening link that a proper {" "} already separated
    `<p>from <a href="/x">releases</a>.</p>`,
  ];
  for (const html of legit) {
    assert.deepEqual(
      gluedBoundaries(html),
      [],
      `detector false-positived on a legitimate adjacency: ${html}`,
    );
  }
});
