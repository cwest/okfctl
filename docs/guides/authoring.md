# Starting and authoring a bundle

A cold-start walkthrough: from an empty directory to a validated bundle.

## 1. Scaffold a bundle

```sh
okfctl bundle init mykb
```

This creates the two reserved files (`index.md`, `log.md`) and the bundle-root
`.okf` sidecar. It does **not** create any concept nodes—a fresh bundle has
zero nodes.

## 2. Add a node

```sh
okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
```

`--type` is required (a non-empty `type` is the OKF spec floor, §7). When a type
template governs `--type`, the node is scaffolded from it: required fields are
stubbed with `TODO` placeholders, recommended fields stubbed empty, and the
template's body sections laid down as empty `##` headings.

List what's there:

```sh
okfctl node list --bundle mykb
```

## 3. Build and check the index

```sh
okfctl index build mykb      # generate the reserved index.md files
okfctl index check mykb      # confirm they are current
```

## 4. Record the change and validate

```sh
okfctl log append mykb --message "added tannin node"
okfctl validate mykb         # OK: bundle conforms to the OKF spec floor
okfctl bundle info mykb      # nodes: 1, reserved: 3, okf_version: 0.2
```

## Moving and removing nodes

A node's path is its identity, so a move is a graph operation:

```sh
okfctl node mv concepts/tannin.md concepts/tannins.md --bundle mykb --dry-run
```

`node mv` rewrites every inbound link, preserving each author's relative link
form; `--dry-run` shows what would change without writing. `okfctl node rm`
removes a node and reports any nodes orphaned as a result. Both take `--bundle`.

After authoring, keep the index current (see
[Index and freshness](index-and-freshness.md)) and check curation health (see
[Curation health](curation-health.md)).
