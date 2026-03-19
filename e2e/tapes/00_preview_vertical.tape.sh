Output e2e/golden/00_preview_vertical.gif

@start_vertical

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

# Navigate to description rewrite (4th block, index 3)
@down 3
@wait 400

# Expand the big diff
@expand

# Scroll the diff
@down 5
@wait 400

# Close modal
@close

# Back to issues for URL picker
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

@quit
