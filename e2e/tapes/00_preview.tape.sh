Output e2e/golden/00_preview.gif

@start

# Look around: detail panel
@panel 0
@wait 400

# Projects panel — pick Platform Services
@panel 3
@down
@select
@wait 300

# Issues — switch to Assigned tab
@panel 2
@tab_next

# Help overlay
@help

# Open first issue (PLAT-3) detail
@open

# Fly through tabs: Body->Sub->Cmt->Lnk->Info->Hist
@tab_next 5
@wait 300

# Navigate history — description rewrite (4th block)
@down 3
@wait 200

# Expand the big diff
@expand

# Scroll the diff
@down 5
@wait 200

# Close modal
@close

# Jump to Comments tab via 'c'
@comments
@wait 300

# Scroll through comments
@down 2
@wait 200

# Go to Info tab (from Cmt: Cmt→Lnk→Info = 2 tabs)
@tab_next 2
@wait 300

# Navigate to Labels field
@down 6
@wait 200

# Edit labels — opens checklist modal
@edit
@wait 400

# Toggle a label (add "frontend")
@down 3
@toggle
@wait 200

# Confirm labels
@confirm
@wait 400

# Back to issues panel
@switch_tab

# URL picker
Type "u"
Sleep 600ms

# Navigate URL list
@down 6
@wait 200

# Select a Jira internal link — navigates to that issue
Enter
Sleep 500ms

# Transition to Done
Type "t"
Sleep 400ms
@down
Enter
Sleep 500ms

# Edit the issue summary
@edit_type [UPDATED] API auth refactor

# === JQL Search ===
# Open JQL search modal
Type "s"
Sleep 400ms

# Type field name with autocomplete
Set TypingSpeed 40ms
Type "status"
Set TypingSpeed 0ms
Sleep 300ms

# Tab to auto-complete "status"
Tab
Sleep 200ms

# Type operator — suggestions appear immediately
Set TypingSpeed 40ms
Type "in"
Set TypingSpeed 0ms
Sleep 300ms

# Pick values for IN list
Tab
Sleep 200ms
@down
Enter
Sleep 200ms

# Pick another value
@down
Enter
Sleep 200ms

# Close the IN list
Backspace
Backspace
Set TypingSpeed 40ms
Type ")"
Set TypingSpeed 0ms
Sleep 300ms

# Submit JQL search
Enter
Sleep 600ms
@wait 400

# Select first result — open detail
@select

# Open Info tab and change assignee
Type "i"
Sleep 300ms
Type "a"
Sleep 400ms
@down
Enter
Sleep 500ms

# Back to issues list
@close

# Close JQL tab
Type "x"
Sleep 300ms

@quit
