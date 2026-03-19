Output e2e/golden/03_issue_detail.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# Focus detail panel directly with Tab (issue already loaded via prefetch)
Tab
Sleep 1s

# Cycle all detail tabs: Body -> Sub -> Cmt -> Lnk -> Info -> Hist
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 400ms
Type "]"
Sleep 1s

# Now on Hist tab — first block is description change (newest, big diff)
# Expand with Space
Space
Sleep 2s

# Inside readOnly modal with full diff — scroll down
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# Dismiss modal
Escape
Sleep 500ms

# Go back with [
Type "["
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
