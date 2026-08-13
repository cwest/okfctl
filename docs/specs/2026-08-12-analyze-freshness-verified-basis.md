# analyze freshness: prefer the §5.2 `verified` instant as the basis

> A corpus-wide reformat that rewrites every node's `modified:` must not disarm
> the freshness signal. `analyze` now dates a node from its content-verification
> instant (§5.2 `verified[].at`), falling back to the prior
> `generated.at → modified → created` chain, so mechanical edits and claim
> re-verification are finally separate axes.

## Problem

`analyze`'s `freshness.stale` and `freshness.time_sensitive` signals both derive
a node's age from `freshnessBasis`, which resolved `generated.at → modified →
created`. A 2026-08 bulk reformat rewrote every corpus node's `modified:` to
~Aug 2026 (measured distribution: 08-04 ×158, 08-02 ×60, 08-08 ×18, 08-07 ×9),
while `created:` stayed genuinely spread across Jun–Jul 2026. Because `modified`
outranked `created` in the basis order, every node read as freshly edited and
**both freshness signals returned zero** across all 246 nodes — even at
`--stale-days 45 --time-sensitive-fraction 0.25`. The signal stays blind until
~2026-Feb (180d after the touch). By its own contract `analyze` was correct
(recently-`modified` nodes are not stale); the defect is that a mechanical
reformat conflated "the file was rewritten" with "the claims were re-verified".

## Decision

The fix lives in **okfctl** (the analyzer), not the corpus. OKF already defines a
content-verification key distinct from `modified`: §5.2 `verified[] {by, at}`
with §5.3 derived trust tiers, and okfctl already reads it (`Node.Verified()`,
`Node.TrustTier()`). The only gap was that `freshnessBasis` never consulted it.

`freshnessBasis` now resolves in this order:

1. **`verified[].at` (§5.2)** — the LATEST verification instant across the event
   list. This is the freshness axis: it records when a node's *claims* were last
   re-confirmed. A present `verified` date takes precedence over every
   mechanical-edit key below, so a bulk `modified:` touch no longer resets the
   clock.
2. **`generated.at` (§5.2)** with the legacy `timestamp` fallback (§13.1).
3. **`modified` → `created`** — okfctl-native compatibility, unchanged.

Both signals inherit the corrected basis for free: `stale` compares the
verified-derived age to `--stale-days`; `time_sensitive` compares it to
`--time-sensitive-fraction × --stale-days`. A marker-bearing node that was
`modified`-touched but never `verified` falls back to its (old) `created` date
and correctly surfaces for re-verification — no new special-case path.

### Why not the alternatives

- **A brand-new `verified:`/`reviewed:` convention key.** Unnecessary — the spec
  already has `verified` (§5.2) with a working reader. Inventing a parallel key
  would fragment provenance.
- **A content-signal path that surfaces marked nodes regardless of age.** Rejected
  as primary fix: it discards the age gate the operator tuned with
  `--time-sensitive-fraction`, trading one blind spot (everything quiet) for
  another (everything loud). Preferring the `verified` basis restores a *truthful*
  age instead of bypassing the gate.
- **A git-blame-of-content backfill.** A corpus-side data task, not a tool change.
  It complements this fix but is independently routed (see below).

## Basis order (single source of truth)

`verified[].at` → `generated.at` (→ legacy `timestamp`) → `modified` → `created`
→ `(none)` (undated, flagged softly). Documented at the `freshnessBasis` doc
comment, the `--stale-days` flag help (regenerated into
`docs/commands/README.md`), and here.

## Corpus follow-up (not blocking this fix)

The tool fix restores the *mechanism*; the corpus still carries no `verified:`
dates, so until nodes are stamped the basis falls through to `created` (which is
honest and pre-migration). Two follow-ups belong to the corpus owner, routed
separately:

1. Begin stamping `verified:` on nodes as their claims are re-checked (the
   standing freshness sweep is the natural trigger).
2. Optionally backfill an initial `verified[].at` from git blame of each node's
   *content* (not the reformat commit), undoing the reset for existing nodes.

## Tests

`internal/okf/analyze_freshness_verified_test.go`:

- `VerifiedBeatsModified` — fresh `modified`, old `verified` ⇒ stale (basis is
  the verified date).
- `LatestVerifiedWins` — old + recent verified events ⇒ the recent one wins ⇒
  fresh.
- `VerifiedBeatsGenerated` — old `generated.at`, recent `verified` ⇒ fresh
  (locks the full precedence).
- `NoVerifiedFallsBackToModified` — no `verified` ⇒ prior behavior preserved.
- `TimeSensitiveUsesVerifiedAge` — marker + fresh `modified` + old `verified` ⇒
  surfaces; fresh `verified` ⇒ quiet.

All pre-existing freshness tests (`StaleByModified`, `ModifiedFallsBackToCreated`,
`GeneratedAtBeatsModified`, `ResolvesLegacyTimestamp`, …) remain green — the
change is additive.
