Output e2e/golden/02_navigation.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# Navigate down in issues list
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Navigate up
Type "k"
Sleep 500ms

# Switch to detail panel with Tab
Tab
Sleep 500ms

# Switch back with Tab
Tab
Sleep 500ms

# Focus all panels by number: 0 detail, 1 status, 2 issues, 3 projects
Type "1"
Sleep 500ms

Type "0"
Sleep 500ms

Type "3"
Sleep 500ms

Type "2"
Sleep 500ms

# Test issue tabs with [ and ]
Type "]"
Sleep 500ms

Type "["
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
