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

"""Gate prose: negative-listing AI-slop cadence AND em-dash spacing, HTML + MD.

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
CONTRIBUTING.md, docs/*.md, docs/guides/*.md, docs/commands/README.md, and
site/public/llms.txt (Markdown by the llms.txt convention, so it rides the
Markdown front end despite its .txt extension). JSON: the top-level description
of plugin.json, the blurb an agent client shows when it lists the plugin; every
other manifest value is data and is never scanned. SVG: the <text> lines of the
social card, which generate-share-assets.mjs writes to site/.share-assets/og.svg
beside the og.png it rasterizes. The card's second line once read "Pure Go --
no CGO, no model runtime." because no check ever read the image. Internal,
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

THE SECOND CLASS: EM-DASH SPACING. Casey's house style for prose is the CLOSED-UP
Chicago em dash (word—word), not the spaced form (word — word). The repo drifted
the other way, so this gate also fails on a SPACED em dash ( — ) in prose to keep
the standard from regressing once the corpus is converted. It rides the SAME two
extractors as the negative-listing check -- so it inherits every exemption for
free: a spaced em dash inside a fenced code block, an inline `code` span, HTML
<pre>/<code>, YAML frontmatter, or a URL is NOT prose and stays silent. That is
the load-bearing negative control: code and CLI-help output follow their own
medium's typography, not prose typography, so the gate must never fire there. The
en dash (–, number ranges) and the closed-up em dash (word—word) are both left
alone; only the spaced em dash between two prose words is a finding.

THE THIRD CLASS: BUILD INTERNALS AS SELLING POINTS. "no CGO", "CGO_ENABLED"
(outside code), "no model runtime", and "no Python" describe how the binary is
built, not what a user gets, so shipped copy must not sell them. This class
reads MORE than the other two: <span> chips and short fragments count, because
the homepage spec strip and a bulleted list are where these phrases shipped.
Code stays exempt, so a build command remains legal. Two prose sections that
explain the build on purpose (install.md's build-from-source section, the
search guide's model2vec section) are exempt through BUILD_INTERNALS_ALLOWLIST,
scoped to the section and only for this class.

Usage:
    scan_page_prose.py <path.html|path.md> [more ...]
    # HTML, typically from CI:
    scan_page_prose.py $(find site/dist -name '*.html')
    # Markdown prose surfaces:
    scan_page_prose.py README.md CONTRIBUTING.md docs/*.md docs/guides/*.md
    # Shipped non-.md surfaces:
    scan_page_prose.py site/public/llms.txt plugin.json
    # The social card's text, as rasterized into og.png:
    scan_page_prose.py site/.share-assets/og.svg

The front end is chosen per file by name: .md/.markdown and llms.txt ->
Markdown, .json -> manifest prose fields, .svg -> the <text> lines of the
social card, every other extension -> HTML.

Exit 0 when clean, 1 when a finding is present, 2 on a usage error (including a
.json file that does not parse). Stdlib only.
"""

import html
import json
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

# A SPACED em dash between two prose words: the drift away from Casey's house
# style (closed-up Chicago, word—word). We match an em dash (U+2014) that has
# whitespace on BOTH sides -- the spaced form. The closed-up form (word—word) has
# no surrounding space and is CORRECT, so it must never match; the en dash
# (U+2013), used for number ranges, is a different character and left alone. The
# match is deliberately narrow (one space each side of a single em dash) so it
# reports the exact spot to close up, and so a stray double space or a line-wrap
# does not smuggle a spaced dash past it or invent a false one.
EM_DASH_SPACING = re.compile(r"\s\u2014\s")

# Build internals sold as features. Users don't care that the binary has "no
# CGO" or "no model runtime" unless the copy says what IS in it, so shipped copy
# must not list these as selling points. ONE mention is a finding (unlike the
# two-member negative-listing chain), case does not matter, and "CGO_ENABLED"
# counts only outside code: the extractors already drop fenced/indented code,
# inline `code`, and HTML <code>/<pre>, so a build command stays legal. Where
# prose genuinely has to explain the build, see BUILD_INTERNALS_ALLOWLIST.
# "CGO_?ENABLED": the Markdown front end strips "_" emphasis runs, so in an
# extracted .md block the token reads "CGOENABLED".
BUILD_INTERNALS = re.compile(
    r"\bno\s+cgo\b|\bCGO_?ENABLED\b|\bno\s+model\s+runtime\b|\bno\s+python\b",
    re.IGNORECASE,
)

