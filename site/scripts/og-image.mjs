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

// og-image.mjs is the one source of the social card's public URL, shared by
// both <head> render paths (the Starlight `head` array in astro.config.mjs and
// the standalone src/pages/index.astro).
//
// Link-preview crawlers (Discord, Slack, LinkedIn, X) cache the card image by
// URL, so a regenerated card at an unchanged URL keeps unfurling as the old one.
// The URL therefore carries a version: a short sha256 of the og.png bytes,
// recorded by generate-share-assets.mjs when it writes the PNG. A query string
// rather than a renamed file keeps /og.png stable for anything that already
// links it and keeps public/ from collecting one PNG per card revision.

import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const siteRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

export const OG_IMAGE_BASE = "https://okfctl.dev/og.png";
export const OG_HASH_LEN = 10;
export const ogVersionFile = join(siteRoot, ".share-assets", "og-version.json");

// ogHash returns the version string for a card's PNG bytes.
export function ogHash(bytes) {
  return createHash("sha256").update(bytes).digest("hex").slice(0, OG_HASH_LEN);
}

// versionedOgUrl builds the card URL from a recorded hash. It throws on a
// missing or malformed hash rather than falling back to the bare URL: a silent
// fallback is exactly the unversioned link this module exists to prevent.
export function versionedOgUrl(hash) {
  if (typeof hash !== "string" || !/^[0-9a-f]+$/.test(hash)) {
    throw new Error(
      `og-version.json has no valid hash (${JSON.stringify(hash)}); run ` +
        "`node scripts/generate-share-assets.mjs` (npm run prebuild) first",
    );
  }
  return `${OG_IMAGE_BASE}?v=${hash}`;
}

// ogImageUrl reads the recorded hash from disk, for astro.config.mjs (which
// Node loads directly). The homepage imports og-version.json through Vite
// instead: Vite bundles .astro routes into dist/.prerender/, where a path
// derived from this module's import.meta.url no longer points at the site.
export function ogImageUrl() {
  let raw;
  try {
    raw = readFileSync(ogVersionFile, "utf8");
  } catch {
    throw new Error(
      `${ogVersionFile} is missing; run \`node scripts/generate-share-assets.mjs\` (npm run prebuild) first`,
    );
  }
  return versionedOgUrl(JSON.parse(raw).hash);
}
