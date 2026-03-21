Output e2e/golden/00_preview.gif

@start

# Look around: detail panel
@panel 0
@wait 800

# Projects panel — pick Platform Services
@panel 3
@down
@select
@wait 600

# Issues — switch to Assigned tab
@panel 2
@tab_next

# Help overlay
@help

# Open first issue (PLAT-3) detail
@open

# Fly through tabs: Body->Sub->Cmt->Lnk->Info->Hist
@tab_next 5
@wait 600

# Navigate history — description rewrite (4th block)
@down 3
@wait 400

# Expand the big diff
@expand

# Scroll the diff
@down 5
@wait 400

# Close modal
@close

# Jump to Comments tab via 'c'
@comments
@wait 600

# Scroll through comments
@down 2
@wait 400

# Go to Info tab (from Cmt: Cmt→Lnk→Info = 2 tabs)
@tab_next 2
@wait 600

# Navigate to Labels field (Status,Priority,Assignee,Reporter,Type,Sprint,Labels = 6 down)
@down 6
@wait 400

# Edit labels — opens checklist modal
@edit
@wait 800

# Toggle a label (add "frontend")
@down 3
@toggle
@wait 400

# Confirm labels
@confirm
@wait 800

# Back to issues panel
@switch_tab

# URL picker — shows URLs grouped by Body/Comments/Links with separators
Type "u"
Sleep 1500ms

# Navigate down through the URL list to see separators and Jira links
@down 6
@wait 400

# Select a Jira internal link (PLAT-1) — navigates to that issue
Enter
Sleep 1000ms

# Now on PLAT-1 — transition it to Done
Type "t"
Sleep 800ms
# Navigate to "Mark Done" (second option)
@down
Enter
Sleep 1000ms

# Edit the issue summary (rename task) — already on issues panel after navigateToIssue
@edit_type [UPDATED] API auth refactor

@quit
