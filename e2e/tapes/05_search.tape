Output e2e/golden/05_search.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# Activate search
Type "/"
Sleep 500ms

# Type filter text
Set TypingSpeed 50ms
Type "checkout"
Sleep 1s

# Confirm search
Enter
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
