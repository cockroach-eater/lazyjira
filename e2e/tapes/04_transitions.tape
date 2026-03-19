Output e2e/golden/04_transitions.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# SHOP-1 is first issue, status "In Progress"
# Open transition modal
Type "t"
Sleep 1s

# Select first transition (Submit Review)
Enter
Sleep 2s

# Status should now be changed — visible in issue list
# Switch to detail panel to verify
Tab
Sleep 500ms

# Navigate to Hist tab to verify transition recorded
# Hist is last tab: ] ] ] ] ]
Type "]"
Sleep 300ms
Type "]"
Sleep 300ms
Type "]"
Sleep 300ms
Type "]"
Sleep 300ms
Type "]"
Sleep 500ms

# Scroll down to see the new history entry
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
