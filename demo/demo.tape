# Records docs/assets/demo.gif and demo.mp4. Run by .github/workflows/demo.yml
# after demo/setup.sh /srv has built the fixture. Every command shown is real.

Output docs/assets/demo.gif
Output docs/assets/demo.mp4

Require restorable
Require restic

Set Shell "bash"
Set FontSize 18
Set Width 1280
Set Height 820
Set Padding 30
Set TypingSpeed 40ms
Set Theme { "name": "Restorable", "black": "#151518", "red": "#E63B5A", "green": "#7FC88A", "yellow": "#E8C37A", "blue": "#8AAFE0", "magenta": "#C792C6", "cyan": "#7FC4C8", "white": "#F2E9D8", "brightBlack": "#6F6A62", "brightRed": "#FF5C77", "brightGreen": "#9EE0A8", "brightYellow": "#F5D79A", "brightBlue": "#A9C6F0", "brightMagenta": "#DDB0DC", "brightCyan": "#9ED9DC", "brightWhite": "#FFFFFF", "background": "#0B0B0D", "foreground": "#F2E9D8", "selection": "#3A1A22", "cursor": "#E63B5A" }

Hide
Type "export PS1='$ ' RESTIC_PASSWORD=demo-only && cd /srv/demo && clear"
Enter
Sleep 500ms
Show

Type "# A Vaultwarden server, backed up nightly with restic."
Enter
Sleep 600ms
Type "# Backups you've never restored aren't backups. Let's restore one."
Enter
Sleep 900ms

Type "restorable init --repo /srv/backups/restic --yes"
Sleep 400ms
Enter
Wait@60s
Sleep 4s

Type "clear"
Enter
Type "# Night two: the backup job copies the database mid-write."
Enter
Sleep 600ms
Type "truncate -s 64K /srv/vaultwarden/db.sqlite3 && restic backup -q -r /srv/backups/restic /srv/vaultwarden"
Sleep 400ms
Enter
Wait@60s
Sleep 800ms

Type "# restic says the backup succeeded. Restorable restores it and checks."
Enter
Sleep 700ms
Type "restorable test --config agent.yaml"
Sleep 400ms
Enter
Wait@60s
Sleep 1s
Type "echo exit code $?"
Enter
Sleep 5s
