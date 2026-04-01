Output e2e/golden/01_create_issue.gif

@start

# select project and issue
@panel 4
@down
@select

# focus issues panel, press n to create
@panel 2
@create

# type picker: select Bug
@down
Enter
Sleep 600ms

# create form opens, summary focused
# type the summary
Set TypingSpeed 40ms
Type "Login page crashes on expired token refresh"
Set TypingSpeed 0ms
Sleep 300ms

# tab to description
Tab
Sleep 200ms

# tab to fields
Tab
Sleep 200ms

# edit priority: press e, select High
@edit
@down
@confirm

# move to assignee, edit with e
@down
@edit

# pick Alice Chen
@down
@down
@confirm

# move to labels, edit with e
@down
@edit

# toggle two labels
@toggle
@down
@toggle

# confirm checklist
@confirm

# scroll down to see all fields
@down

# go back to summary to review
Tab
Sleep 200ms
Sleep 300ms

# tab to description
Tab
Sleep 200ms

# tab to fields
Tab
Sleep 200ms
Sleep 300ms

# submit with enter from summary
Tab
Sleep 200ms
Enter
Sleep 600ms

# issue created, back to issues list
Sleep 500ms
Type "q"
Sleep 300ms