# The prose detectors, each a (class-name, compiled-pattern) pair. find_hits
# runs EVERY detector against EVERY extracted prose string, so both classes ride
# the same code-block / inline-code / frontmatter exemptions the extractors give.
DETECTORS: tuple[tuple[str, "re.Pattern[str]"], ...] = (
    ("negative-listing", NEG_LISTING),
    ("spaced-em-dash", EM_DASH_SPACING),
)

# The build-internals detector runs on a WIDER extraction than the prose
# detectors (fragments=True): it also reads <span> chips and strings shorter
# than the 15-character prose floor, because a spec-strip chip or a two-word
# bullet ("no Python") is exactly where a build internal gets sold. The
# chip/bullet exemption exists for the negative-listing RHYTHM, not for content.
FRAGMENT_DETECTORS: tuple[tuple[str, "re.Pattern[str]"], ...] = (
    ("build-internals", BUILD_INTERNALS),
)

# Sections where prose legitimately explains how okfctl is built, exempt from
# the build-internals class ONLY. Each entry is (path suffix, heading prefix):
# the section runs from a heading whose visible text starts with the prefix to
# the next heading of the same or higher level. Matched in both the Markdown
# source and the built HTML of the same page. Keep this list short and give
# every entry a reason; anything not listed is selling copy.
BUILD_INTERNALS_ALLOWLIST: tuple[tuple[str, str], ...] = (
    # Build instructions: the reader asked how to compile it.
    ("docs/install.md", "Building from source"),
    # Technical explanation of how the model2vec embedder stays pure Go.
    ("docs/guides/search.md", "Real semantic search with"),
    ("guides/search/index.html", "Real semantic search with"),
)

# Non-prose regions: their text is not a sentence a reader reads, so a match
# inside them is noise. Stripped before body extraction.
NON_PROSE_TAGS = ("script", "style", "noscript", "svg", "pre", "code", "template")

# Meta name/property suffixes whose content= is copy a human reads: search
# snippets, link-card titles and descriptions, and the card image's alt text.
META_PROSE_SUFFIX = r"(?:description|title|image:alt)"

# Block-level prose containers. <span> is deliberately EXCLUDED (spec-chip row).
PROSE_BLOCKS = ("h1", "h2", "h3", "h4", "h5", "h6", "p", "li", "blockquote",
                "figcaption", "dd", "dt")


