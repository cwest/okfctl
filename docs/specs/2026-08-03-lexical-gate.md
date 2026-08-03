# Spec: `--lexical-gate` — gate semantic results lexically, preserving lexical recall

Closes cwest/okfctl#66.
Branch: `wt/t_1e16ab69`  Base: `main` @ ac8a7a5

## Problem

`okfctl-search --semantic` is embedding-only; `okfctl search` (core) is lexical,
unranked substring matching. The two surfaces never meet. On an
exact-identifier-shaped query (`ERR_EVICT_7741`, a verbatim-unique token) the
semantic top-1 is often a near-miss because embeddings blur exact tokens, while
the lexical surface would rank the exact match first but cannot rank at all.

Three mechanisms, all reproduced on the real corpus
(`~/src/knowledge-base/bundles/knowledge`, 234 indexed nodes, base `main`
@ ac8a7a5):

1. **`--field body` is phrase-wise**, so a question-shaped query returns nothing:
   `okfctl search --field body "how should an agent decide when to delegate work"`
   → **0 hits**. A naive gate that reused this phrase-wise match would empty every
   result list for the common query shape. The gate MUST match **term-wise with
   stopwords dropped**.
2. **The substring match is raw and asymmetric.** Real-corpus body hit counts:
   `agent` 172, `agents` 100, `hash` 18, `hashes` 0. `agent` matches more than
   `agents` because it is a raw byte substring of it; `hashes` matches nothing
   while `hash` matches 18. The gate MUST normalize morphology (stem, or match
   both directions) so `hash`/`hashes` gate to overlapping sets.
3. **A pure gate is a quality regression on the default embedder.** The issue's
   own `hash`-embedder run scores a pure gate 0.386 vs 0.677 for lexical alone,
   because gold is often outside the semantic top-N and a strict intersection then
   discards a correct lexical hit. Preserving the lexical tail (step 4 below) is
   what makes this a win on both embedders.

## Design

Add `--lexical-gate` to `okfctl-search`, **off by default**. When on, the query
pipeline is:

1. Rank the semantic query wide (top-N, N well above `--k`; N = 50 by default,
   never below `--k`).
2. Compute the **term-wise lexical match set**: tokenize the query, drop
   stopwords, stem each remaining term, and match a node when any stemmed query
   term equals a stemmed token of the node's title+body.
3. Emit the intersection (semantic ∩ lexical) in **semantic order**.
4. **Append the lexical hits the semantic band missed**, in lexical (path) order.
5. Cut to `--k`.

### Degrade to pure semantic — the correctness rule

The gate is a **no-op** (byte-identical to gate-off) when either:

- the query has **no content terms** after stopword removal (an all-stopword
  query like `"how should the"`), or
- the lexical match set covers **more than `overBroadFraction` of the bundle**
  (default **0.60**). `agent` matches 172/234 = 73% of the real corpus and MUST
  degrade; a term that matches most of the corpus carries no discriminating
  signal, so gating on it only reorders noise.

This degrade rule is what keeps question-shaped queries — the common case —
exactly as good as they are today, and it is why the gate can be added without
regressing the default embedder.

### Where the lexical match resolves

Term-wise matching needs node title+body text, which the index does not carry
(the `Store` holds vectors, not prose; `contentHash` keys on title+body only).
So the gate resolves against the **live bundle at query time**, exactly like the
§4.1 scoping filters and §5.2 recency decay already do. The command loads the
bundle when the gate is on (it already loads it when a filter or `--half-life` is
set).

### Tokenization and stemming

- **Tokenize:** lowercase; split on any non-alphanumeric rune; drop empty tokens.
- **Stopwords:** a small closed set of English function words plus the
  question-shaped leaders proven necessary by the 0-hit finding (`how`, `should`,
  `an`, `agent` is NOT a stopword — content words stay). See `lexgate.go`.
- **Stem:** a deliberately light suffix stripper — strip a trailing `es`/`s`
  (plurals) and `ing`/`ed` (verb inflections) with a minimum stem length so
  short tokens are left intact. This is enough to make `hash`/`hashes` and
  `agent`/`agents` collapse to the same stem without the over-stemming risk of a
  full Porter stemmer. `okfctl` is a spec consumer, not an NLP toolkit; the
  lightest transform that fixes the proven asymmetry is the right one.

### Interaction with filters (#63) and decay (#65)

The gate composes with the existing pipeline, not around it:

- **Filters** run PRE-ranking. The semantic band is already filtered, and the
  lexical match set is intersected with the same filter (a lexical-only hit that
  fails the filter is not appended). So `--lexical-gate --path wine/` never
  surfaces a node outside `wine/`.
- **Decay** runs POST-ranking on the semantic scores. The gate reorders and
  extends the *result list*; decay's relevance floor still keys on raw cosine.
  Lexical-only appended hits carry their semantic cosine (they were scored in the
  wide band even if below the top-N cut) so decay has a real score to work on.

A combined-flags smoke test asserts no panic and sane output for both
combinations. No silent resolution of ambiguous interaction: filters constrain
both bands (documented), decay reorders the semantic band only (documented).

## Acceptance criteria

- **Positive:** an exact-token query where the semantic top-1 is a near-miss —
  with `--lexical-gate` the exact match ranks 1, without it it does not.
- **Negative control (load-bearing):** the full question-shaped gold set scores
  **no worse** with the gate on than off, on BOTH embedders (`hash` and
  `model2vec`). Four numbers pinned in the PR body.
- **Second negative:** gate **off** (default) is byte-identical to `main` on
  every query shape.
- **Empty-term degrade:** an all-stopword query with the gate on == gate off.
- **Over-broad degrade:** a term matching > `overBroadFraction` of the bundle
  (`agent` at 73%) makes the gate a no-op.
- **Stemming symmetry:** `hash` and `hashes` gate to overlapping result sets.
- **Interaction:** `--lexical-gate` with `--path` and with `--half-life` — no
  panic, sane output.

## Spec citations

- §4.1 (frontmatter: title/type/tags) — the fields the lexical surface and the
  filters key on, v0.2.
- The gate adds no spec-defined behavior; it is a ranking overlay behind an
  explicit opt-in flag, consistent with the PRD's "floor for everyone, extras
  behind opt-in" line.
