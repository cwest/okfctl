#!/usr/bin/env python3
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

"""Gate the BUILT site's rendered prose against negative-listing AI-slop cadence.

WHY THIS EXISTS. AI-slop copy shipped to okfctl.dev twice and both times was
caught by hand, not by CI. The specific tell each time was a negative-listing
cadence -- "no CGO, no Python, no model runtime" written as a SENTENCE -- which
reads as a memorized rhythm rather than a fact a human wrote. The fix on PR #111
needed two rounds because round one scanned only body copy and never saw the
same cadence sitting in the <meta description>, the <og:description>, and the
footer <p>: text that exists only after render and only in the built HTML.

WHY IT RUNS ON THE BUILT OUTPUT, NOT THE SOURCE. The round-one miss is the whole
argument. Meta/OG/footer prose is assembled at build time; a source-level scan
structurally cannot see it. So this runs against site/dist/**/*.html.

WHY IT DETECTS EXACTLY ONE CLASS. This is a gate, and a gate that cries wolf gets
ignored. The broader prose-voice heuristics (vocabulary tells, "surface"/"layer"/
"lens" filler bans, sincerity disclaimers) fire heavily on legitimate technical
reference docs -- "semantic layer", "command surface", "TACA lens" are precise
domain nouns, not slop -- so gating on them would be ~75% false positives on this
corpus and would train everyone to ignore the check. Those remain a DRAFTING lint
(the writing-voice skill), not a CI gate. This gate detects the one class that
actually shipped: negative-listing cadence in running prose.

THE SPEC-CHIP EXEMPTION (deliberate, not a widening to force green). The homepage
renders the tool's constraints as a row of discrete <span> badges:

    <span>pure Go</span><span>no CGO</span><span>no Python</span>...

Each chip is a verifiable constraint from AGENTS.md, and no reader parses the row
as a sentence -- it is a specification table. So <span> is excluded from prose
extraction. The exemption is SCOPED: the identical facts written as a running-
prose sentence ("It is pure Go, no CGO, no Python, no model runtime.") are still
caught. Chips pass; the sentence fails. That asymmetry is proven by the tests and
is what keeps the exemption from hiding the real finding.

Usage:
    scan_page_prose.py <built.html> [more.html ...]
    # or, typically from CI:
    scan_page_prose.py $(find site/dist -name '*.html')

Exit 0 when clean, 1 when a finding is present, 2 on a usage error. Stdlib only.
"""

import html
import re
import sys
from pathlib import Path

# A negative-listing chain: "no X, no Y" (optionally longer, optionally "...and
# no Z"). Anchored on repeated "no <item>" separated by ordinary punctuation and
# whitespace only -- em/en dashes are intentionally NOT separators, because the
# chip row is dash-free and the running-prose tic uses commas/"and". The chain
# needs at least TWO "no <item>" members so a bare "no cache" never trips.
#
# Each ITEM is "no" followed by ONE-to-THREE words, not a single token: the slop
# the gate exists to catch routinely uses multi-word items -- the OKF spec's own
# wording, "no schema registry, no central authority, and no required tooling",
# is three two-word items. A single-token matcher ("no <word>") silently passed
# that entire class and even truncated "no model runtime" to "no model". The
# inner-word negative lookahead (?!and\b|no\b) stops an item from swallowing the
# next separator, so "no cache, no index and it just works" matches exactly the
# chain and not the trailing clause.
_WORD = r"(?!and\b|no\b)[A-Za-z][\w-]*"       # a word that is not a separator token
_ITEM = rf"no\s+{_WORD}(?:\s+{_WORD}){{0,2}}"  # no <1-3 words>
NEG_LISTING = re.compile(
    rf"\b{_ITEM}"                              # no X
    rf"(?:\s*,\s*(?:and\s+)?{_ITEM})+"         # , [and] no Y (, no Z ...)
    rf"|"
    rf"\b{_ITEM}\s+and\s+{_ITEM}",             # no X and no Y
    re.IGNORECASE,
)

