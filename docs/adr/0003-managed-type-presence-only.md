# 3. Manage `type` as presence, never a value allowlist

- **Status:** Accepted (2026-07-22)
- **Deciders:** Casey West
- **Sources:** PRD [§7](../PRD.md#7-managed-typethe-one-required-field), especially [§7.4](../PRD.md#74-the-hard-boundarypresence-not-a-value-allowlist)

## Context

OKF requires exactly one frontmatter field on every concept node: `type` (spec
§4.1, conformance rule 2 — *"Every frontmatter block contains a non-empty `type`
field."*). Because `type` is the single field the whole format guarantees, it is
the field every downstream consumer routes, filters, and presents on. That makes
it the natural field for okfctl to treat specially.

There is a tempting adjacent step: if okfctl manages `type`, should it also
govern the *values* of `type` — ship a known taxonomy (`Reference`, `Playbook`,
`Concept`, …), reject unfamiliar values in `validate`, and offer completion
against the known set? A value allowlist would catch typos and drift at
validation time and give the tool a strong opinion about what a bundle should
contain.

But the OKF spec is explicit in the other direction: type values are *"not
registered centrally"*, consumers *"MUST tolerate unknown types gracefully"*
(spec §4.1), and defining a fixed taxonomy of concept types is an explicit OKF
non-goal. A tool that rejected a node for an unfamiliar `type` value would be
non-conformant to the very format it manages.

## Decision

Manage the **presence and non-emptiness** of `type`, and nothing more. `validate`
hard-fails a node with a missing or empty `type` (the spec floor, non-negotiable,
never opt-out) and passes any non-empty value, however unfamiliar — a bundle full
of types okfctl has never seen still passes `validate`. `node new` requires a
non-empty `type` (prompting interactively, failing non-zero in non-interactive
use). `node show`/`node list`, `serve`, and `search` surface and filter by `type`
because it is what consumers route on. okfctl ships **no built-in list of allowed
values**.

Soft value-hygiene — flagging near-duplicate spellings of one conceptual type
(`Playbook`, `playbook`, `Play Book`) — belongs in `lint` as a **warning**, never
in `validate` as a rejection. Teams that want stricter value discipline express
it through the opt-in, team-owned template overlay (`validate --templates`,
PRD §9.4), which they version themselves — not through a taxonomy baked into the
tool.

## Consequences

**What it buys.** okfctl stays strictly conformant to OKF's anti-taxonomy stance:
it never rejects a valid-but-unfamiliar bundle, so it works on any OKF corpus,
not only ones that share its vocabulary. The floor is small, machine-checkable,
and non-negotiable, which keeps `validate` a reliable pass/fail gate. Value
discipline is still available to teams that want it, but as an opt-in overlay
they own and version, not a global policy the tool imposes.

**What it costs.** okfctl cannot catch a mistyped or drifting `type` value at
`validate` time — `type: Refernce` passes the floor exactly like `type:
Reference`. Value-hygiene help is demoted to advisory `lint` warnings and the
opt-in template overlay, both of which a user can ignore, so the tool provides no
hard guardrail against taxonomy drift. Accepting any non-empty string as a valid
`type` is a deliberate cost of conformance: the spec forbids the stronger
guarantee, so okfctl declines to offer it.
