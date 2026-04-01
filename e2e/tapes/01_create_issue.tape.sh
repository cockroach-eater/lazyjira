Output e2e/golden/01_create_issue.gif

@start

# select Platform Services project
@panel 4
@down
@select

# switch to Assigned tab, cursor lands on PLAT-3
@panel 2
@tab_next

# duplicate with ctrl+n
Ctrl+n
Sleep 400ms

# type picker: select Bug
@down
Enter
Sleep 600ms

# create form opens with prefilled fields from PLAT-3
# clear summary and type a new one
Ctrl+a
Ctrl+k
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

# edit priority to High
@edit
@down
@confirm

# assignee: pick Demo User (ourselves so it shows in Assigned tab)
@down
@edit
@confirm

# labels: pick a few
@down
@edit
@toggle
@down
@toggle
@down
@toggle
@confirm

# components: pick API and Frontend
@down
@edit
@toggle
@down
@toggle
@confirm

# review: scroll through fields
@down
@wait 300

# go back to summary
Tab
Sleep 300ms

# check description
Tab
Sleep 200ms

# back to fields
Tab
Sleep 300ms

# submit
Tab
Sleep 200ms
Enter
Sleep 600ms

# issue created, detail view shows new issue immediately
@wait 500
@quit
