Output demo.gif

Set Shell bash
Set FontSize 14
Set Width 1500
Set Height 820
Set Padding 12

Hide
Type "export TERM=xterm-256color"
Enter
Type "export COLORTERM=truecolor"
Enter
Type "unset NO_COLOR"
Enter
Type "export CLICOLOR=1"
Enter
Type "export CLICOLOR_FORCE=1"
Enter
Type "export PS1='$ '"
Enter
Type "clear"
Enter
Show

Type "cq"
Sleep 700ms
Enter
Sleep 2500ms

# Compact mode: short hold so grouped list is visible
Sleep 1400ms

# Switch to tab mode
Type "v"
Sleep 2600ms

# In tab mode: small, clear switching only
Down
Sleep 1800ms
Up
Sleep 1800ms

# Show plus account info quickly
Type "i"
Sleep 2800ms
Escape
Sleep 1600ms

# Demonstrate apply flow (both targets are preselected)
Type "o"
Sleep 1800ms
Enter
Sleep 1400ms
Enter
Sleep 2600ms

Type "q"
Sleep 700ms
