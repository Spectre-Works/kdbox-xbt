#!/bin/sh
umask 077
# Keep cooling at full output while management and telemetry initialize.
echo 100 >/sys/class/fans/fan0/lvl
echo 100 >/sys/class/fans/fan1/lvl
# Keep management available even when the native controller disables SSH.
(
 while :; do
  pidof sshd >/dev/null || /usr/sbin/sshd
  sleep 10
 done
) </dev/null >/tmp/kdbox-xbt-ssh.log 2>&1 &
exec /usr/config/kdbox-xbt/supervisor.sh
