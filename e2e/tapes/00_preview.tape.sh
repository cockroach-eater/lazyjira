Output e2e/golden/00_preview.gif

@start

# Projects panel — pick Platform Services
@panel 4
@down
@select

# Issues — switch to Assigned tab
@panel 2
@tab_next

# Open PLAT-3 (space = activate + open)
@select

# Detail tabs: Body→Cmt→Hist
@tab_next 2

# History — navigate to description diff (block 3)
@down 3

# Expand the big diff, scroll, close
@expand
@down 5
@close

# Comments
@comments
@down 2

# === Info panel [3] ===
Type "i"
Sleep 100ms

# Browse fields, edit labels
@down 6
@edit
@wait 200
@down 3
@toggle
@confirm
@wait 200

# Lnk tab — preview linked issue
@tab_next
@open

# Sub tab — activate subtask
@tab_next
@down
@select

# Back to info, h/l cycling demo
Type "i"
Sleep 100ms
Type "l"
Sleep 150ms
Type "l"
Sleep 150ms
Type "l"
Sleep 150ms

# Help
@help

# Edit summary
@panel 2
@edit_type [UPDATED] API auth refactor

# Transition
Type "t"
Sleep 200ms
@down
Enter
Sleep 200ms

# JQL Search
Type "s"
Sleep 200ms
Set TypingSpeed 30ms
Type "status"
Set TypingSpeed 0ms
Tab
Sleep 100ms
Set TypingSpeed 30ms
Type "in"
Set TypingSpeed 0ms
Tab
Sleep 100ms
@down
Enter
Sleep 100ms
@down
Enter
Sleep 100ms
Backspace
Backspace
Set TypingSpeed 30ms
Type ")"
Set TypingSpeed 0ms
Enter
Sleep 300ms

# Select result, change assignee from info
@select
Type "i"
Sleep 100ms
Type "a"
Sleep 200ms
@down
Enter
Sleep 200ms

@close
Type "x"
Sleep 100ms

@quit
