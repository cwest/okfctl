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

// version.ts DERIVES the latest okfctl release version at build time. No version
// string is ever hand-typed into the site — the install snippet and any badge
// read whatever this returns. The single source of truth is the GitHub Releases
// API for cwest/okfctl.
//
// This runs during the SSG build (Node), never in the browser.

const RELEASES_API =
  "https://api.github.com/repos/cwest/okfctl/releases/latest";

export interface OkfctlVersion {
  // The tag as published (e.g. a leading-"v" semver tag). `null` when no release
  // exists yet or the API could not be reached at build time.
  tag: string | null;
  // The tag with a leading "v" stripped — the form the tarball filenames use.
  // `null` under the same conditions as `tag`.
  bare: string | null;
}

/**
 * Fetch the latest published release tag from GitHub at build time.
 *
 * Degrades gracefully: if the repo has no releases yet, or the API is
 * unreachable / rate-limited during the build, this returns nulls rather than
 * failing the build. Callers render a sensible fallback (e.g. `@latest`) in that
 * case. This keeps the site buildable from a clean checkout before the first
 * release is cut.
 */
export async function fetchLatestVersion(): Promise<OkfctlVersion> {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "User-Agent": "okfctl-site-build",
  };
  // CI provides GITHUB_TOKEN; using it raises the rate limit but is optional.
  const token = process.env.GITHUB_TOKEN;
  if (token) headers.Authorization = `Bearer ${token}`;

  try {
    const res = await fetch(RELEASES_API, { headers });
    if (!res.ok) {
      console.warn(
        `version: GitHub releases API returned ${res.status}; ` +
          "falling back to unversioned install snippet.",
      );
      return { tag: null, bare: null };
    }
    const data = (await res.json()) as { tag_name?: string };
    const tag = typeof data.tag_name === "string" ? data.tag_name : null;
    if (!tag) return { tag: null, bare: null };
    return { tag, bare: tag.replace(/^v/, "") };
  } catch (err) {
    console.warn(
      `version: could not reach GitHub releases API (${String(err)}); ` +
        "falling back to unversioned install snippet.",
    );
    return { tag: null, bare: null };
  }
}
