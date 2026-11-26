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

// install-sh.test.mjs runs the public installer (public/install.sh) end to end
// against stubbed downloaders, so it never touches the network.
//
// The load-bearing case: the GitHub REST API answers 403 (the unauthenticated
// rate limit shared CI runners and NAT'd users hit), and the installer must
// STILL resolve the latest tag, from the github.com /releases/latest redirect,
// and install a checksum-verified archive. The stubs log every URL requested so
// the test can also prove the API is never called when the redirect works.

import { test } from "node:test";
import assert from "node:assert/strict";
import { spawnSync, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const installer = join(here, "..", "public", "install.sh");

const TAG = "v9.9.9";
const ARCHIVE = "okfctl_9.9.9_linux_amd64.tar.gz";

// Shared stub logic for curl and wget. Behavior is keyed on the URL:
//   api.github.com/...            -> 403 unless API_OK=1 (then a JSON tag_name)
//   github.com/.../releases/latest -> a 302 to /releases/tag/$TAG, unless
//                                    REDIRECT=none (then no Location at all)
//   .../releases/download/$TAG/*  -> the fixture file of that name
const stubCommon = `
log="$STUB_LOG"
fixtures="$STUB_FIXTURES"
api_json() { printf '{\\n  "tag_name": "${TAG}",\\n  "name": "x"\\n}\\n'; }
redirect_target() {
  [ "\${REDIRECT:-}" = "none" ] && return 0
  printf 'https://github.com/cwest/okfctl/releases/tag/${TAG}'
}
`;

const curlStub = `#!/bin/sh
${stubCommon}
out="" fmt="" url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    -w) fmt="$2"; shift ;;
    -H) printf 'HEADER %s\\n' "$2" >>"$log"; shift ;;
    http*) url="$1" ;;
  esac
  shift
done
printf 'curl %s\\n' "$url" >>"$log"
case "$url" in
  https://api.github.com/*)
    if [ "\${API_OK:-}" = "1" ]; then api_json; exit 0; fi
    echo "curl: (22) The requested URL returned error: 403" >&2; exit 22 ;;
  */releases/latest)
    case "$fmt" in *redirect_url*) redirect_target ;; esac
    exit 0 ;;
  */releases/download/${TAG}/*)
    cp "$fixtures/\${url##*/}" "$out" ;;
  *) echo "curl stub: unexpected $url" >&2; exit 22 ;;
esac
`;

const wgetStub = `#!/bin/sh
${stubCommon}
out="" url="" headers=""
while [ $# -gt 0 ]; do
  case "$1" in
    -O) out="$2"; shift ;;
    -qO) out="$2"; shift ;;
    -qO-) out="-" ;;
    -S) headers=1 ;;
    --header=*) printf 'HEADER %s\\n' "\${1#--header=}" >>"$log" ;;
    http*) url="$1" ;;
  esac
  shift
done
printf 'wget %s\\n' "$url" >>"$log"
case "$url" in
  https://api.github.com/*)
    if [ "\${API_OK:-}" = "1" ]; then api_json; exit 0; fi
    echo "  HTTP/1.1 403 Forbidden" >&2; exit 8 ;;
  */releases/latest)
    t="$(redirect_target)"
    if [ -n "$headers" ]; then
      echo "  HTTP/1.1 302 Found" >&2
      [ -n "$t" ] && printf '  Location: %s\\r\\n' "$t" >&2
      [ -n "$t" ] && printf 'Location: %s [following]\\n0 redirections exceeded.\\n' "$t" >&2
    fi
    exit 8 ;;
  */releases/download/${TAG}/*)
    cp "$fixtures/\${url##*/}" "$out" ;;
  *) echo "wget stub: unexpected $url" >&2; exit 8 ;;
esac
`;

const unameStub = `#!/bin/sh
case "$1" in -s) echo Linux ;; -m) echo x86_64 ;; *) echo Linux ;; esac
`;

function writeExe(path, body) {
  writeFileSync(path, body);
  chmodSync(path, 0o755);
}

// fixture builds a fake release (archive + checksums.txt) and a stub bin dir.
// downloader is "curl" or "wget". For wget the PATH is a curated dir of
// symlinks so the real curl is invisible and the installer takes its wget path.
function fixture(downloader) {
  const root = mkdtempSync(join(tmpdir(), "okfctl-install-"));
  const fixtures = join(root, "release");
  const stage = join(root, "stage");
  const bin = join(root, "bin");
  const dest = join(root, "dest");
  for (const d of [fixtures, stage, bin]) mkdirSync(d);

  writeExe(join(stage, "okfctl"), "#!/bin/sh\necho okfctl-fake\n");
  writeExe(join(stage, "okfctl-search"), "#!/bin/sh\necho search-fake\n");
  execFileSync("tar", ["-czf", join(fixtures, ARCHIVE), "-C", stage, "okfctl", "okfctl-search"]);
  const sum = createHash("sha256").update(readFileSync(join(fixtures, ARCHIVE))).digest("hex");
  writeFileSync(join(fixtures, "checksums.txt"), `${sum}  ${ARCHIVE}\n`);

  writeExe(join(bin, "uname"), unameStub);
  let path;
  if (downloader === "curl") {
    writeExe(join(bin, "curl"), curlStub);
    path = `${bin}:${process.env.PATH}`;
  } else {
    writeExe(join(bin, "wget"), wgetStub);
    const tools = ["sh", "tar", "gzip", "mktemp", "grep", "awk", "sed", "head", "tail", "tr",
      "chmod", "mv", "mkdir", "rm", "dirname", "cat", "cp", "printf",
      "sha256sum", "shasum", "perl"];
    for (const t of tools) {
      const r = spawnSync("sh", ["-c", `command -v ${t}`], { encoding: "utf8" });
      const p = r.stdout.trim();
      if (r.status === 0 && p.startsWith("/") && !existsSync(join(bin, t))) symlinkSync(p, join(bin, t));
    }
    path = bin;
  }
  const log = join(root, "requests.log");
  writeFileSync(log, "");
  return { root, dest, log, path, fixtures };
}

function runInstaller(fx, extraEnv = {}) {
  const env = {
    HOME: fx.root,
    PATH: fx.path,
    STUB_LOG: fx.log,
    STUB_FIXTURES: fx.fixtures,
    OKFCTL_BIN_DIR: fx.dest,
    ...extraEnv,
  };
  const r = spawnSync("sh", [installer], { env, encoding: "utf8" });
  return { ...r, requests: readFileSync(fx.log, "utf8") };
}

for (const downloader of ["curl", "wget"]) {
  test(`${downloader}: resolves latest from the releases redirect when the API is 403`, () => {
    const fx = fixture(downloader);
    const r = runInstaller(fx);
    assert.equal(r.status, 0, `installer failed:\n${r.stderr}`);
    assert.match(r.stderr, new RegExp(`installing okfctl ${TAG} for linux/amd64`));
    assert.match(r.stderr, /checksum verified/);
    assert.ok(existsSync(join(fx.dest, "okfctl")), "okfctl not installed");
    assert.ok(existsSync(join(fx.dest, "okfctl-search")), "okfctl-search not installed");
    assert.match(r.requests, /github\.com\/cwest\/okfctl\/releases\/latest/);
    assert.doesNotMatch(r.requests, /api\.github\.com/, "redirect worked, API must not be called");
  });

  test(`${downloader}: falls back to the API when the redirect carries no tag`, () => {
    const fx = fixture(downloader);
    const r = runInstaller(fx, { REDIRECT: "none", API_OK: "1" });
    assert.equal(r.status, 0, `installer failed:\n${r.stderr}`);
    assert.match(r.stderr, new RegExp(`installing okfctl ${TAG}`));
    assert.match(r.requests, /api\.github\.com\/repos\/cwest\/okfctl\/releases\/latest/);
  });

  test(`${downloader}: API fallback sends GITHUB_TOKEN when set`, () => {
    const fx = fixture(downloader);
    const r = runInstaller(fx, { REDIRECT: "none", API_OK: "1", GITHUB_TOKEN: "tok123" });
    assert.equal(r.status, 0, `installer failed:\n${r.stderr}`);
    assert.match(r.requests, /HEADER Authorization: Bearer tok123/);
  });

  test(`${downloader}: no redirect tag and API 403 fails with the OKFCTL_VERSION hint`, () => {
    const fx = fixture(downloader);
    const r = runInstaller(fx, { REDIRECT: "none" });
    assert.notEqual(r.status, 0);
    assert.match(r.stderr, /could not determine the latest version; set OKFCTL_VERSION/);
    assert.ok(!existsSync(join(fx.dest, "okfctl")));
  });
}

test("OKFCTL_VERSION skips resolution entirely", () => {
  const fx = fixture("curl");
  const r = runInstaller(fx, { OKFCTL_VERSION: TAG });
  assert.equal(r.status, 0, `installer failed:\n${r.stderr}`);
  assert.doesNotMatch(r.requests, /releases\/latest/);
});

test("installer parses under sh -n", () => {
  const r = spawnSync("sh", ["-n", installer], { encoding: "utf8" });
  assert.equal(r.status, 0, r.stderr);
});
