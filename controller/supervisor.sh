#!/bin/sh
umask 077
while :; do
    started=$(date +%s)
    /usr/config/kdbox-xbt/controller >>/tmp/kdbox-xbt-controller.log 2>&1 &
    child=$!
    sleep 15
    while kill -0 "$child" 2>/dev/null; do
        now=$(date +%s)
        last=$(cat /tmp/kdbox-xbt-heartbeat 2>/dev/null)
        case "$last" in ''|*[!0-9]*) last=0;; esac
        test "$last" -ge "$started" || last=0
        if { test "$last" -gt 0 && test $((now-last)) -gt 45; } || { test "$last" = 0 && test $((now-started)) -gt 180; }; then
            # Independent watchdog covers a wedged controller, including a performance adjustment.
            for p in $(pidof xminerd); do kill -STOP "$p"; done
            for p in $(pidof intminer kdaminer); do kill -KILL "$p"; done
            echo 100 >/sys/class/fans/fan0/lvl
            echo 100 >/sys/class/fans/fan1/lvl
            kill -KILL "$child"
            break
        fi
        sleep 5
    done
    wait "$child"
    sleep 2
done
