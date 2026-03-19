#!/usr/bin/env bash
# VHS tape preprocessor — expands shorthand commands into VHS tape syntax.
#
# Usage in .tape files:
#   Source e2e/tape.sh    (ignored by VHS, parsed by preprocessor)
#   @start                → Output + Set Shell/Width/Height/FontSize/Theme + launch demo
#   @start_vertical       → same but narrow (550px) for vertical layout
#   @down [N]             → Type "j" + Sleep (repeated N times, default 1)
#   @up [N]               → Type "k" + Sleep
#   @tab_next [N]         → Type "]" + Sleep
#   @tab_prev [N]         → Type "[" + Sleep
#   @panel N              → Type "N" + Sleep (focus panel 0-3)
#   @open                 → Type "l" + Sleep (open issue in detail)
#   @select               → Space + Sleep (select item)
#   @transition           → Type "t" + Sleep + Enter + Sleep (pick first transition)
#   @search TEXT          → Type "/" + Sleep + Type "TEXT" + Enter + Sleep
#   @help                 → Type "?" + Sleep ... Escape + Sleep
#   @expand               → Space + Sleep (expand block in detail)
#   @close                → Escape + Sleep (close modal/overlay)
#   @quit                 → Type "q" + Sleep
#   @wait [MS]            → Sleep Nms (default 500)
#   @switch_tab           → Tab + Sleep (switch left/right panel)
#
# Run: ./e2e/tape.sh e2e/tapes/foo.tape | vhs -

set -euo pipefail

DEFAULT_SLEEP=400
LONG_SLEEP=800

repeat() {
    local n="${1:-1}"
    shift
    for ((i = 0; i < n; i++)); do
        "$@"
    done
}

process_line() {
    local line="$1"

    # Strip comments on @-lines
    line="${line%%#*}"
    line="${line%"${line##*[![:space:]]}"}" # rtrim

    case "$line" in
        @start)
            echo 'Set Shell bash'
            echo 'Set Width 1200'
            echo 'Set Height 600'
            echo 'Set FontSize 14'
            echo 'Set TypingSpeed 0ms'
            echo 'Set Theme "Catppuccin Mocha"'
            echo ''
            echo 'Type "./lazyjira --demo"'
            echo 'Enter'
            echo 'Sleep 2s'
            ;;
        @start_vertical)
            echo 'Set Shell bash'
            echo 'Set Width 550'
            echo 'Set Height 600'
            echo 'Set FontSize 14'
            echo 'Set TypingSpeed 0ms'
            echo 'Set Theme "Catppuccin Mocha"'
            echo ''
            echo 'Type "./lazyjira --demo"'
            echo 'Enter'
            echo 'Sleep 2s'
            ;;
        @down*)
            local n="${line#@down}"; n="${n// /}"; n="${n:-1}"
            repeat "$n" printf 'Type "j"\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @up*)
            local n="${line#@up}"; n="${n// /}"; n="${n:-1}"
            repeat "$n" printf 'Type "k"\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @tab_next*)
            local n="${line#@tab_next}"; n="${n// /}"; n="${n:-1}"
            repeat "$n" printf 'Type "]"\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @tab_prev*)
            local n="${line#@tab_prev}"; n="${n// /}"; n="${n:-1}"
            repeat "$n" printf 'Type "["\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @panel\ *)
            local p="${line#@panel }"; p="${p// /}"
            printf 'Type "%s"\nSleep %sms\n' "$p" "$DEFAULT_SLEEP"
            ;;
        @open)
            printf 'Type "l"\nSleep %sms\n' "$LONG_SLEEP"
            ;;
        @select)
            printf 'Space\nSleep %sms\n' "$LONG_SLEEP"
            ;;
        @transition)
            printf 'Type "t"\nSleep %sms\nEnter\nSleep %sms\n' "$LONG_SLEEP" "1000"
            ;;
        @search\ *)
            local text="${line#@search }"
            printf 'Type "/"\nSleep %sms\n' "$DEFAULT_SLEEP"
            printf 'Set TypingSpeed 50ms\nType "%s"\nSet TypingSpeed 0ms\n' "$text"
            printf 'Sleep %sms\nEnter\nSleep %sms\n' "$LONG_SLEEP" "$DEFAULT_SLEEP"
            ;;
        @help)
            printf 'Type "?"\nSleep 1s\nEscape\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @expand)
            printf 'Space\nSleep 1500ms\n'
            ;;
        @close)
            printf 'Escape\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @quit)
            printf 'Sleep %sms\nType "q"\nSleep 300ms\n' "$DEFAULT_SLEEP"
            ;;
        @wait*)
            local ms="${line#@wait}"; ms="${ms// /}"; ms="${ms:-500}"
            printf 'Sleep %sms\n' "$ms"
            ;;
        @switch_tab)
            printf 'Tab\nSleep %sms\n' "$DEFAULT_SLEEP"
            ;;
        @*)
            echo "# WARNING: unknown directive: $line" >&2
            ;;
        *)
            echo "$line"
            ;;
    esac
}

# Main: read tape file, expand @-directives, pass through everything else
input="${1:--}"
while IFS= read -r line || [[ -n "$line" ]]; do
    process_line "$line"
done < "$input"
