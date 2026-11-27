# Docs writing audit and proposed direction (v0.5.0)

Status: proposal. Nothing here merges until a direction is picked.
Base: `main` at `d990129` (v0.5.0). Spec: OKF v0.3
([SPEC.md](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md),
version line checked: `**Version 0.3**`).

PR #232 made every page factually current for OKF v0.3. This is the pass
that comes after it: how the prose reads, if the pages show the tool working,
and if the claims still check out against the binary and the spec. Every
command and output quoted here came from a `v0.5.0` binary built with
`go install github.com/cwest/okfctl@v0.5.0`.

## Recommendation in one line

Lead every task page with a real run of the current binary, explain after
it, and put the reference detail below the run. The three samples in this PR
show what that looks like.

## Inventory and scores

Score is the aidetector prose model, 0 (reads human) to 100 (reads machine),
with 40 as the target. It describes the text, not who wrote it. "Em dash"
is the share of prose paragraphs carrying one; the house limit is 10%.
"Output shown" counts blocks of real command output on the page.

| Doc | Words | Score | Em dash | Output shown | First-time reader hits it |
| :-- | --: | --: | --: | --: | :-- |
| `README.md` | 1,011 | 97 | 52% | 0 | yes, first |
| okfctl.dev homepage (`index.astro`, visible text of the build) | 937 | 95 | 12% | 5 tabs | yes, first |
| `okfctl --help` (root; 206 words of output, mostly the command list) | 6 | too short | n/a | n/a | yes |
| `docs/install.md` | 199 | 87 | 67% | 0 | yes |
| `docs/README.md` | 292 | 75 | 60% | 0 | yes |
| `docs/concepts.md` | 624 | 95 | 25% | 0 | yes ("read this first") |
| `docs/guides/authoring.md` | 285 | 45 | 14% | 0 | yes (the quickstart guide) |
| `docs/guides/migrating.md` | 776 | 96 | 0% | 1 (not reproducible from the page) | upgrade path |
| `docs/guides/curation-health.md` | 1,314 | 99 | 30% | 0 | |
| `docs/guides/index-and-freshness.md` | 530 | 98 | 62% | 0 | |
| `docs/guides/search.md` | 670 | 96 | 44% | 0 | |
| `docs/guides/remote-sources.md` | 205 | 44 | 50% | 0 | |
| `docs/guides/plugins.md` | 196 | 53 | 25% | 0 | |
| CLI help, all 49 commands' Short/Long (source of `docs/commands/README.md`) | 3,115 | 98 | 41% (27 spaced) | n/a | yes, via `--help` |
| `okfctl migrate --help` | 282 | 91 | 100% (one paragraph, spaced dash) | n/a | upgrade path |
| `skills/okf-authoring/SKILL.md` | 1,601 | 98 | 44% | some | agents |
| `skills/okf-curation-health/SKILL.md` | 2,012 | 98 | 36% | some | agents |
| `skills/okf-migrate-plan/SKILL.md` | 3,334 | 100 | 54% | some | agents |
| `skills/okf-semantic-search/SKILL.md` | 1,489 | 99 | 33% | some | agents |
| `site/public/llms.txt` | 200 | 89 | 40% (2 spaced) | n/a | answer engines |
| `plugin.json` description | 33 | too short | 1 spaced | n/a | agent clients |

Totals: about 18,800 words of reader-facing prose. Of the 19 rows with a score,
16 are above 70. The three that score under 60 (`authoring`, `remote-sources`,
`plugins`) are the three shortest guides, and short pages carry less of the
signal the model counts. Read those three as unknown, not as good.

## Findings, ranked by reader impact

### F1. The first things a new reader sees describe the tool instead of showing it

