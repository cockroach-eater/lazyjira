Output e2e/golden/07_url_picker.gif

Set Shell bash
Set Width 1200
Set Height 600
Set FontSize 14
Set TypingSpeed 0ms
Set Theme "Catppuccin Mocha"

Type "./lazyjira --demo"
Enter
Sleep 3s

# Navigate to SHOP-2 which has issue link (blocks SHOP-4)
Type "j"
Sleep 500ms

# Open URL picker — shows linked SHOP-4
Type "u"
Sleep 1s

# Select the link — navigates to SHOP-4 in the UI
Enter
Sleep 1s

Sleep 500ms
Type "q"
Sleep 500ms
