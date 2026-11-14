#!/bin/sh
# Restore disk parameters before the native daemon and its API handshake.
until /usr/config/kdbox-xbt/controller -preflight >>/tmp/kdbox-xbt-preflight.log 2>&1; do sleep 5; done
while test -f /usr/config/kdbox-xbt/halted.json || test -f /usr/config/kdbox-xbt/mining-reset.json; do sleep 2; done
exec xminerd
