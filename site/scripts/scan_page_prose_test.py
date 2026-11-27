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

"""Tests for scan_page_prose.py — the CI prose gate.

The gate detects one class: negative-listing cadence ("no X, no Y") in RENDERED
running prose (meta/OG/title + body block elements + alt/aria), the exact slop
that shipped to okfctl.dev twice and that PR #111 needed two rounds to clear
because a body-only scanner never saw the meta/footer copy.

Every test writes a self-contained HTML fixture to a temp dir and runs the
scanner as a subprocess, so it exercises the same code path CI runs. Stdlib
only (unittest + subprocess), no third-party deps — the repo is pure Go + a
Node site and must stay installable on a clean runner with nothing but Python.
"""

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCANNER = Path(__file__).with_name("scan_page_prose.py")


def run(*html_pages: str):
    """Write each page to a temp .html file, run the scanner, return CompletedProcess."""
    with tempfile.TemporaryDirectory() as d:
        paths = []
        for i, page in enumerate(html_pages):
            p = Path(d) / f"page{i}.html"
            p.write_text(page)
            paths.append(str(p))
        return subprocess.run(
            [sys.executable, str(SCANNER), *paths],
            capture_output=True,
            text=True,
        )


def run_md(*docs: str):
    """Write each doc to a temp .md file, run the scanner, return CompletedProcess.

    Same as run() but the .md extension routes the scanner through the Markdown
    front end (extract_prose_markdown) instead of the HTML one.
    """
    with tempfile.TemporaryDirectory() as d:
        paths = []
        for i, doc in enumerate(docs):
            p = Path(d) / f"doc{i}.md"
            p.write_text(doc)
            paths.append(str(p))
        return subprocess.run(
            [sys.executable, str(SCANNER), *paths],
            capture_output=True,
            text=True,
        )


CLEAN_PAGE = """<!doctype html><html><head>
<meta name="description" content="A single static Go binary you drop on your PATH.">
<meta property="og:description" content="Knowledge you can move without breaking.">
<title>okfctl</title></head><body>
<h1>okfctl</h1>
<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>
<span>pure Go</span><span>no server</span><span>no daemon</span><span>no account</span>
</body></html>"""