The README opens with a metaphor ("Markdown knowledge bases don't rot because
people stop writing") and then a definition stacked with three em dashes. Its
quickstart is eight commands with comments and no output, and the reader can't
tell what success looks like. It also teaches a step no reader needs. `node new`
rebuilds both `index.md` files and logs the node itself, and that makes the
`index build` right after it redundant (v0.5.0: `node new` then `index check`
prints `OK: index.md is current`).

The homepage is the one page that shows real runs today. Sample 1 brings the
README up to it.

### F2. Spec citations that point at the wrong section of v0.3

Each of these cites a section number that means something else in v0.3 or doesn't exist at all.

| Where | Says | v0.3 actually |
| :-- | :-- | :-- |
| `docs/concepts.md` | non-empty `type` is the floor "(OKF §7)" | `type` is §4.1; §7 is the actor convention |
| `docs/concepts.md` | `type` values are open "(§7.4)" | no §7.4 exists; open types are §4.1 ("Consumers MUST accept unknown types") |
| `docs/guides/authoring.md` | "a non-empty `type` is the OKF spec floor, §7" | §4.1 |
| `README.md`, homepage band | overlay "(`--templates`, §9.4)" | §9 is log files; §9.4 is a section of `docs/PRD.md`, which a reader can't tell |
| `cmd/node.go` `node new` Short | "type required, PRD §7" | the binary's own error says `OKF §4.1 / §11` |

A reader who follows any of these lands on the wrong rule. The AGENTS.md
"cite the section" convention makes this worse, because a wrong citation looks
authoritative.

### F3. The homepage shows output the binary doesn't print

- The `lint --strict` tab shows `mentions "tannin"`. v0.5.0 prints
  `missing-xref: concepts/oak.md mentions "Tannin" but does not link to concepts/tannin.md`,
  with the node's title casing.
- The "at scale" tab is styled as a transcript but prints `nodes: hundreds`
  and `Thousands of internal link(s)`. The binary prints numbers. A comment in
  the page source says the real counts were dropped on purpose because they
  rot. Dropping them was right. Putting a placeholder where output goes was
  not, since a transcript can't contain text the binary never printed. Either
  show a real run of a public bundle, or label the tab as a description.
- The `eval transparency` tab lists `uncited` ahead of `grade-missing`; v0.5.0
  prints `grade-missing` first for each node.

### F4. Examples teach an actor that skips the human trust tier

The old `migrate --help` example, the old migrating guide, and the generated
command reference all pass `--generated-by "casey"`. OKF v0.3 §7: "producers
MUST use `human:` for content that a person wrote or confirmed," and only
`human:` earns the human-reviewed tier (§5.3). okfctl accepts the bare string
(it writes `generated: {by: casey, ...}`) and nothing warns. The samples use
`human:casey`. The skills get it right (`human:alex`).

### F5. Guides explain without showing

Six of the seven guides contain no command output at all. The seventh,
`migrating.md`, showed one summary from a fixture the page never shows. A
reader couldn't reproduce `7 edit(s), 2 judgment item(s)`. Sample 2 rebuilds
the page around one bundle the reader can see whole.

### F6. Help text is one paragraph per command, written for the author

`okfctl migrate --help` was 282 words in a single paragraph, with
"PURE READ", "consumer-agnostic", "deterministic edit", and "the plan's
consumer (agent, colleague, shell loop, or a human)". It's accurate and hard to
read at a terminal. Over all 49 commands, 27 paragraphs use a spaced em dash.
The repo rules exempt CLI help from the closed-dash style, fairly enough for
terminal output, but the same strings render as prose on okfctl.dev through
`docs/commands/README.md`. Sample 3 rewrites one command.

### F7. Em dashes run several times over the house limit

Eighteen of the 19 scored rows exceed the 10% house limit, 14 of them by three
times or more. The one at 0% is the migrating guide. One habit drives it
everywhere: a bold label, then an em dash, then the gloss
(`**Homebrew**—install…`, `**Author**—build and edit…`).

### F8. The gates don't see two shipped files

`scan_page_prose.py` scans the built HTML and the Markdown, but not
`site/public/llms.txt` or the `plugin.json` description. The first line of
`llms.txt` sets "Pure Go" off with a spaced em dash. What follows it is a
negative-listing cadence the gate was written to block after that cadence
shipped twice. It got through because the file is `.txt`.

### F9. Smaller things

- The homepage mixes British spelling ("behaviour", "Reorganising") into an
  otherwise US-spelled project.
- `docs/concepts.md` opens "Read this first; it makes every other page make
  sense." The sentence promises the page is important. The page should
  just start.
- The four skills score 98-100 at 1,489-3,334 words. Agents are the readers,
  and the register can stay denser than a guide's, but the length is worth a
  look: `okf-migrate-plan` runs 3,334 words, more than twice the longest guide.

### Checked and correct

`bundle info` output in the README (`nodes: 1`, `reserved: 3`,
`okf_version: 0.3 (declared in index.md)`). Nineteen top-level commands on the
homepage (20 in `--help`, counting `help`). Prebuilt binaries for macOS, Linux,
and Windows on amd64 and arm64, plus deb and rpm (`.goreleaser.yaml`). Cosign
keyless signing of `checksums.txt` (`.goreleaser.yaml` `signs`). Every runnable
command has a Long and an Example (`TestHelpSurface_EveryRunnableCommandHasLongAndExample`
plus a pass over all 49 `--help` outputs). The migrating guide's change list
against v0.3 §6, §6.2, §8.1, §12.1, §13.1 and Appendix B, and its claim that
re-applying a plan changes nothing (re-applied on a scratch copy: no diff).
`validate` exits 1 on unparseable frontmatter and `lint --strict` exits 1 on a
finding (both run).

## The direction

There are three real options, and they pull in different directions.

**O1. Show, then explain (recommended).** A task page opens with a real run of
the current binary on a bundle small enough to see whole. Then it says what
just happened. The reference detail sits underneath for whoever needs it. Write
in second person with plain verbs and contractions, one idea to a paragraph,
and no em dash unless the sentence breaks without it.

On the README, that means the tool catches a defect on the first screen. A
guide follows one bundle from start to finish with its output. Help text gets
three or four short paragraphs (what the command does and how to run it, what it
changes, what it leaves for you, and what it won't do). Concepts stays an
explainer, but each idea gets one command that proves it. Spec citations stay,
checked against v0.3.

The cost is real. Shown output rots when the binary changes, and O1 needs a test
that replays the transcripts (follow-up A2).

**O2. Reference first.** Pages stay terse, like man pages, with a one-sentence
summary, the flags, an example, and no walkthroughs. It's cheaper to write and
maintain, and it suits someone who knows OKF. For the reader who arrives from the
README not knowing what a bundle is, it does nothing, and that's most readers.

**O3. One long tutorial.** One story from `bundle init` to CI, with each topic
guide cut to a stub. It makes the best first read and the worst lookup. All the
guide content ends up in one file, and changing any command means editing it.

My pick is O1. It answers F1 and F5 head-on, and the transcript test it needs
would have caught F3 (the homepage casing) and the migrate summary no reader could
reproduce.

## The three samples

This PR carries each one as a before/after diff.

| Sample | Before | After | Loop result |
| :-- | --: | --: | :-- |
| README top section (to `## Install`) | 97 | 39 | PASS |
| `docs/guides/migrating.md` | 96 | 54 | STOP-PLATEAU after 16 accepted rounds |
| `okfctl migrate --help` (Long text, regenerated into `docs/commands/README.md`) | 91 | 42 | STOP-PLATEAU after 11 accepted rounds |

Two of the three missed the 40 target. In the guide, the opening paragraph
never cooled off. Over the 20 recorded rounds it went through several
rewrites, and as a segment it never scored under 82. The reference half has to
repeat spec terms too. What's left in the help
text is "plan" and the `. 2` n-gram that numbered spec sections produce, and
neither can go without taking the citations with it.

Past the score, the samples changed these things.

The README opener says who the tool is for and what it does, in two sentences.
Its quickstart is now a run that catches a real `missing-xref` finding. You see
`validate` pass, `lint` report the finding, and `lint --strict` fail on it, and
the text explains each result. I cut the redundant `index build` and wrote one
line saying why you can skip it. Nothing under `## Install` changed. That is
why the file as a whole still scores 92.

In the migrating guide, one two-node v0.1 bundle goes through the plan, the
preview, the apply, one resolved judgment item, and the v0.3 declaration, and
you see every file and every line of output (all from v0.5.0). The reference
sections come after, carrying the old page's facts, with `human:casey` as the
example actor (F4).

`migrate --help` has four short paragraphs where it had one long one. "PURE
READ", "consumer-agnostic", and "two-phase" are gone, along with the spaced em
dash. The spec citations all survive, and the examples now use `./mykb` and
`human:casey`.

## Follow-up cards (proposed, not filed)

These are sized for after a direction is picked. A1 and A12 are worth doing
whichever direction wins.

| Code | Card | Size | Kind |
| :-- | :-- | :-- | :-- |
| A1 | Fix the wrong facts: §-citations (F2), homepage output (F3), `--generated-by` actors (F4), `llms.txt` cadence (F8) | S | docs, touches `cmd/node.go` and `index.astro` |
| A2 | Transcript test: replay every `$ okfctl` block in `docs/` and the README against the built binary, fail on drift | M | code |
| A3 | Rest of the README, plus `docs/README.md` and `docs/install.md` | M | docs |
| A4 | Guides, part 1: `authoring`, `index-and-freshness`, `curation-health` | M | docs |
| A5 | Guides, part 2: `search`, `remote-sources`, `plugins` | M | docs |
| A6 | `docs/concepts.md`, one proving command per idea | M | docs |
| A7 | CLI help, author group: `bundle`, `node`, `index`, `log` (regenerate the reference) | M | docs |
| A8 | CLI help, check and explore groups: `validate`, `lint`, `eval`, `analyze`, `search`, `graph`, `serve` | M | docs |
| A9 | CLI help, extend group: `template`, `registry`, `connect`, `plugin`, `config`, `completion`, `version` | S | docs |
| A10 | Homepage copy (`index.astro`), including the "at scale" tab | M | docs |
| A11 | The four skills plus `plugin.json` and `llms.txt`, in an agent register | M | docs |
| A12 | Extend `scan_page_prose.py` to `llms.txt` and the `plugin.json` description | S | code |

If O1 wins, A2 should land ahead of A3-A10 so each rewrite ships with its
transcripts under test from day one.
