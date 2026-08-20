# VHS tape script — https://github.com/charmbracelet/vhs
#
# Produces docs/demo.gif from a real run of scripts/demo.sh. Run from the
# repo root:
#
#   vhs docs/demo.tape
#
# Requires vhs and ffmpeg, plus the headless-Chromium shared libraries VHS
# renders through (see the README's Demo section for the apt one-liner).

Output docs/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 800
Set Theme "Dracula"
Set Padding 20

Type "./scripts/demo.sh"
Enter

# scripts/demo.sh paces itself with internal sleeps; measured at ~26s
# end-to-end, so 35s leaves comfortable margin. Adjust if you change the
# pause() durations in the script.
Sleep 35s
