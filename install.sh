#!/bin/sh
# Editable source installer. Dependencies are installed locally, not bundled.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
command -v python3 >/dev/null 2>&1 || { echo 'Install Python 3.12+ with Tk support first.' >&2; exit 1; }
python3 -c 'import sys; assert sys.version_info >= (3, 12), "Python 3.12+ required"'
if [ ! -x .venv/bin/python ]; then
    python3 -m venv .venv
fi
PYTHON=.venv/bin/python
"$PYTHON" -c 'import tkinter' || { echo 'Install your distribution Tk package (often python3-tk), then retry.' >&2; exit 1; }
# Do not reinstall on every launch: users may replace or modify their LGPL library.
if ! "$PYTHON" -c 'import paramiko' >/dev/null 2>&1; then
    "$PYTHON" -m pip install -r requirements.txt
fi
exec "$PYTHON" -m flasher
