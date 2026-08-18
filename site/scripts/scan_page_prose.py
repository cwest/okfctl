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

"""Gate prose against negative-listing AI-slop cadence, in HTML AND Markdown.

WHY THIS EXISTS. AI-slop copy shipped to okfctl.dev twice and both times was
caught by hand, not by CI. The specific tell each time was a negative-listing
cadence -- "no CGO, no Python, no model runtime" written as a SENTENCE -- which
reads as a memorized rhythm rather than a fact a human wrote. The fix on PR #111
needed two rounds because round one scanned only body copy and never saw the
same cadence sitting in the <meta description>, the <og:description>, and the
footer <p>: text that exists only after render and only in the built HTML.

WHY IT ALSO SCANS MARKDOWN NOW. The rendered-HTML gate guarded one door while the
same defect walked through another. The README's line 30 once read "no CGO, no
Python, and no model runtime" -- the precise cadence this gate exists to block --
and CI stayed green, because the README is not HTML in site/dist. The tell in
running prose is the same whether it renders through Astro or ships as a raw .md
a reader browses on GitHub, so the SAME pattern core runs against both. There is
ONE detector (NEG_LISTING + find_hits) and two front ends that extract prose:
extract_prose (HTML) and extract_prose_markdown (Markdown). The pattern set is
never forked.

WHICH SURFACES. HTML: the BUILT site (site/dist/**/*.html), because meta/OG/footer
copy is assembled at build time and a source-level scan structurally cannot see
it. Markdown: the prose surfaces this repo SHIPS to readers -- README.md,
CONTRIBUTING.md, docs/*.md, docs/guides/*.md, docs/commands/README.md. Internal,
unpublished Markdown (docs/PRD.md, docs/adr/, docs/plans/, docs/specs/, testdata/)
is deliberately excluded: it is working material full of fixture prose, not
reader-facing copy, and gating it would train everyone to ignore the check. The
CI job owns that surface list; the scanner scans whatever paths it is handed.

WHY IT DETECTS EXACTLY ONE CLASS. This is a gate, and a gate that cries wolf gets
ignored. The broader prose-voice heuristics (vocabulary tells, "surface"/"layer"/
"lens" filler bans, sincerity disclaimers) fire heavily on legitimate technical
reference docs -- "semantic layer", "command surface", "TACA lens" are precise
domain nouns, not slop -- so gating on them would be ~75% false positives on this
corpus and would train everyone to ignore the check. Those remain a DRAFTING lint
(the writing-voice skill), not a CI gate. This gate detects the one class that
actually shipped: negative-listing cadence in running prose.

THE SPEC-CHIP / DISCRETE-LIST EXEMPTION (deliberate, not a widening to force
green). The homepage renders the tool's constraints as a row of discrete <span>
badges:

    <span>pure Go</span><span>no CGO</span><span>no Python</span>...

Each chip is a verifiable constraint from AGENTS.md, and no reader parses the row
as a sentence -- it is a specification table. So <span> is excluded from HTML
prose extraction. The Markdown front end has the SAME asymmetry by the SAME
mechanism: prose is extracted one block at a time (blocks are separated by blank
lines), and each list item is its own block. So the identical facts written as
three separate bullets --

    - no CGO
    - no Python
    - no model runtime

-- are three distinct one-item strings, and a chain needs TWO "no <item>" members
in ONE string to trip, so the list passes. The same facts in a running-prose
sentence ("It's pure Go, no CGO, no Python, no model runtime.") land in one block
and are caught. List passes; sentence fails -- the exact asymmetry the HTML side
has for chips, proven by the tests.

Usage:
    scan_page_prose.py <path.html|path.md> [more ...]
    # HTML, typically from CI:
    scan_page_prose.py $(find site/dist -name '*.html')
    # Markdown prose surfaces:
    scan_page_prose.py README.md CONTRIBUTING.md docs/*.md docs/guides/*.md

The front end is chosen per file by extension: .md/.markdown -> Markdown, every
other extension -> HTML.

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


def extract_prose_markdown(doc: str) -> list[tuple[str, str]]:
    """Return (source_label, text) for every human-visible prose block in Markdown.

    Same contract as extract_prose, different front end: it feeds the SAME
    NEG_LISTING detector via find_hits, so there is one pattern set, not two.

    Extraction is BLOCK-AT-A-TIME, blocks separated by blank lines -- the direct
    analog of the HTML side's per-element extraction, and the mechanism behind
    the discrete-list exemption (see the module docstring): three facts as three
    separate bullets are three one-item strings and cannot form a two-member
    chain, while the same facts in one running-prose sentence land in one block
    and are caught.

    Excluded, because none of it is a sentence a reader reads as running prose:
      - fenced code blocks (``` / ~~~), including the info string;
      - indented code blocks (4-space / tab lead on a non-list line);
      - inline code spans (`...`), stripped in place so backticked tokens do not
        contribute text (mirrors the HTML <code> exemption);
      - YAML frontmatter (--- ... --- at the top of the file);
      - HTML comments (<!-- ... -->), which carry no reader-facing prose.
    Link/image markup is reduced to its visible text so a URL never trips the
    detector. Markdown emphasis/heading punctuation is stripped so "**no X**"
    reads as "no X".
    """
    out: list[tuple[str, str]] = []

    # 0. Strip HTML comments spanning any number of lines.
    doc = re.sub(r"<!--.*?-->", " ", doc, flags=re.S)

    lines = doc.split("\n")

    # 1. Skip YAML frontmatter: a leading '---' fence closed by the next '---'.
    start = 0
    if lines and lines[0].strip() == "---":
        for i in range(1, len(lines)):
            if lines[i].strip() == "---":
                start = i + 1
                break

    # 2. Walk lines, dropping fenced/indented code, accumulating prose blocks.
    #    A block ends at a blank line so each paragraph and each list item is a
    #    separate string (the discrete-list exemption).
    fence: str | None = None            # the active code fence marker, or None
    block: list[str] = []
    block_line = 0                       # 1-based line where the current block began

    def flush() -> None:
        if not block:
            return
        raw = " ".join(block)
        txt = _markdown_to_text(raw)
        if len(txt) > 15:                # skip nav fragments / lone words
            out.append((f"md:L{block_line}", txt))
        block.clear()

    for idx in range(start, len(lines)):
        line = lines[idx]
        stripped = line.strip()

        # Fenced code block: toggle on a ``` / ~~~ run; skip everything inside.
        m = re.match(r"^\s*(`{3,}|~{3,})", line)
        if m:
            marker = m.group(1)[0] * 3
            if fence is None:
                flush()
                fence = marker
            elif marker == fence:
                fence = None
            continue
        if fence is not None:
            continue

        # Blank line ends the current block.
        if not stripped:
            flush()
            continue

        # Indented code block: 4-space / tab lead on a line that is NOT a list
        # continuation. Only treat as code when no block is open (a fresh block),
        # so wrapped list/paragraph lines are not misread.
        if not block and re.match(r"^(\t| {4,})", line) and not re.match(
            r"^(\t| {4,})*\s*([-*+]|\d+\.)\s", line
        ):
            continue

        if not block:
            block_line = idx + 1
        block.append(stripped)

    flush()
    return out


# Markdown emphasis/list/heading punctuation and link/code markup that must be
# reduced to visible text before the detector reads a block.
_MD_INLINE_CODE = re.compile(r"`[^`]*`")
_MD_IMAGE = re.compile(r"!\[([^\]]*)\]\([^)]*\)")
_MD_LINK = re.compile(r"\[([^\]]*)\]\([^)]*\)")
_MD_LEAD = re.compile(r"^\s*(#{1,6}\s+|>\s?|[-*+]\s+|\d+\.\s+)")
_MD_EMPH = re.compile(r"(\*{1,3}|_{1,3}|~~)")


def _markdown_to_text(raw: str) -> str:
    """Reduce a Markdown block to the plain prose a reader reads."""
    raw = _MD_INLINE_CODE.sub(" ", raw)      # drop backticked tokens (HTML <code>)
    raw = _MD_IMAGE.sub(r"\1", raw)          # image -> its alt text
    raw = _MD_LINK.sub(r"\1", raw)           # link -> its visible label, not the URL
    raw = _MD_LEAD.sub("", raw)              # strip heading/quote/list lead marker
    raw = _MD_EMPH.sub("", raw)              # strip * _ ~ emphasis runs
    raw = re.sub(r"\s+", " ", raw).strip()
    return raw


def find_hits(items: list[tuple[str, str]]) -> list[tuple[str, str, str]]:
    """Return (source_label, matched_text, full_text) for each finding."""
    hits: list[tuple[str, str, str]] = []
    for label, txt in items:
        for m in NEG_LISTING.finditer(txt):
            hits.append((label, m.group(0), txt))
    return hits


MARKDOWN_SUFFIXES = (".md", ".markdown")


def extract_for_path(path: str, text: str) -> list[tuple[str, str]]:
    """Pick the front end by file extension: .md/.markdown -> Markdown, else HTML."""
    if path.lower().endswith(MARKDOWN_SUFFIXES):
        return extract_prose_markdown(text)
    return extract_prose(text)


def main() -> int:
    if len(sys.argv) < 2:
        print(
            "usage: scan_page_prose.py <path.html|path.md> [more ...]",
            file=sys.stderr,
        )
        return 2

    total_hits = 0
    for path in sys.argv[1:]:
        page = Path(path).read_text()
        items = extract_for_path(path, page)
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
