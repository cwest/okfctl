# VHS tape for the okfctl 60-second quickstart demo.
#
# Regenerate the GIF from the repo root with the REAL binary on PATH:
#
#   CGO_ENABLED=0 go build -o /tmp/okfctl . && PATH="/tmp:$PATH" \
#     vhs docs/assets/quickstart.tape
#
# The recorded sequence is the cold-start flow documented in the README's
# 60-second quickstart: bundle init -> node new -> node list -> index build
# -> index check -> log append -> validate -> bundle info.

Output docs/assets/quickstart.gif

Set Shell "bash"
Set FontSize 16
Set Width 1000
Set Height 640
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set PlaybackSpeed 1.0

# Start from a clean scratch directory so the demo is reproducible.
Hide
Type "cd $(mktemp -d) && clear"  Enter
Show

Type "okfctl bundle init mykb"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl node new concepts/tannin.md --type Reference --title Tannin --bundle mykb"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl node list --bundle mykb"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl index build mykb"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl index check mykb"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl log append mykb --message 'added tannin node'"  Sleep 400ms  Enter  Sleep 900ms
Type "okfctl validate mykb"  Sleep 400ms  Enter  Sleep 1200ms
Type "okfctl bundle info mykb"  Sleep 400ms  Enter  Sleep 2500ms
