# okfctl—read the v0.2 provenance families with v0.1 fallback

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` **v0.2** §5 (Provenance, trust, lifecycle),
§7 (Actor convention), §11 (Conformance), §12 (Versioning), §13 (Changes from
v0.1). Upstream: GoogleCloudPlatform/knowledge-catalog `okf/SPEC.md`.
**Base:** `main` @ `b54a456`  **Branch:** `wt/t_b6bc7390`

## Problem

Upstream OKF is at **v0.2**; okfctl pins `SpecVersion = "0.1"`
(`internal/okf/reserved.go`). Nothing in the tool reads any v0.2 provenance
field. Against a hand-built v0.2 fixture, `validate` and `lint --strict` both
exit 0—which is CORRECT floor behavior (§11 forbids rejecting unknown types or
unknown frontmatter keys) but is **not support**. Passing is not understanding:
no code path reads `sources`, `generated`, `verified`, `status`, or
`stale_after`.

## Goal (READ-ONLY)

Teach the in-memory model to READ the v0.2 provenance/trust/lifecycle families as
OPTIONAL accessors on `*Node`, with the two §13.1 v0.1 fallbacks. This card adds
NO writer, NO migration, and does NOT flip `SpecVersion`. Existing v0.1 behavior
must be byte-identical (negative control).

## Design

A new pure-model file `internal/okf/provenance.go` (sibling to `node.go`,
importing no CLI package) provides accessors on `*Node`. It reads only; it never
mutates frontmatter and never adds a Validate/Lint finding.

### Families (all §5)

1. **`Sources()` (§5.1)**—parses the `sources` list into typed entries:
   `Resource` (REQUIRED per entry; entries missing it are dropped—the reader
   surfaces only well-formed sources), optional `ID`, `Title`, and credibility
   signals `Author`, `UsageCount`, `LastModified`. The `usage_window` `{from,to}`
   sibling is read once and framed onto every entry; an entry MAY carry its own
   `usage_window` to override the shared one.

2. **`Generated()` (§5.2)**—`{by, at}`; `By` REQUIRED (an actor, §7). Returns
   ok=false when neither `generated` nor the legacy fallback is usable.
   **§13.1 fallback:** when `generated` is absent, fall back to legacy
   `timestamp` for `.At` (with an empty `By`, since v0.1 recorded no author).

3. **`Verified()` (§5.2, §11 MUST)**—a list of `{by, at}` events. **A BARE
   MAPPING is normalized to a one-element list**—a §11 conformance MUST, not an
   option. Absent `verified` yields an empty list.

4. **`TrustTier()` (§5.3)**—DERIVED, never stored: no `verified` ⇒
   `unverified`; `verified` by non-`human:` actors only ⇒ `machine-confirmed`;
   any `human:<id>` verifier ⇒ `human-reviewed`. Classification keys off the
   `human:` prefix (§7).

5. **`Status()` (§5.4)**—`draft|stable|deprecated`; absent ⇒ `stable`.

6. **`StaleAfter()` / `IsStale(today)` (§5.5)**—absolute `YYYY-MM-DD`; stale
   when `today >= stale_after`.

7. **`SourceCitations()` (§13.1 fallback)**—read frontmatter `sources`, and for
   a v0.1 document with no `sources`, fall back to parsing the legacy body
   `# Citations` list (reusing the existing `citationCount` grammar).

### Version-driven behavior

Behavior is driven by the DECLARED version (`Bundle.OkfVersion`, already read from
the `.okf` sidecar). Per §12 an unrecognized version is BEST-EFFORT—never a
hard failure. The accessors themselves are version-agnostic reads (they simply
honor the fallbacks), so a v0.1 bundle reads identically to today and a v0.2
bundle gains the new families.

## Both controls (per AGENTS.md)

- **Positive:** each family and both fallbacks (`timestamp`, `# Citations`) are
  proven by a test that reads a v0.2 (and, for fallbacks, a v0.1) node.
- **Negative:** a v0.1 bundle validates and lints byte-identical to today; the
  real corpus (`validate` 0, `lint --strict` 0) is unchanged. Trust tiers, status,
  and staleness are DERIVED—nothing is written back.

## Out of scope

Writing/migrating any file; flipping `SpecVersion`; §10 Attested Computation
execution (parse/represent only; attestation runtime is deferred by §12).