def extract_prose(page: str, fragments: bool = False) -> list[tuple[str, str]]:
    """Return (source_label, text) for every human-visible prose string.

    Subtractive on the body: strip non-prose regions first, then take the block
    containers that remain -- so a newly-added prose element is included by
    default and only <span>/script/style/code/pre are ever exempt. With
    fragments=True, <span> text and strings under the prose floor are kept too
    (the build-internals pass; see FRAGMENT_DETECTORS).
    """
    out: list[tuple[str, str]] = []

    # 1. Meta / OG / Twitter descriptions, titles, and image alt text.
    #    Invisible on screen, read by humans in search results and on the
    #    unfurled link card. THE class PR #111 missed. og:image:alt and
    #    twitter:image:alt are the card's alt text; og:image, og:url, and
    #    twitter:card are data (URLs, enums) and never match.
    for m in re.finditer(
        rf'<meta[^>]+(?:name|property)="([^"]*{META_PROSE_SUFFIX})"[^>]+content="([^"]+)"',
        page,
        re.I,
    ):
        out.append((f"meta[{m.group(1)}]", html.unescape(m.group(2))))
    # content= may precede name/property in attribute order; catch that too.
    for m in re.finditer(
        rf'<meta[^>]+content="([^"]+)"[^>]+(?:name|property)="([^"]*{META_PROSE_SUFFIX})"',
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

    floor = 0 if fragments else 15
    blocks = PROSE_BLOCKS + (("span",) if fragments else ())
    block_re = re.compile(
        rf"<({'|'.join(blocks)})\b[^>]*>(.*?)</\1>", re.S | re.I
    )
    for m in block_re.finditer(body):
        txt = html.unescape(re.sub(r"<[^>]+>", " ", m.group(2)))
        txt = re.sub(r"\s+", " ", txt).strip()
        if len(txt) > floor:  # skip nav fragments / single words
            out.append((f"<{m.group(1).lower()}>", txt))

    # 4. Alt text and aria-labels -- prose a human reads via assistive tech.
    for attr in ("alt", "aria-label"):
        for m in re.finditer(rf'{attr}="([^"]{{16,}})"', body):
            out.append((attr, html.unescape(m.group(1))))
    return out


def extract_prose_svg(doc: str) -> list[tuple[str, str]]:
    """Return ("svg:text", text) for every <text> element in an SVG.

    The social card (og.png) is rasterized from an SVG that
    generate-share-assets.mjs composes and also writes out for this gate. Its
    <text> lines are the only words a reader sees in the image; attributes
    (font stacks, coordinates) and XML comments are not rendered and are never
    read. No length floor: every card line is display copy, however short.
    """
    doc = re.sub(r"<!--.*?-->", " ", doc, flags=re.S)
    out: list[tuple[str, str]] = []
    for m in re.finditer(r"<text\b[^>]*>(.*?)</text>", doc, re.S | re.I):
        txt = html.unescape(re.sub(r"<[^>]+>", " ", m.group(1)))
        txt = re.sub(r"\s+", " ", txt).strip()
        if txt:
            out.append(("svg:text", txt))
    return out


def extract_prose_markdown(doc: str, fragments: bool = False) -> list[tuple[str, str]]:
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
        if len(txt) > (0 if fragments else 15):  # skip nav fragments / lone words
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


def find_hits(
    items: list[tuple[str, str]],
    suppress: frozenset[str] = frozenset(),
    detectors: tuple[tuple[str, "re.Pattern[str]"], ...] = DETECTORS,
) -> list[tuple[str, str, str, str]]:
    """Return (detector_class, source_label, matched_text, full_text) per finding.

    Runs every detector in `detectors` (default DETECTORS) against every
    extracted prose string, so the negative-listing and spaced-em-dash classes
    share one extraction pass and one set of exemptions (code fences, inline
    code, frontmatter). The build-internals pass calls it with
    FRAGMENT_DETECTORS on a wider extraction.

    `suppress` names detector classes to skip for this file. It exists for ONE
    case: a GENERATED reference doc (docs/commands/README.md) mirrors the CLI
    help text verbatim, and CLI help output follows its own medium's typography,
    not prose typography (AGENTS.md: "let the generated file match" its source).
    So the spaced-em-dash class is suppressed on generated files, while the
    negative-listing class — a real slop cadence wherever it appears — still runs.
    """
    hits: list[tuple[str, str, str, str]] = []
    for label, txt in items:
        for cls, pattern in detectors:
            if cls in suppress:
                continue
            for m in pattern.finditer(txt):
                hits.append((cls, label, m.group(0), txt))
    return hits


MARKDOWN_SUFFIXES = (".md", ".markdown")

# A file that declares itself generated from the command tree mirrors CLI help
# text; its em-dash spacing follows the help output's typography, not prose
# style, so the spaced-em-dash detector is suppressed on it (see find_hits).
GENERATED_MARKER = re.compile(r"generated from the command tree", re.I)


def suppressed_classes(text: str) -> frozenset[str]:
    """Detector classes to skip for this file. Only the em-dash class, and only
    on a self-declared generated reference doc."""
    if GENERATED_MARKER.search(text):
        return frozenset({"spaced-em-dash"})
    return frozenset()


# llms.txt (https://llmstxt.org) is Markdown by convention despite its .txt
# extension. It ships verbatim to okfctl.dev, so it gets the Markdown front end;
# routed by extension it fell to the HTML front end, found zero prose strings,
# and passed unread.
MARKDOWN_BASENAMES = ("llms.txt", "llms-full.txt")

# JSON manifest fields that are prose a reader reads. In plugin.json only the
# top-level description qualifies: an agent client shows it as the plugin's
# blurb. Every other value (schema URL, names, keywords, install commands) is
# data, not prose, and is never scanned.
JSON_PROSE_KEYS = ("description",)


class UsageError(Exception):
    """An input the scanner cannot read. Exits 2, never passes silently."""


def extract_prose_json(doc: str) -> list[tuple[str, str]]:
    """Return (source_label, text) for the top-level prose fields of a manifest."""
    try:
        data = json.loads(doc)
    except json.JSONDecodeError as e:
        raise UsageError(f"not valid JSON: {e}") from e
    out: list[tuple[str, str]] = []
    if isinstance(data, dict):
        for key in JSON_PROSE_KEYS:
            if isinstance(data.get(key), str):
                out.append((f"json:{key}", data[key]))
    return out


def extract_for_path(path: str, text: str, fragments: bool = False) -> list[tuple[str, str]]:
    """Pick the front end by file name: .md/.markdown/llms.txt -> Markdown,
    .json -> manifest prose fields, .svg -> <text> lines, every other
    extension -> HTML."""
    lower = path.lower()
    if lower.endswith(".svg"):
        return extract_prose_svg(text)
    if lower.endswith(MARKDOWN_SUFFIXES) or Path(lower).name in MARKDOWN_BASENAMES:
        return extract_prose_markdown(text, fragments=fragments)
    if lower.endswith(".json"):
        return extract_prose_json(text)
    return extract_prose(text, fragments=fragments)


def _allowlisted_headings(path: str) -> list[str]:
    """Heading prefixes allowlisted for this path (suffix match on a / boundary)."""
    norm = "/" + Path(path).as_posix().lstrip("/")
    return [h for suffix, h in BUILD_INTERNALS_ALLOWLIST if norm.endswith("/" + suffix)]


def strip_allowlisted_sections(path: str, text: str) -> str:
    """Remove BUILD_INTERNALS_ALLOWLIST sections from text before the
    build-internals pass. A section runs from its heading to the next heading of
    the same or higher level (Markdown, fence-aware) or the next <h1>..<hN> /
    </main> (HTML). Files with no allowlist entry are returned unchanged."""
    prefixes = _allowlisted_headings(path)
    if not prefixes:
        return text
    is_md = not path.lower().endswith((".html", ".htm"))
    for prefix in prefixes:
        text = _strip_md_section(text, prefix) if is_md else _strip_html_section(text, prefix)
    return text


def _strip_md_section(doc: str, prefix: str) -> str:
    lines = doc.split("\n")
    out: list[str] = []
    fence: str | None = None
    cut_level = 0                        # >0 while inside the allowlisted section
    for line in lines:
        m = re.match(r"^\s*(`{3,}|~{3,})", line)
        if m:
            marker = m.group(1)[0] * 3
            fence = marker if fence is None else (None if marker == fence else fence)
        elif fence is None and (h := re.match(r"^(#{1,6})\s+(.*)$", line)):
            level = len(h.group(1))
            if cut_level and level <= cut_level:
                cut_level = 0
            if not cut_level and _markdown_to_text(h.group(2)).startswith(prefix):
                cut_level = level
        if not cut_level:
            out.append(line)
    return "\n".join(out)


def _strip_html_section(page: str, prefix: str) -> str:
    for m in re.finditer(r"<h([1-6])\b[^>]*>(.*?)</h\1>", page, re.S | re.I):
        visible = html.unescape(re.sub(r"<[^>]+>", "", m.group(2))).strip()
        if not visible.startswith(prefix):
            continue
        level = int(m.group(1))
        end = re.compile(rf"<h[1-{level}]\b|</main>", re.I).search(page, m.end())
        return page[: m.start()] + page[end.start() if end else len(page):]
    return page


def main() -> int:
    if len(sys.argv) < 2:
        print(
            "usage: scan_page_prose.py <path.html|path.md> [more ...]",
            file=sys.stderr,
        )
        return 2

    total_hits = 0
    class_counts: dict[str, int] = {}
    for path in sys.argv[1:]:
        page = Path(path).read_text()
        try:
            items = extract_for_path(path, page)
        except UsageError as e:
            print(f"scan_page_prose.py: {path}: {e}", file=sys.stderr)
            return 2
        hits = find_hits(items, suppress=suppressed_classes(page))
        fragment_items = extract_for_path(
            path, strip_allowlisted_sections(path, page), fragments=True
        )
        hits += find_hits(fragment_items, detectors=FRAGMENT_DETECTORS)
        print(f"=== {path} — {len(items)} prose strings scanned ===")
        if hits:
            for cls, label, matched, txt in hits:
                excerpt = txt if len(txt) <= 120 else txt[:117] + "..."
                shown = matched.replace("\u2014", "—")
                print(f"    {cls.upper()} in {label}: [{shown}]  ({excerpt})")
                total_hits += 1
                class_counts[cls] = class_counts.get(cls, 0) + 1
        else:
            print("    (clean)")

    print()
    if total_hits:
        parts = ", ".join(f"{n} {cls}" for cls, n in sorted(class_counts.items()))
        print(f"RESULT: {total_hits} prose finding(s) [{parts}] — rewrite "
              f"negative-listing prose to plain positive statements (every fact "
              f"preserved), close up spaced em dashes to Chicago (word—word), and "
              f"replace build internals (no CGO, CGO_ENABLED, no model runtime, "
              f"no Python) with what okfctl IS or does.")
        return 1
    print("RESULT: CLEAN")
    return 0


if __name__ == "__main__":
    sys.exit(main())
