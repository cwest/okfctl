# Homepage transcript fixture

This is what the homepage's "at scale" panel replays. It's
`bundles/stackoverflow/` from
[open-knowledge-format/open-knowledge-format](https://github.com/open-knowledge-format/open-knowledge-format/tree/527e81aff55992be21507e97004286789126f88c/bundles/stackoverflow)
at `527e81a` (Apache-2.0), byte for byte, minus `viz.html` (that's a generated
viewer). Want to check the panel's counts? Fetch that commit.

To refresh it, copy upstream over `stackoverflow/` and delete `viz.html`.
`go test ./internal/transcript/` will then tell you which homepage numbers
changed. Fix them until it passes.
