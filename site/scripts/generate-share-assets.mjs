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

// generate-share-assets.mjs is a build-time step that produces the site's raster
// share assets from the brand tokens in brand.mjs, so they cannot drift from the
// theme the pages render:
//
//   public/og.png            1200x630  — the og:image / twitter:image social card
//   public/apple-touch-icon.png  180x180  — iOS home-screen icon
//   public/favicon.png           180x180  — PNG fallback for the SVG favicon
//
// The card is BUILT from tokens (an SVG string composed here), not a checked-in
// binary, then rasterized with @resvg/resvg-js — a pure prebuilt Rust binding,
// no CGO, no browser, no network. Runs in prebuild before astro build so the PNGs
// land in dist/. The three PNGs are generated artifacts (git-ignored); the SVG
// sources (favicon.svg here and the card SVG below) are the committed truth.

import { writeFile, mkdir, readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { Resvg } from "@resvg/resvg-js";
import { brand } from "./brand.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const siteRoot = join(here, "..");
const publicDir = join(siteRoot, "public");

// XML-escape text we interpolate into the SVG. Tokens are ours, but escape
// anyway so a future token with an & or < can never emit broken SVG.
function xml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// The 1200x630 social card, composed from brand tokens. A dark indigo surface,
// the same CLI-prompt mark as the favicon, the wordmark, and the tagline —
// the shape a link unfurls to on Bluesky / X / LinkedIn / Slack / Discord.
function cardSvg() {
  const W = 1200;
  const H = 630;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="${brand.accentDark}"/>
      <stop offset="1" stop-color="#0e0c2e"/>
    </linearGradient>
  </defs>
  <rect width="${W}" height="${H}" fill="url(#bg)"/>
  <rect x="0" y="0" width="${W}" height="8" fill="${brand.accent}"/>

  <!-- Mark: the CLI prompt glyph, matching the favicon. -->
  <g transform="translate(96 150)">
    <rect width="120" height="120" rx="26" fill="${brand.accentDark}"/>
    <rect x="4" y="4" width="112" height="112" rx="22" fill="none" stroke="${brand.accent}" stroke-width="5"/>
    <path d="M30 42 L56 60 L30 78" fill="none" stroke="${brand.accentLight}" stroke-width="10" stroke-linecap="round" stroke-linejoin="round"/>
    <line x1="64" y1="80" x2="90" y2="80" stroke="${brand.accent}" stroke-width="10" stroke-linecap="round"/>
  </g>

  <!-- Wordmark -->
  <text x="248" y="245" font-family="${xml(brand.mono)}" font-size="132" font-weight="700" fill="${brand.ink}">${xml(brand.wordmark)}</text>

  <!-- Tagline -->
  <text x="98" y="360" font-family="${xml(brand.sans)}" font-size="42" font-weight="500" fill="${brand.inkMuted}">${xml(brand.tagline)}</text>
  <text x="98" y="418" font-family="${xml(brand.sans)}" font-size="42" font-weight="500" fill="${brand.inkMuted}">Pure Go — no CGO, no model runtime.</text>

  <!-- Footer: domain + spec posture -->
  <text x="98" y="560" font-family="${xml(brand.mono)}" font-size="34" font-weight="600" fill="${brand.accentLight}">${xml(brand.domain)}</text>
  <text x="${W - 98}" y="560" text-anchor="end" font-family="${xml(brand.sans)}" font-size="30" font-weight="500" fill="${brand.inkMuted}">A conformant consumer of the OKF spec</text>
</svg>`;
}

// Render an SVG string to a PNG buffer at a fixed pixel width.
function rasterize(svg, width) {
  const r = new Resvg(svg, {
    fitTo: { mode: "width", value: width },
    font: { loadSystemFonts: true },
  });
  return r.render().asPng();
}

async function main() {
  await mkdir(publicDir, { recursive: true });

  // 1) Social card.
  const card = cardSvg();
  await writeFile(join(publicDir, "og.png"), rasterize(card, 1200));
  console.log("share-assets: wrote public/og.png (1200x630)");

  // 2) Icons, rasterized from the committed favicon.svg so the PNG fallback and
  //    apple-touch-icon are the exact same mark.
  const faviconSvg = await readFile(join(publicDir, "favicon.svg"), "utf8");
  const iconPng = rasterize(faviconSvg, 180);
  await writeFile(join(publicDir, "apple-touch-icon.png"), iconPng);
  await writeFile(join(publicDir, "favicon.png"), iconPng);
  console.log("share-assets: wrote public/apple-touch-icon.png + favicon.png (180x180)");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
