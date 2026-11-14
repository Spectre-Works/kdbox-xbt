#!/bin/sh
# Lives on /root: stock factory reset erases /usr/config, not this launcher.
# It restores access and cooling only; it never restores credentials or resumes.
umask 077
runtime=/usr/config/kdbox-xbt
ready() {
    test -x "$runtime/controller" && test -x "$runtime/launcher.sh" &&
    test -x "$runtime/supervisor.sh" && test -x "$runtime/boot.sh" && test -s "$runtime/config.json" &&
    test -s "$runtime/approved.json"
}
supported() {
    test "$(cat /root/version 2>/dev/null)" = 2.2.1 &&
    test "$(cat /root/minername 2>/dev/null)" = Goldshell-KDBox &&
    test "$(cat /root/mcbversion 2>/dev/null)" = MCB_V5_4
}
cool_and_hold() {
    echo 100 >/sys/class/fans/fan0/lvl
    echo 100 >/sys/class/fans/fan1/lvl
    for p in $(pidof xminerd); do
        test "$(readlink /proc/$p/exe)" = /usr/bin/xminerd && kill -STOP "$p"
    done
    for p in $(pidof intminer kdaminer); do
        case "$(readlink /proc/$p/exe)" in /usr/bin/intminer|/usr/bin/kdaminer) kill -KILL "$p";; esac
    done
}
case "$1" in
    mining)
        while ! supported || ! ready; do cool_and_hold; sleep 5; done
        exec "$runtime/launcher.sh"
        ;;
    watch)
        # A boot hook and installer can both invoke this. Keep one watcher,
        # checking the command line so a stale/reused PID cannot claim the lock.
        lock=/var/run/kdbox-xbt-recovery.lock
        if ! mkdir "$lock" 2>/dev/null; then
            sleep 1
            pid=$(cat "$lock/pid" 2>/dev/null)
            case "$pid" in ''|*[!0-9]*) pid=0;; esac
            if test "$pid" -gt 1 && kill -0 "$pid" 2>/dev/null &&
                tr '\000' ' ' </proc/$pid/cmdline | grep -Fq '/root/kdbox-xbt-recovery.sh watch'; then exit 0; fi
            rm -f "$lock/pid"; rmdir "$lock" 2>/dev/null
            mkdir "$lock" 2>/dev/null || exit 1
        fi
        echo $$ >"$lock/pid"
        trap 'rm -f "$lock/pid"; rmdir "$lock" 2>/dev/null' EXIT
        trap 'exit 0' TERM INT
        while :; do
            if supported; then
                pidof sshd >/dev/null || /usr/sbin/sshd
                ready || cool_and_hold
            else cool_and_hold
            fi
            sleep 10
        done
        ;;
    *) exit 2;;
esac
