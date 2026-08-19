#!/usr/bin/env bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Gate a release on a real changelog entry.
#
# WHY THIS EXISTS. okfctl tagged v0.3.0 and v0.3.1 with NO CHANGELOG entry at
# all, and shipped the entire distribution story parked under `## [Unreleased]`.
# An empty release is a defect a human caught after the fact, not something CI
# stopped. This gate makes an empty release impossible: it runs in the release
# workflow BEFORE goreleaser, so a bad tag fails the build rather than
# publishing and being reported afterward.
#
# WHAT IT ASSERTS for the version being released (derived from the pushed tag):
#   1. a `## [X.Y.Z] - <date>` heading exists for that exact version;
#   2. the heading is DATED (a heading with no date fails);
#   3. the section is NON-EMPTY: at least one `### <subsection>` with a real
#      content line under it before the next `## [` heading. A heading with
#      nothing under it, or with empty `###` subsections only, FAILS.
#
# Both controls are exercised in the release workflow and in the offline test:
#   positive control  — a tag with no/empty section MUST fail (the v0.3.0
#                        pre-fix state);
#   negative control  — the reconciled section MUST pass.
#
# USAGE:
#   check-changelog.sh <version> [changelog-path]
#     <version>        e.g. v0.4.0 or 0.4.0 (a leading 'v' is stripped)
#     changelog-path   defaults to CHANGELOG.md
#
# Exit 0 when the section is present, dated, and non-empty; exit 1 otherwise.
# POSIX tools only (bash + awk) so it needs no extra runtime on the CI runner.

set -euo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: check-changelog.sh <version> [changelog-path]" >&2
  exit 2
fi

raw_version="$1"
changelog="${2:-CHANGELOG.md}"

# Strip a leading 'v' so a tag (v0.4.0) and a bare version (0.4.0) both work.
version="${raw_version#v}"

if [ ! -f "$changelog" ]; then
  echo "::error::changelog not found: $changelog" >&2
  exit 1
fi

# awk does the parsing: find the `## [<version>]` heading, confirm it carries a
# date, then confirm at least one `###` subsection has a real content line
# before the next `## [` heading. Exit status carries the verdict; awk prints a
# human-readable reason on failure.
awk -v version="$version" '
  BEGIN {
    # Match "## [X.Y.Z]" for exactly this version. The version is interpolated
    # into a regex, so escape every ERE metacharacter into a separate variable
    # first. Otherwise the dots in a version match any character and the gate
    # false-PASSes a mistyped heading (e.g. `0.4.0` would match `## [0X4X0]`).
    # SemVer can carry `.`, `-`, `+` and a build id, so escaping the full
    # metacharacter set (not just `.`) keeps the match exact for any legal
    # version string. Keep `version` itself unescaped for the error messages.
    version_re = version
    gsub(/[][(){}.^$*+?|\\]/, "\\\\&", version_re)
    heading_re = "^## \\[" version_re "\\]"
    found_heading = 0     # the version heading exists
    dated = 0             # the heading line carries a date after the "]"
    in_section = 0        # currently between this heading and the next "## ["
    has_content = 0       # a non-blank content line under some "###" subsection
    in_subsection = 0     # currently under a "###" subsection in this section
  }

  # A new top-level release heading. Entering ours starts the section; any other
  # "## [" heading after ours ends it.
  /^## \[/ {
    if (in_section) { in_section = 0 }   # next release heading closes our section
    if ($0 ~ heading_re) {
      found_heading = 1
      in_section = 1
      in_subsection = 0
      # Dated iff there is a "- <something>" after the "]" on the heading line.
      if ($0 ~ /\][[:space:]]*-[[:space:]]*[^[:space:]]/) { dated = 1 }
    }
    next
  }

  # Inside our section, track subsections and look for real content.
  in_section {
    if ($0 ~ /^### /) { in_subsection = 1; next }
    # A content line is any non-blank line that is not itself a heading. It
    # only counts once we are under a "###" subsection, so a heading with prose
    # but no subsection still fails the "at least one ### with content" rule.
    if (in_subsection && $0 ~ /[^[:space:]]/ && $0 !~ /^#/) {
      has_content = 1
    }
  }

  END {
    if (!found_heading) {
      printf("::error::no `## [%s]` section found in changelog\n", version) > "/dev/stderr"
      exit 1
    }
    if (!dated) {
      printf("::error::`## [%s]` heading is not dated (expected `## [%s] - YYYY-MM-DD`)\n", version, version) > "/dev/stderr"
      exit 1
    }
    if (!has_content) {
      printf("::error::`## [%s]` section is empty: it needs at least one `###` subsection with content\n", version) > "/dev/stderr"
      exit 1
    }
    printf("changelog gate: `## [%s]` is present, dated, and non-empty\n", version)
    exit 0
  }
' "$changelog"
