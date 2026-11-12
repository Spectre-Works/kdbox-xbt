#!/bin/sh
cd "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
sh ./install.sh
RESULT=$?
if [ "$RESULT" -ne 0 ]; then
 printf '\nSee docs/installer-hosts.md. Press Enter to close. '
 read -r REPLY
fi
exit "$RESULT"
