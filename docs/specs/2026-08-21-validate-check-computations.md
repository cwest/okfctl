# Spec—`validate --check-computations`: the OKF §10 attested-computation contract shape

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Closes:** #142 (triaged from the #147 roadmap)

## Why

OKF §10 defines the Attested Computation concept—a `type: Attested Computation`
node that carries not just what a value *means* but a sanctioned way to *compute*
it (§10.1). §10.2 fixes the contract fields: `runtime` is REQUIRED for this type;
`parameters` entries are `{ name, type, required }`; `computation` is optional,
and its absence means the body `# Computation` fence is the computation (§10.3);
`executor` and `attester` carry a `resource`. Today okfctl is blind to this
type—`grep -rni attest --include='*.go'` returns nothing. This increment gives it
a front door: the three structural contract checks §10 makes checkable, behind an
opt-in flag.

OKF fixes the interface, not the packaging, and does not execute anything itself
(§10, §10.5 informative). So this is a **structural shape** check only: it reads
frontmatter and the node body, and it never reads, resolves the contents of, or
executes anything named by `computation`, `executor`, or `attester`.

## What

`validate --check-computations` runs three structural checks that apply **only to
`type: Attested Computation` nodes**. Every other node—and every check when the
flag is absent—is untouched.

### Placement: `validate` behind the flag (resolved, argued in the PR)

Placed on `validate`, not `lint`, because `runtime` being REQUIRED for this type
(§10.2) is a **shape rule**, not curation guidance—the same class as `type` being
required at the floor. It sits behind `--check-computations` so it never joins the
unconditional floor (`validate` with no flags stays byte-identical to today), and
so a bundle with no attested computations is entirely unaffected. This mirrors the
existing `--templates` opt-in overlay on `validate` (§9.4).

### The three checks (all §10.2 / §10.3)

1. **`runtime` present and non-empty (§10.2).** Missing or empty/whitespace
   `runtime` → finding. `runtime` is REQUIRED for this type.
2. **The computation is provided exactly one way (§10.3).** §10.3 says provide the
   computation in **one of two ways**: an inline body `# Computation` fence, OR a
   `computation` path with the body fence omitted.
   - Neither present → finding.
   - Both present → a DISTINCT finding naming the ambiguity (which one governs is
     unspecified, so it is surfaced rather than silently resolved).
   - A `computation` path that does not resolve to a file on disk → finding naming
     the unresolved path. Resolution is §6.2: relative to the node's directory,
     within the bundle root; the file is `os.Stat`'d only, never read.
3. **`parameters` entry well-formedness (§10.2).** An entry is
   `{ name, type, required }`; `name` is the identity of the hole. An entry
   missing `name` → finding. Entries missing only `type` or `required` → NO
   finding (those are shape hints the spec does not make mandatory per-entry;
   holding the permissive line).

### The permissive line (§11, load-bearing)

Missing `executor`, missing `attester`, and absent `parameters` are **NOT**
findings. §11 forbids rejecting a concept for a missing optional family, and
over-conformance is the failure mode that ships by accident because it looks like
rigor (AGENTS.md). The negative control test pins this.

## Done when

- Checks apply only to `type: Attested Computation`; inert for every other node.
- `validate` with no flags is byte-identical to today (golden-tested).
- Missing/empty `runtime` → finding (§10.2).
- Neither `computation` path nor body `# Computation` fence → finding (§10.3).
- Both present → a DISTINCT finding naming the ambiguity (§10.3).
- Unresolvable `computation` path → finding naming the unresolved path (§10.3/§6.2).
- **Negative control:** missing `executor`, missing `attester`, absent
  `parameters` produce no finding (§11).
- `parameters` entry missing `name` → finding; entries missing only `type` or
  `required` → no finding (§10.2).
- A test asserts no subprocess is spawned and nothing named by `computation`,
  `executor`, or `attester` is read or executed.
- Fixtures cover one conformant node plus one instance of each failure, with the
  §-number cited in each finding message and in the test name.
- **Real-corpus control:** the 254-node corpus has zero nodes of this type;
  `validate --check-computations` returns OK there, unchanged from the flagless
  run.
- `internal/okf` stays cobra-free (pure model—`CheckComputations(b) []Finding`);
  `cmd/validate.go` wires the flag. Full `-race` suite green; gofmt / vet clean.

## Scope / Routing

Repo: `cwest/okfctl` · branch: topic (worktree) · PR: draft. Closes #142.

## Explicitly out of scope

- Executing, reading, or resolving the *contents* of any `computation`,
  `executor`, or `attester` resource. OKF does not execute; neither does this.
- Attestation itself (per-run, runtime, not stored in the bundle—§10.5, §10.6).
- Enum-gating `runtime` values, `parameters[].type` values, or any optional
  family. §11 unknown-key/permissive tolerance stands.