# Non-prose regions: their text is not a sentence a reader reads, so a match
# inside them is noise. Stripped before body extraction.
NON_PROSE_TAGS = ("script", "style", "noscript", "svg", "pre", "code", "template")

# Block-level prose containers. <span> is deliberately EXCLUDED (spec-chip row).
PROSE_BLOCKS = ("h1", "h2", "h3", "h4", "h5", "h6", "p", "li", "blockquote",
                "figcaption", "dd", "dt")


def extract_prose(page: str) -> list[tuple[str, str]]:
    """Return (source_label, text) for every human-visible prose string.

    Subtractive on the body: strip non-prose regions first, then take the block
    containers that remain -- so a newly-added prose element is included by
    default and only <span>/script/style/code/pre are ever exempt.
    """
    out: list[tuple[str, str]] = []

    # 1. Meta / OG / Twitter descriptions and titles. Invisible on screen, read
    #    by humans in search results and social cards. THE class PR #111 missed.
    for m in re.finditer(
        r'<meta[^>]+(?:name|property)="([^"]*(?:description|title))"[^>]+content="([^"]+)"',
        page,
        re.I,
    ):
        out.append((f"meta[{m.group(1)}]", html.unescape(m.group(2))))
    # content= may precede name/property in attribute order; catch that too.
    for m in re.finditer(
        r'<meta[^>]+content="([^"]+)"[^>]+(?:name|property)="([^"]*(?:description|title))"',
        page,
        re.I,
    ):
        out.append((f"meta[{m.group(2)}]", html.unescape(m.group(1))))

    # 2. <title>
    if t := re.search(r"<title[^>]*>(.*?)</title>", page, re.S | re.I):
        out.append(("title", html.unescape(re.sub(r"<[^>]+>", "", t.group(1)))))

    # 3. Body prose -- strip non-prose regions FIRST, then the block containers.
    body = page
    for tag in NON_PROSE_TAGS:
        body = re.sub(rf"<{tag}\b.*?</{tag}>", " ", body, flags=re.S | re.I)

    block_re = re.compile(
        rf"<({'|'.join(PROSE_BLOCKS)})\b[^>]*>(.*?)</\1>", re.S | re.I
    )
    for m in block_re.finditer(body):
        txt = html.unescape(re.sub(r"<[^>]+>", " ", m.group(2)))
        txt = re.sub(r"\s+", " ", txt).strip()
        if len(txt) > 15:  # skip nav fragments / single words
            out.append((f"<{m.group(1).lower()}>", txt))

    # 4. Alt text and aria-labels -- prose a human reads via assistive tech.
    for attr in ("alt", "aria-label"):
        for m in re.finditer(rf'{attr}="([^"]{{16,}})"', body):
            out.append((attr, html.unescape(m.group(1))))
    return out


def find_hits(items: list[tuple[str, str]]) -> list[tuple[str, str, str]]:
    """Return (source_label, matched_text, full_text) for each finding."""
    hits: list[tuple[str, str, str]] = []
    for label, txt in items:
        for m in NEG_LISTING.finditer(txt):
            hits.append((label, m.group(0), txt))
    return hits


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: scan_page_prose.py <built.html> [more.html ...]", file=sys.stderr)
        return 2

    total_hits = 0
    for path in sys.argv[1:]:
        page = Path(path).read_text()
        items = extract_prose(page)
        hits = find_hits(items)
        print(f"=== {path} — {len(items)} prose strings scanned ===")
        if hits:
            for label, matched, txt in hits:
                excerpt = txt if len(txt) <= 120 else txt[:117] + "..."
                print(f"    NEGATIVE-LISTING in {label}: [{matched}]  ({excerpt})")
                total_hits += 1
        else:
            print("    (clean)")

    print()
    if total_hits:
        print(f"RESULT: {total_hits} negative-listing finding(s) — rewrite the "
              f"flagged prose to plain positive statements (every fact preserved).")
        return 1
    print("RESULT: CLEAN")
    return 0


if __name__ == "__main__":
    sys.exit(main())
