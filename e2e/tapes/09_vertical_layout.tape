Output e2e/golden/09_vertical_layout.gif

Set Shell bash
Set Width 550
Set Height 700
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# In vertical layout — panels stacked, collapsed bars
# Navigate issues
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Select issue
Type "l"
Sleep 1s

# Switch panels
Type "3"
Sleep 500ms

Type "2"
Sleep 500ms

Type "0"
Sleep 500ms

# Cycle detail tabs
Type "]"
Sleep 500ms
Type "]"
Sleep 500ms
Type "]"
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
