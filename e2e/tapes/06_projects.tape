Output e2e/golden/06_projects.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# Focus projects panel
Type "3"
Sleep 500ms

# Navigate to second project (Platform Services)
Type "j"
Sleep 500ms

# Select project — issues should reload
Type " "
Sleep 2s

# Switch to issues panel to see PLAT issues
Type "2"
Sleep 500ms

# Test issue tabs on new project
Type "]"
Sleep 500ms

Type "["
Sleep 500ms

Sleep 500ms
Type "q"
Sleep 500ms
