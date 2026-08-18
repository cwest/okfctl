# Migrating a v0.1 bundle to v0.2

OKF v0.2 renames two things (§13.1): frontmatter `timestamp` → `generated.at`,
and the body `# Citations` list → frontmatter `sources`. Consumers fall back to
the legacy forms, so a v0.1 bundle stays readable — but you can convert one in
place with `okfctl migrate`.

`migrate` runs in **two phases** so it never acquires a model dependency, and it
never guesses a judgment call.

## Phase 1 — compute the plan (pure read)

```sh
okfctl migrate ./mykb --plan migrate-plan.json --generated-by "casey"
```

This is read-only. It computes every deterministic §13.1 edit and enumerates
every **judgment item** — a prose citation with no follow-able resource (§5.1),
or a `timestamp` rename with no actor (§7) — writing only the plan file.
Judgment items are left in the plan for a human (or agent) to resolve; they're
never auto-guessed.

## Phase 2 — apply

Preview first (byte-identical to the real apply, writes nothing):

```sh
okfctl migrate ./mykb --apply --plan migrate-plan.json --dry-run
```

Then apply the plan's deterministic, order-preserving, additive-only edits and
re-validate:

```sh
okfctl migrate ./mykb --apply --plan migrate-plan.json
```

`--generated-by` records the actor as `generated.by` for each timestamp rename.
See `okfctl migrate --help` for the full flag set.
