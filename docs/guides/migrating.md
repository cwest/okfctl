# Migrating a bundle to OKF v0.3

`okfctl migrate` upgrades a bundle that declares OKF v0.1 or v0.2, or declares
no version, to v0.3 in place. It runs in **two phases** so it never acquires a
model dependency, and it never guesses a judgment call.

## What changes

From v0.1 (v0.3 Appendix B), two renames:

- frontmatter `timestamp` → `generated.at`
- body `# Citations` list → frontmatter `sources`

From v0.2 (v0.3 §13.1), two edits:

- A link or path field without a leading `/` resolves from the directory of its
  file in v0.3 (§6). A v0.2 bundle MAY still resolve it from the bundle root
  (§12.1). Before the bundle declares v0.3, `migrate` rewrites each value that
  resolves **only** from the bundle root to the `/`-rooted form, so it points at
  the same file under v0.3. A value that already resolves from its file, a
  `/`-rooted value, a URL, and a broken link all stay as written.
- A §5 timestamp (`generated.at`, `verified[].at`, `stale_after`,
  `sources[].last_modified`, `usage_window`) that carries an offset but isn't
  RFC 3339, for example `2026-06-20 10:00:00Z`, is respelled to
  `"2026-06-20T10:00:00Z"`. The offset is kept as written, so the instant doesn't
  move.

Last, the bundle-root `index.md` declares `okf_version: "0.3"` (§8.1). An
existing root frontmatter block keeps every other byte; an index with no block
gets an `okf_version`-only block. A legacy `.okf` sidecar, if present, is bumped
too.

## Judgment items

`migrate` writes these to the plan and leaves them unchanged:

- a prose citation with no follow-able resource (§5.1 requires one)
- a `timestamp` rename with no actor (§7)
- a date-time with no offset, for example `2026-06-20T10:00:00` (§5, §13.2: it
  names no instant, and only the author knows which offset was meant)

## Phase 1—compute the plan (pure read)

```sh
okfctl migrate ./mykb --plan migrate-plan.json --generated-by "casey"
```

This is read-only. It computes every deterministic edit and enumerates every
judgment item, writing only the plan file, and prints a summary of the counts by
kind:

```text
Wrote migrate-plan.json: 2 file(s) with deterministic edits (6 edit(s)), 3 judgment item(s).
version: 0.1 -> declare v0.3
edits: 2 file(s); timestamp->generated 2, citations->sources 3, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 3 item(s); prose-citation 3
```

## Phase 2—apply

Preview first. `--dry-run` writes nothing and prints the unified diff the real
apply would write:

```sh
okfctl migrate ./mykb --apply --plan migrate-plan.json --dry-run
```

Then apply the plan's deterministic, order-preserving, additive-only edits and
re-validate:

```sh
okfctl migrate ./mykb --apply --plan migrate-plan.json
```

A second plan on the migrated bundle carries no edits, so `migrate` is safe to
re-run.

## Bundles migrate leaves alone

A bundle that already declares v0.3 gets only content edits, and its version
stays. A bundle that declares a later version, or one okfctl doesn't recognize,
plans nothing and writes nothing: §12 says a consumer tries best-effort
consumption and doesn't refuse the bundle, and rewriting it toward v0.3 would be a
downgrade.

`--generated-by` records the actor as `generated.by` for each timestamp rename.
See `okfctl migrate --help` for the full flag set.
