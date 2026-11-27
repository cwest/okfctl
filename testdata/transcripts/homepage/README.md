# Homepage transcript fixture

`stackoverflow/` is a byte-for-byte copy of `bundles/stackoverflow/` from
[open-knowledge-format/open-knowledge-format](https://github.com/open-knowledge-format/open-knowledge-format/tree/527e81aff55992be21507e97004286789126f88c/bundles/stackoverflow)
at commit `527e81a`, licensed Apache-2.0, minus its `viz.html` (a generated
viewer, not part of the bundle). The okfctl.dev homepage's "real bundle" panel
replays against it, so the counts on that panel come from a bundle anyone can
fetch and check.

To refresh it, copy the upstream directory over this one, drop `viz.html`, and
update the numbers the homepage shows until `go test ./internal/transcript/`
passes.