class ExitCodeContract(unittest.TestCase):
    def test_clean_page_exits_zero(self):
        r = run(CLEAN_PAGE)
        self.assertEqual(r.returncode, 0, f"expected clean, got:\n{r.stdout}\n{r.stderr}")
        self.assertIn("CLEAN", r.stdout)

    def test_no_args_is_usage_error(self):
        r = subprocess.run([sys.executable, str(SCANNER)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 2)


class NegativeListingInRunningProse(unittest.TestCase):
    """The incident class: 'no X, no Y' cadence in rendered prose must FAIL."""

    def test_body_paragraph_negative_listing_fails(self):
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            "<p>Pure Go, no CGO, no Python, no model runtime.</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("no CGO, no Python", r.stdout)

    def test_meta_description_negative_listing_fails(self):
        # The exact class PR #111 round one missed: meta text only exists after
        # render, so a source-level or body-only scan cannot see it.
        page = CLEAN_PAGE.replace(
            'content="A single static Go binary you drop on your PATH."',
            'content="Pure Go, no CGO, no Python, no model runtime."',
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("meta", r.stdout)
        self.assertIn("no CGO, no Python", r.stdout)

    def test_og_description_negative_listing_fails(self):
        page = CLEAN_PAGE.replace(
            'content="Knowledge you can move without breaking."',
            'content="No model, no index, no server."',
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("og:description", r.stdout)

    def test_footer_paragraph_negative_listing_fails(self):
        # The other PR #111 miss: a footer <p> outside the main body copy.
        page = CLEAN_PAGE.replace(
            "</body>",
            "<footer><p>One binary. No CGO, no Python, no ONNX runtime.</p></footer></body>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("No CGO, no Python", r.stdout)


class MultiWordNegativeItems(unittest.TestCase):
    """Regression: each negative item may be MORE than one word. A single-token
    matcher ('no <word>') silently passes a chain of multi-word items — the
    exact gap that let the OKF spec's own wording ('no schema registry, no
    central authority, and no required tooling') through the gate. The item
    'schema registry' / 'central authority' / 'required tooling' each spans two
    words; the detector must still fire.
    """

    SPEC_PHRASE = (
        "No schema registry, no central authority, and no required tooling."
    )

    def test_multiword_items_in_meta_description_fail(self):
        # The spec's own sentence, in the class PR #111 missed (render-only meta).
        page = CLEAN_PAGE.replace(
            'content="A single static Go binary you drop on your PATH."',
            f'content="{self.SPEC_PHRASE}"',
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("meta", r.stdout)
        self.assertIn("no schema registry", r.stdout.lower())

    def test_multiword_items_in_body_paragraph_fail(self):
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            f"<p>{self.SPEC_PHRASE}</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("no schema registry", r.stdout.lower())

    def test_multiword_match_spans_the_whole_last_item(self):
        # The single-token matcher truncated 'no model runtime' to 'no model';
        # the multi-word matcher must capture the trailing words of each item.
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            "<p>Pure Go, no build step, no model runtime.</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("no build step", r.stdout)


class SingleNegativeIsClean(unittest.TestCase):
    """The chain needs at least TWO negative members; a lone 'no <phrase>' — even
    a multi-word one — is an ordinary factual clause, not the slop cadence, and
    must NOT trip. This is the negative control that keeps the widened matcher
    from firing on legitimate technical prose.
    """

    def test_single_multiword_negative_passes(self):
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            "<p>It needs no external model server to run offline.</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"single negative must pass:\n{r.stdout}")

    def test_two_distant_unrelated_negatives_pass(self):
        # Two 'no' clauses in different sentences are not a comma/and-joined
        # chain and must not be stitched into a false finding.
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            "<p>There is no ambiguity here. The format leaves no room for drift.</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"unrelated negatives must pass:\n{r.stdout}")


class SpecChipExemption(unittest.TestCase):
    """PR #111's established exemption: discrete <span> chips are a spec table,
    not a sentence. The SAME 'no X, no Y' facts as chips must PASS while the
    same facts as running prose must FAIL — proving the exemption is scoped, not
    a blanket suppression that hides the real finding."""

    def test_spec_chips_pass(self):
        # CLEAN_PAGE already carries the chip row; assert it does not trip.
        r = run(CLEAN_PAGE)
        self.assertEqual(r.returncode, 0, f"chips must not trip:\n{r.stdout}")

    def test_chips_pass_but_same_facts_in_prose_fail(self):
        chips_page = CLEAN_PAGE  # facts as chips -> clean
        prose_page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            "<p>It is pure Go, no server, no daemon, no account.</p>",
        )
        self.assertEqual(run(chips_page).returncode, 0)
        self.assertEqual(run(prose_page).returncode, 1)


class NonProseRegionsExcluded(unittest.TestCase):
    """The scanner must not fire inside script/style/code/pre — those are not
    prose a reader reads as a sentence, and firing there is pure noise."""

    def test_code_block_negative_listing_ignored(self):
        page = CLEAN_PAGE.replace(
            "</body>",
            "<pre><code>okfctl build --no-cache, no-index, no-model</code></pre></body>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"code must be exempt:\n{r.stdout}")


# ---------------------------------------------------------------------------
# Markdown front end (extract_prose_markdown). Same NEG_LISTING detector, a
# different extractor. These tests are the card's load-bearing requirement: the
# README hole (a .md the HTML gate never saw) is closed, proven with BOTH
# controls — the negative-listing sentence FAILS, and a legitimate look-alike
# (discrete bullets / a code fence) PASSES.
# ---------------------------------------------------------------------------

# A README-shaped clean fixture: the same three constraint facts the site chip
# row carries, written as running prose the plain-positive way (no chain) plus a
# discrete bulleted restatement. This must stay clean — it is the corpus shape
# after the prose cards landed.
CLEAN_MD = """# okfctl

A knowledge base decays the moment a path changes. okfctl keeps links honest.

It's a single static Go binary that runs anywhere without a toolchain, an
interpreter, or a model download.

Run guarantees:

- pure Go
- no server
- no daemon
- no account
"""


class MarkdownExitCodeContract(unittest.TestCase):
    def test_clean_markdown_exits_zero(self):
        r = run_md(CLEAN_MD)
        self.assertEqual(r.returncode, 0, f"expected clean, got:\n{r.stdout}\n{r.stderr}")
        self.assertIn("CLEAN", r.stdout)


class MarkdownNegativeListingFails(unittest.TestCase):
    """POSITIVE CONTROL. The exact README hole the card names: a negative-listing
    sentence in a Markdown prose block must FAIL, because the README is a .md the
    HTML gate structurally never saw."""

    def test_readme_hole_sentence_fails(self):
        # The literal cadence from the card: README line 30 once read this.
        doc = CLEAN_MD.replace(
            "It's a single static Go binary that runs anywhere without a toolchain, an\n"
            "interpreter, or a model download.",
            "It's pure Go: no CGO, no Python, and no model runtime.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"expected failure:\n{r.stdout}")
        self.assertIn("no CGO, no Python", r.stdout)

    def test_negative_listing_in_heading_fails(self):
        doc = CLEAN_MD.replace("# okfctl", "# okfctl: no CGO, no Python, no ONNX")
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"heading prose must be scanned:\n{r.stdout}")
        self.assertIn("no CGO, no Python", r.stdout)

    def test_negative_listing_in_blockquote_fails(self):
        # A blockquote is running prose a reader reads as a sentence (unlike the
        # contraction lint, which leaves quotes alone — different class).
        doc = CLEAN_MD + "\n> It ships with no CGO, no Python, and no model runtime.\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"blockquote prose must be scanned:\n{r.stdout}")
        self.assertIn("no CGO, no Python", r.stdout)

    def test_multiword_items_in_markdown_fail(self):
        # The spec's own wording, the multi-word-item class: 'schema registry' /
        # 'central authority' / 'required tooling' each span two words.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "The format has no schema registry, no central authority, and no required tooling.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"multiword items must fire:\n{r.stdout}")
        self.assertIn("no schema registry", r.stdout.lower())

    def test_emphasis_markup_does_not_hide_the_chain(self):
        # Bold/italic around items must not smuggle the cadence past the detector.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "It's pure Go: **no CGO**, *no Python*, and no model runtime.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"emphasis must not hide the chain:\n{r.stdout}")
        self.assertIn("no CGO", r.stdout)


class MarkdownLegitimateLookAlikesPass(unittest.TestCase):
    """NEGATIVE CONTROL — the load-bearing half. A legitimate look-alike must stay
    silent, or the gate is a nuisance that gets disabled. The card names two: the
    same three facts as a bulleted list, and the phrase inside a fenced code
    block. Both must PASS while the running-prose sentence FAILS."""

    def test_discrete_bullets_pass(self):
        # CLEAN_MD already carries the three facts as separate bullets; assert it
        # does not trip. Each bullet is its own block (one 'no <item>' each), so
        # no two-member chain forms — the Markdown analog of the <span> chip row.
        r = run_md(CLEAN_MD)
        self.assertEqual(r.returncode, 0, f"discrete bullets must pass:\n{r.stdout}")

    def test_bullets_pass_but_same_facts_in_prose_fail(self):
        # The scoped-exemption proof, mirroring test_chips_pass_but... on the HTML
        # side: identical facts, list passes, sentence fails.
        prose = CLEAN_MD.replace(
            "It's a single static Go binary that runs anywhere without a toolchain, an\n"
            "interpreter, or a model download.",
            "It's pure Go, no server, no daemon, no account.",
        )
        self.assertEqual(run_md(CLEAN_MD).returncode, 0)
        self.assertEqual(run_md(prose).returncode, 1)

    def test_fenced_code_block_passes(self):
        # The phrase inside a fenced code block is a command/config sample, not a
        # sentence — it must not trip.
        doc = CLEAN_MD + "\n```sh\nokfctl build   # no CGO, no Python, no model runtime\n```\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"fenced code must be exempt:\n{r.stdout}")

    def test_tilde_fenced_code_block_passes(self):
        doc = CLEAN_MD + "\n~~~\nno CGO, no Python, no model runtime\n~~~\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"tilde fence must be exempt:\n{r.stdout}")

    def test_indented_code_block_passes(self):
        doc = CLEAN_MD + "\nExample invocation:\n\n    no CGO, no Python, no model runtime\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"indented code must be exempt:\n{r.stdout}")

    def test_inline_code_span_negatives_pass(self):
        # Backticked flag tokens are not prose; a chain of them must not trip.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "Pass `--no-cache`, `--no-index`, and `--no-model` to skip those steps.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"inline code must be exempt:\n{r.stdout}")

    def test_single_negative_sentence_passes(self):
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "It needs no external model server to run offline.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"single negative must pass:\n{r.stdout}")

    def test_two_negatives_in_separate_sentences_pass(self):
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "There is no ambiguity here. The format leaves no room for drift.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"distant negatives must pass:\n{r.stdout}")

    def test_yaml_frontmatter_is_skipped(self):
        # Frontmatter is metadata, not reader prose; a chain there must not trip.
        doc = "---\nsummary: no CGO, no Python, no model runtime\n---\n\n" + CLEAN_MD
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"frontmatter must be skipped:\n{r.stdout}")

    def test_url_in_link_does_not_trip(self):
        # A URL path like /no-cache/no-index is not prose; only the link label is.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "See the [flags reference](https://okfctl.dev/no-cache/no-index/no-model).",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"link URL must not trip:\n{r.stdout}")


class MarkdownAndHtmlShareOneDetector(unittest.TestCase):
    """The same facts in the same cadence must fail through BOTH front ends —
    proof that there is one pattern set, not two diverging copies."""

    SENTENCE = "It's pure Go, no CGO, no Python, no model runtime."

    def test_same_sentence_fails_in_both_html_and_markdown(self):
        html = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            f"<p>{self.SENTENCE}</p>",
        )
        md = f"# okfctl\n\n{self.SENTENCE}\n"
        self.assertEqual(run(html).returncode, 1, "HTML front end must fire")
        self.assertEqual(run_md(md).returncode, 1, "Markdown front end must fire")


# ---------------------------------------------------------------------------
# The SECOND class: em-dash spacing. Casey's house style is the closed-up
# Chicago em dash (word—word); a SPACED em dash (word — word) in prose is a
# finding. It rides the SAME extractors as the negative-listing check, so both
# controls the card names must hold: the POSITIVE (a spaced dash in a prose
# block fails) and the load-bearing NEGATIVE (a spaced dash inside a fenced
# code block or inline code span stays silent, because code follows its own
# medium's typography, not prose typography).
# ---------------------------------------------------------------------------

# The three prose-style spaced-dash forms, spelled with the real U+2014 so the
# fixture is exactly what a drifted doc contains.
_SPACED = "It keeps links honest \u2014 a path change never breaks a reference."
_CLOSED = "It keeps links honest\u2014a path change never breaks a reference."


class EmDashSpacingInProse(unittest.TestCase):
    """POSITIVE CONTROL. A spaced em dash in a prose block must FAIL, in both
    front ends, so the standard cannot regress once the corpus is converted."""

    def test_spaced_em_dash_in_markdown_paragraph_fails(self):
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            _SPACED,
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"spaced em dash must fail:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)

    def test_spaced_em_dash_in_markdown_heading_fails(self):
        doc = CLEAN_MD.replace("# okfctl", "# okfctl \u2014 a knowledge CLI")
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"heading prose must be scanned:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)

    def test_spaced_em_dash_in_html_paragraph_fails(self):
        page = CLEAN_PAGE.replace(
            "<p>A knowledge base decays at the moment a path changes. okfctl keeps links honest.</p>",
            f"<p>{_SPACED}</p>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"HTML prose must be scanned:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)

    def test_spaced_em_dash_in_meta_description_fails(self):
        page = CLEAN_PAGE.replace(
            'content="A single static Go binary you drop on your PATH."',
            f'content="{_SPACED}"',
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"meta prose must be scanned:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)


class EmDashSpacingLegitimateFormsPass(unittest.TestCase):
    """NEGATIVE CONTROL — the load-bearing half. The correct closed-up form, and
    a spaced dash sitting in code (fenced, inline, indented, HTML <pre>/<code>),
    a URL, or YAML frontmatter, must ALL stay silent. A gate that fires on
    correct prose or on code typography is a nuisance that gets disabled."""

    def test_closed_up_em_dash_passes(self):
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            _CLOSED,
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"closed-up dash must pass:\n{r.stdout}")

    def test_en_dash_number_range_passes(self):
        # An en dash (U+2013) in a number range is a different character and not
        # the spaced-em-dash class; it must not trip.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "The floor holds across sections 6\u20138 of the spec without exception.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"en dash range must pass:\n{r.stdout}")

    def test_spaced_em_dash_in_fenced_code_block_passes(self):
        # THE load-bearing negative control the card names: a spaced em dash in a
        # fenced code block is CLI/config sample text, not prose — stay silent.
        doc = CLEAN_MD + "\n```sh\nokfctl lint ./b   # advisory \u2014 exits 0 without --strict\n```\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"fenced code must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_tilde_fenced_code_block_passes(self):
        doc = CLEAN_MD + "\n~~~\nvalidate \u2014 always fatal on a violation\n~~~\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"tilde fence must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_indented_code_block_passes(self):
        doc = CLEAN_MD + "\nExample:\n\n    okfctl serve   # graph view \u2014 opens a browser\n"
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"indented code must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_inline_code_span_passes(self):
        # A spaced em dash inside a backticked span (e.g. a flag help string) is
        # code, not prose, and is stripped before the detector reads the block.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "Run `okfctl lint --strict   # exit 1 \u2014 any finding` to gate on it.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"inline code must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_html_code_block_passes(self):
        page = CLEAN_PAGE.replace(
            "</body>",
            "<pre><code>okfctl serve   # graph view \u2014 opens a browser</code></pre></body>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"HTML code must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_html_span_chip_passes(self):
        # <span> is excluded from prose extraction (the spec-chip row); a spaced
        # dash there is not prose typography and must not trip.
        page = CLEAN_PAGE.replace(
            "<span>no account</span>",
            "<span>no \u2014 account</span>",
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"span chip must be exempt:\n{r.stdout}")

    def test_spaced_em_dash_in_yaml_frontmatter_passes(self):
        # Frontmatter is metadata the Markdown extractor skips; the reader-facing
        # grep sweep converts frontmatter taglines, but the gate scans prose
        # BLOCKS, and a spaced dash there is out of the gate's scope by design.
        doc = "---\ntagline: honest links \u2014 always\n---\n\n" + CLEAN_MD
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"frontmatter must be skipped:\n{r.stdout}")

    def test_spaced_em_dash_in_url_passes(self):
        # A spaced-dash-shaped path segment in a link URL is not prose; only the
        # visible label is read, and a bare URL is reduced to its label.
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "See the [reference](https://okfctl.dev/a%20\u2014%20b/) for the flags.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"link URL must not trip:\n{r.stdout}")


class EmDashAndNegativeListingCoexist(unittest.TestCase):
    """Both detectors run on the same extraction pass: a doc with BOTH defects
    reports BOTH classes and the count reflects each."""

    def test_both_classes_reported(self):
        doc = CLEAN_MD.replace(
            "A knowledge base decays the moment a path changes. okfctl keeps links honest.",
            "It's pure Go, no CGO, no Python, no model runtime \u2014 nothing to install.",
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"both defects must fail:\n{r.stdout}")
        self.assertIn("NEGATIVE-LISTING", r.stdout)
        self.assertIn("SPACED-EM-DASH", r.stdout)


class GeneratedReferenceSuppressesEmDashOnly(unittest.TestCase):
    """A doc that declares itself generated from the command tree mirrors CLI
    help text; its em-dash spacing follows help-output typography, not prose,
    so the spaced-em-dash class is suppressed on it. The negative-listing class
    is NOT suppressed — a real slop cadence is a finding wherever it appears."""

    GEN_HEADER = (
        "# Command reference\n\n"
        "**This page is generated from the command tree. Don't edit it by hand.**\n\n"
    )

    def test_generated_doc_em_dash_is_suppressed(self):
        # A spaced em dash carried in from CLI help must NOT fail on a generated
        # reference doc — the card's rule: leave CLI help typography alone.
        doc = self.GEN_HEADER + (
            "validate enforces the OKF spec floor \u2014 it always fails on a violation.\n"
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 0, f"generated em dash must be exempt:\n{r.stdout}")

    def test_non_generated_doc_same_em_dash_fails(self):
        # The exact same sentence in a NON-generated doc IS a finding — proving
        # the suppression is scoped to the generated marker, not a blanket pass.
        doc = (
            "# Notes\n\n"
            "validate enforces the OKF spec floor \u2014 it always fails on a violation.\n"
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"non-generated em dash must fail:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)

    def test_generated_doc_negative_listing_still_fails(self):
        # Suppression is em-dash-only. A negative-listing cadence is a finding
        # even in a generated doc — it is slop wherever it lands.
        doc = self.GEN_HEADER + (
            "The binary is pure Go, no CGO, no Python, no model runtime.\n"
        )
        r = run_md(doc)
        self.assertEqual(r.returncode, 1, f"neg-listing must still fire:\n{r.stdout}")
        self.assertIn("NEGATIVE-LISTING", r.stdout)


def run_named(name: str, text: str):
    """Write text to a temp file with an exact basename, run the scanner on it.

    The front end is chosen by file name, so the shipped surfaces that are not
    .md/.html (llms.txt, plugin.json) need their real names in the fixture.
    """
    with tempfile.TemporaryDirectory() as d:
        p = Path(d) / name
        p.write_text(text)
        return subprocess.run(
            [sys.executable, str(SCANNER), str(p)],
            capture_output=True,
            text=True,
        )


LLMS_CLEAN = (
    "# okfctl\n\n"
    "> A command-line tool for authoring Open Knowledge Format bundles. It is a\n"
    "> single static binary written in pure Go (`CGO_ENABLED=0`).\n\n"
    "## Docs\n\n"
    "- [Command reference](https://okfctl.dev/reference/commands/): Every command\u2014generated from the binary.\n"
)


class LlmsTxtIsScannedAsMarkdown(unittest.TestCase):
    """site/public/llms.txt ships verbatim to okfctl.dev and is Markdown by the
    llms.txt convention, but its .txt extension once routed it to the HTML
    front end, which found zero prose strings in it, so it passed unread."""

    def test_clean_llms_txt_passes(self):
        r = run_named("llms.txt", LLMS_CLEAN)
        self.assertEqual(r.returncode, 0, f"expected clean:\n{r.stdout}")
        self.assertNotIn("0 prose strings", r.stdout, "llms.txt must be read, not skipped")

    def test_spaced_em_dash_in_llms_txt_fails(self):
        # Positive control: an em-dash-dense sentence planted in llms.txt.
        doc = LLMS_CLEAN + (
            "\nokfctl checks a corpus \u2014 every node \u2014 against the spec floor.\n"
        )
        r = run_named("llms.txt", doc)
        self.assertEqual(r.returncode, 1, f"planted em dash must fail:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH", r.stdout)

    def test_negative_listing_in_llms_txt_fails(self):
        # The exact line F8 of the writing audit found shipped in llms.txt.
        doc = "# okfctl\n\n> Pure Go, no CGO, no Python, no model runtime.\n"
        r = run_named("llms.txt", doc)
        self.assertEqual(r.returncode, 1, f"neg-listing must fail:\n{r.stdout}")
        self.assertIn("NEGATIVE-LISTING", r.stdout)

    def test_spaced_em_dash_in_llms_txt_code_span_passes(self):
        # Negative control: the Markdown exemptions carry over to llms.txt.
        doc = LLMS_CLEAN + "\nRun `okfctl lint \u2014 strict` in CI.\n"
        r = run_named("llms.txt", doc)
        self.assertEqual(r.returncode, 0, f"code span must be exempt:\n{r.stdout}")


PLUGIN_CLEAN = """{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "okfctl",
  "description": "Author and maintain OKF bundles from your agent\\u2014scaffold and validate nodes.",
  "keywords": ["okf"]
}
"""


class PluginJsonDescriptionIsScanned(unittest.TestCase):
    """plugin.json's description is the blurb an agent client shows when it
    lists the plugin. It is prose a reader reads; the rest of the manifest
    (schema URL, names, keywords, install commands) is data, not prose."""

    def test_clean_plugin_json_passes(self):
        r = run_named("plugin.json", PLUGIN_CLEAN)
        self.assertEqual(r.returncode, 0, f"expected clean:\n{r.stdout}")
        self.assertIn("1 prose strings", r.stdout, "the description must be read")

    def test_spaced_em_dash_in_description_fails(self):
        doc = PLUGIN_CLEAN.replace("agent\\u2014scaffold", "agent \\u2014 scaffold")
        r = run_named("plugin.json", doc)
        self.assertEqual(r.returncode, 1, f"description em dash must fail:\n{r.stdout}")
        self.assertIn("SPACED-EM-DASH in json:description", r.stdout)

    def test_negative_listing_in_description_fails(self):
        doc = PLUGIN_CLEAN.replace(
            "scaffold and validate nodes.", "pure Go, no CGO, no Python."
        )
        r = run_named("plugin.json", doc)
        self.assertEqual(r.returncode, 1, f"neg-listing must fail:\n{r.stdout}")
        self.assertIn("NEGATIVE-LISTING", r.stdout)

    def test_spaced_em_dash_outside_description_passes(self):
        # Negative control: a non-description value is data, never scanned.
        doc = PLUGIN_CLEAN.replace(
            '"name": "okfctl"', '"name": "okfctl \\u2014 no CGO, no Python"'
        )
        r = run_named("plugin.json", doc)
        self.assertEqual(r.returncode, 0, f"non-description field must be exempt:\n{r.stdout}")

    def test_malformed_json_is_usage_error(self):
        r = run_named("plugin.json", "{ not json")
        self.assertEqual(r.returncode, 2, f"malformed JSON must not pass silently:\n{r.stdout}")


# The homepage spec strip exactly as it shipped before build internals were
# dropped from selling copy.
SPEC_STRIP_AS_SHIPPED = (
    '<div class="specstrip">\n'
    "  <span>pure Go</span><span>no CGO</span><span>no Python</span>"
    "<span>no model runtime</span><span>CGO_ENABLED=0 static build</span>\n"
    "</div>"
)


class BuildInternalsInShippedCopyFail(unittest.TestCase):
    """Shipped copy must say what okfctl IS, not list build internals as selling
    points: users don't care about 'no CGO' or 'no model runtime'. Flagged,
    case-insensitively: 'no CGO', 'CGO_ENABLED' outside code, 'no model
    runtime', 'no Python'."""

    def test_homepage_spec_strip_fails(self):
        # Positive control: the chip row is exempt from the negative-listing
        # class, but NOT from this one; chips are copy a reader reads.
        page = CLEAN_PAGE.replace("</body>", SPEC_STRIP_AS_SHIPPED + "</body>")
        r = run(page)
        self.assertEqual(r.returncode, 1, f"spec strip must fail:\n{r.stdout}")
        self.assertIn("BUILD-INTERNALS in <span>: [no CGO]", r.stdout)
        self.assertIn("[CGO_ENABLED]", r.stdout)
        self.assertIn("[no model runtime]", r.stdout)
        self.assertIn("[no Python]", r.stdout)

    def test_llms_txt_line_4_as_shipped_fails(self):
        # Positive control: llms.txt as it shipped on main.
        doc = (
            "# okfctl\n\n"
            "> A command-line tool for authoring and maintaining Open Knowledge Format (OKF)\n"
            "> bundles. Pure Go \u2014 no CGO, no Python, no model runtime. okfctl does not author\n"
        )
        r = run_named("llms.txt", doc)
        self.assertEqual(r.returncode, 1, f"llms.txt line 4 must fail:\n{r.stdout}")
        self.assertIn("BUILD-INTERNALS", r.stdout)

    def test_single_mention_is_enough_and_case_insensitive(self):
        # Unlike negative-listing, ONE mention is a finding; case does not matter.
        r = run_md("# Notes\n\nIt is a static binary with No cgo at all.\n")
        self.assertEqual(r.returncode, 1, f"'No cgo' must fail:\n{r.stdout}")
        self.assertIn("BUILD-INTERNALS", r.stdout)

    def test_cgo_enabled_in_running_prose_fails(self):
        r = run_md("# Notes\n\nThe release is built with CGO_ENABLED=0 for portability.\n")
        self.assertEqual(r.returncode, 1, f"bare CGO_ENABLED must fail:\n{r.stdout}")

    def test_bulleted_build_internals_fail(self):
        # The discrete-list exemption is for the negative-listing RHYTHM; a
        # build-internal listed as a bullet is still a selling point.
        r = run_md("# Why\n\n- no Python\n- fast\n")
        self.assertEqual(r.returncode, 1, f"bullet must fail:\n{r.stdout}")

    def test_plugin_description_build_internals_fail(self):
        doc = PLUGIN_CLEAN.replace("scaffold and validate nodes.", "runs with no Python.")
        r = run_named("plugin.json", doc)
        self.assertEqual(r.returncode, 1, f"description must fail:\n{r.stdout}")
        self.assertIn("BUILD-INTERNALS in json:description", r.stdout)

    def test_html_paragraph_build_internals_fail(self):
        page = CLEAN_PAGE.replace(
            "okfctl keeps links honest.", "okfctl is static, no cgo, and keeps links honest."
        )
        r = run(page)
        self.assertEqual(r.returncode, 1, f"<p> must fail:\n{r.stdout}")


INSTALL_MD = (
    "# Install\n\n"
    "## Prebuilt binaries\n\n"
    "Download the archive for your platform.\n\n"
    "## Building from source\n\n"
    "```sh\n"
    "CGO_ENABLED=0 go build -o okfctl .      # static, no cgo\n"
    "```\n\n"
    "The source build sets CGO_ENABLED=0 so the binary is static.\n"
)


class BuildInternalsLegitimateUsesPass(unittest.TestCase):
    """Build instructions and the technical explanation of how a feature is
    built are where these words belong. Code is always exempt; two prose
    sections are exempt through an explicit allowlist."""

    def test_install_md_build_line_passes(self):
        # Negative control: install.md's CGO_ENABLED=0 build line (in a fence).
        doc = "# Install\n\n```sh\nCGO_ENABLED=0 go build -o okfctl .      # static, no cgo\n```\n"
        r = run_named("install.md", doc)
        self.assertEqual(r.returncode, 0, f"fenced build line must pass:\n{r.stdout}")

    def test_inline_code_cgo_enabled_passes(self):
        r = run_md("# Style\n\nThe build must pass with `CGO_ENABLED=0 go build ./...`.\n")
        self.assertEqual(r.returncode, 0, f"inline code must pass:\n{r.stdout}")

    def test_html_code_cgo_enabled_passes(self):
        page = CLEAN_PAGE.replace(
            "okfctl keeps links honest.", "Build it: <code>CGO_ENABLED=0 go build</code>."
        )
        r = run(page)
        self.assertEqual(r.returncode, 0, f"<code> must pass:\n{r.stdout}")

    def test_allowlisted_install_section_prose_passes(self):
        d = tempfile.mkdtemp()
        p = Path(d) / "docs" / "install.md"
        p.parent.mkdir(parents=True)
        p.write_text(INSTALL_MD)
        r = subprocess.run([sys.executable, str(SCANNER), str(p)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, f"allowlisted section must pass:\n{r.stdout}")

    def test_same_prose_outside_allowlisted_section_fails(self):
        # The allowlist is scoped to the section, not the whole file.
        d = tempfile.mkdtemp()
        p = Path(d) / "docs" / "install.md"
        p.parent.mkdir(parents=True)
        p.write_text(INSTALL_MD.replace(
            "Download the archive for your platform.",
            "Download the archive; it has no Python inside.",
        ))
        r = subprocess.run([sys.executable, str(SCANNER), str(p)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 1, f"outside the section must fail:\n{r.stdout}")
        self.assertIn("[no Python]", r.stdout)

    def test_allowlist_does_not_cover_other_files(self):
        # The same heading in a file NOT on the allowlist is not exempt.
        r = run_md(INSTALL_MD)
        self.assertEqual(r.returncode, 1, f"non-allowlisted file must fail:\n{r.stdout}")

    def test_allowlisted_search_section_passes_in_markdown_and_html(self):
        md = (
            "# Search\n\n"
            "## Core\n\nLexical search.\n\n"
            "### Real semantic search with `--embedder model2vec`\n\n"
            "The tokenizer is ported to Go, so it compiles with CGO_ENABLED=0 and no model runtime.\n"
        )
        d = tempfile.mkdtemp()
        p = Path(d) / "docs" / "guides" / "search.md"
        p.parent.mkdir(parents=True)
        p.write_text(md)
        html_page = (
            "<html><head><title>Search</title></head><body>"
            "<h2>Core</h2><p>Lexical search across the bundle.</p>"
            '<h3 id="x">Real semantic search with <code>--embedder model2vec</code></h3>'
            "<p>The tokenizer is ported to Go, so it compiles with CGO_ENABLED=0 and no model runtime.</p>"
            "</body></html>"
        )
        h = Path(d) / "guides" / "search" / "index.html"
        h.parent.mkdir(parents=True)
        h.write_text(html_page)
        r = subprocess.run(
            [sys.executable, str(SCANNER), str(p), str(h)], capture_output=True, text=True
        )
        self.assertEqual(r.returncode, 0, f"allowlisted search section must pass:\n{r.stdout}")


if __name__ == "__main__":
    unittest.main()
