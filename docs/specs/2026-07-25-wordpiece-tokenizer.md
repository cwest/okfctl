# Spec: Increment 5c-2 - WordPiece tokenizer + wire --embedder model2vec (pure Go)

Status: Approved  Owner: Casey West  License: Apache-2.0
Increment: 5c-2 (second half of 5c; completes the pure-Go Model2Vec embedder, native per 13.2)
Depends on: 5c-1 (ReadSafetensorsMatrix, StaticModel, EncodeIDs — all merged on main)
Protocol source of truth: minishlab/potion-base-8M tokenizer.json (BERT WordPiece) +
model2vec StaticModel.encode (which tokenizes with add_special_tokens=FALSE)

## Goal

Complete okfctl-search's production embedder: a pure-Go BERT WordPiece tokenizer that
turns TEXT into the content token IDs 5c-1's StaticModel.EncodeIDs consumes, then wire it
behind --embedder model2vec so the whole pipeline (text -> ids -> vector) is a single
static, zero-CGO binary. Verified for byte-level fidelity against the real potion-base-8M
via model2vec's own output.

## The tokenizer (ground-truthed against tokenizer.json)

Standard BERT WordPiece stack, three stages, all pure-Go stdlib:

1. BertNormalizer {clean_text:true, handle_chinese_chars:true, lowercase:true,
   strip_accents:null}:
   - clean_text: strip control chars, normalize whitespace runs to single space.
   - handle_chinese_chars: surround each CJK codepoint with spaces (so each is its own token).
   - lowercase: Unicode lowercase. (strip_accents:null => with lowercase on, BERT default
     does NOT strip accents — "café" stays "café" pre-tokenized; matched against vocab as-is.)
2. BertPreTokenizer: split on whitespace AND split every punctuation char into its own token
   (a run "notes." -> "notes","."). Punctuation = Unicode P* plus the ASCII punct BERT treats
   as punct.
3. WordPiece {continuing_subword_prefix:"##", unk_token:"[UNK]", max_input_chars_per_word:100}:
   greedy longest-match-first from the vocab; first subword matched bare, continuations need
   "##" prefix; a word with no match (or >100 chars) -> single [UNK] id. Vocab from vocab.txt
   (line N = id N; 29528 entries, matching the 5c-1 embedding matrix rows).

CRITICAL: model2vec calls the tokenizer with add_special_tokens=FALSE. The Go tokenizer
MUST return content ids only — NO [CLS]/[SEP] wrapping. (Raw HF encode wraps 101...102;
model2vec strips them; we never add them.)

## Fidelity anchors (captured from the live model)

Tokenize (content-only ids):
- "tannin structure" -> [8098, 10489, 2258]
- "Wine"             -> [3517]         (lowercased -> "wine")
- "oaky vanilla notes" -> [5122, 1106, 20167, 2970]  (note "oaky"->oak+##y, WordPiece split)
- ""                -> []
Full pipeline (Tokenize -> StaticModel.EncodeIDs), model2vec.encode("tannin structure"):
- dim 256, L2 norm 1.0, first 4 = [0.236271, -0.08241, -0.142059, -0.152239]

## Package surface (internal/search, stdlib-only, no cobra/net-http/CGO)

- LoadWordPiece(dir string) (*WordPiece, error): read dir/tokenizer.json (or vocab.txt) into
  vocab map[string]int + unk id + "##" prefix. Prefer vocab.txt (simple id=line-number) with
  tokenizer.json read only for the normalizer/wordpiece params (which are the fixed BERT
  defaults above, so may be hard-defaulted with a config cross-check).
- (t *WordPiece) Tokenize(text string) []int: normalize -> pre-tokenize -> WordPiece each
  word -> flat content id slice (NO specials).
- type Model2VecEmbedder struct { model *StaticModel; tok *WordPiece; name string }
  implementing the 5b Embedder interface: Name()/Dim()/Encode([]string) [][]float64 where
  Encode(texts) = for each text: model.EncodeIDs(tok.Tokenize(text)).
- LoadModel2VecEmbedder(dir string) (*Model2VecEmbedder, error): LoadStaticModel(dir) +
  LoadWordPiece(dir); Name() = the model dir/name.

## Command wiring (cmd/okfctl-search)

- --embedder model2vec resolves to LoadModel2VecEmbedder, replacing the 5b honest-defer error.
  It needs a model DIR. Resolution order for the dir (config-first, no new dependency):
    1. --model-path <dir> flag (optional per-invocation override)
    2. the "model_path" key in okfctl's EXISTING JSON config (~/.config/okfctl/config.json,
       the increment-1 config store; read via the same loadConfig/configPath mechanism),
       settable with `okfctl config set model_path <dir>`
    3. neither present -> a clear error telling the user to set model_path via
       `okfctl config set model_path <dir>` or pass --model-path.
  A Model2Vec dir has config.json + model.safetensors + tokenizer.json/vocab.txt. NO network
  fetch at runtime (okfctl thesis: no separate install / no runtime download). hash stays the
  default embedder. RATIONALE: model_path is a stable per-machine setting, not a per-invocation
  flag — config-first avoids the "retype it every run" friction. JSON (not TOML) keeps okfctl
  stdlib-only / zero-dependency; a TOML migration, if ever wanted, is its own separate increment.

## Boundaries / decisions

- stdlib only (strings, unicode, unicode/utf8, encoding/json, os). NO tokenizers/HF lib, NO CGO.
- Fidelity guarantee is scoped honestly: EXACT on the captured anchors + a representative
  corpus of real KB node text; a hand-ported WordPiece can diverge on exotic Unicode edge
  cases, so the test asserts parity on real content, flagging (not silently absorbing) any
  divergence. This is the weaker-than-HashEmbedder guarantee flagged at the 5c brainstorm.
- Model asset (safetensors + tokenizer, ~30 MB) is NOT vendored in the repo; --model-path
  points at it. (Embedding it as a build asset is a later packaging call, not 5c-2.)
- No new module deps; CGO_ENABLED=0 must build.

## Done criteria (unit + real-model fidelity)

1. Tokenize matches every captured anchor exactly (content-only, no specials); ""->[];
   lowercase applied ("Wine"->[3517]); WordPiece subword split ("oaky"->oak+##y); an
   all-unknown word -> [unk id]; a >100-char word -> [unk id].
2. Model2VecEmbedder implements Embedder; Encode("tannin structure") == model2vec.encode
   output within 1e-5 (dim 256, unit norm, first4 anchor) — a REAL end-to-end fidelity test
   gated on the potion-base-8M model being present (skip w/ clear message if absent, run in CI
   only where the model is cached).
3. `okfctl-search --embedder model2vec --model-path <dir> --semantic "q"` ranks a bundle;
   missing/invalid --model-path -> clear error (no panic, no silent hash fallback).
4. `--embedder hash` unchanged (default, offline). internal/search stays
   cobra/net-http/CGO-free; full -race green; no new deps; CGO_ENABLED=0 builds.
