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
<span>pure Go</span><span>no CGO</span><span>no Python</span><span>no model runtime</span>
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
            "<p>It is pure Go, no CGO, no Python, no model runtime.</p>",
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

Build guarantees:

- pure Go
- no CGO
- no Python
- no model runtime
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
            "It's pure Go, no CGO, no Python, no model runtime.",
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


if __name__ == "__main__":
    unittest.main()
