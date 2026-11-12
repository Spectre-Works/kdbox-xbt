#!/bin/sh
# Downloads a checksum-pinned uv bootstrap; Python/libraries stay in this folder.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT=$(pwd)
fail() { printf '\nSetup stopped: %s\nSee docs/installer-hosts.md or https://github.com/Spectre-Works/kdbox-xbt/issues\n' "$1" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || fail 'curl is required; see the host setup guide.'
command -v tar >/dev/null 2>&1 || fail 'tar is required; see the host setup guide.'
case "$(uname -s)/$(uname -m)" in
 Linux/x86_64) TARGET=x86_64-unknown-linux-gnu; SHA=c5221ff41b7892bfc223543883600125f6a47fe845093e0f5bdff4f359196d83;;
 Linux/aarch64|Linux/arm64) TARGET=aarch64-unknown-linux-gnu; SHA=71279f8d3c67241e807b9d8db872d1bf541579fb9f08b7d1eb1682196d1e57dc;;
 Darwin/x86_64) TARGET=x86_64-apple-darwin; SHA=20c47e4c1f630b8abaff8c291fa9b9aa300fef92c4fbff95adf198b55287cda7;;
 Darwin/arm64) TARGET=aarch64-apple-darwin; SHA=81b811b57e4624df7ed3aac148e0359100d9962f3d9dce00aec533dc2e086731;;
 *) fail 'This launcher supports 64-bit Linux and macOS on Intel/AMD or ARM. See the manual Python setup guide.';;
esac
mkdir -p .installer
UV="$ROOT/.installer/uv-0.13.7-$TARGET"
if [ ! -x "$UV" ]; then
 printf 'Preparing the installer (first launch needs Internet access)…\n'
 TEMP=$(mktemp -d "$ROOT/.installer/download.XXXXXX")
 trap 'rm -rf "$TEMP"' EXIT HUP INT TERM
 curl --fail --location --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 180 --retry 2 \
  "https://github.com/astral-sh/uv/releases/download/0.13.7/uv-$TARGET.tar.gz" -o "$TEMP/uv.tar.gz" || fail 'Could not download the runtime manager. Check Internet access and retry.'
 if command -v sha256sum >/dev/null 2>&1; then ACTUAL=$(sha256sum "$TEMP/uv.tar.gz");
 elif command -v shasum >/dev/null 2>&1; then ACTUAL=$(shasum -a 256 "$TEMP/uv.tar.gz");
 else fail 'A SHA-256 utility is required; see the host setup guide.'; fi
 [ "${ACTUAL%% *}" = "$SHA" ] || fail 'Runtime manager checksum mismatch. Nothing was executed.'
 tar -xzf "$TEMP/uv.tar.gz" -C "$TEMP" "uv-$TARGET/uv"
 mv "$TEMP/uv-$TARGET/uv" "$UV"
 chmod 700 "$UV"
 rm -rf "$TEMP"
 trap - EXIT HUP INT TERM
fi
export UV_PYTHON_INSTALL_DIR="$ROOT/.installer/python"
export UV_CACHE_DIR="$ROOT/.installer/cache"
export UV_NO_MODIFY_PATH=1
if [ ! -x .venv/bin/python ]; then
 "$UV" venv --managed-python --python 3.13 .venv || fail 'Python setup failed. See the host setup guide for supported systems.'
fi
.venv/bin/python -c 'import sys; assert sys.version_info >= (3,12)' || fail 'The existing .venv is too old. Rename it and launch again.'
# Preserve a user's replaceable LGPL library instead of reinstalling every time.
if ! .venv/bin/python -c 'import paramiko' >/dev/null 2>&1; then
 "$UV" pip install --python .venv/bin/python -r requirements.txt || fail 'Dependency download failed. Check Internet access and retry.'
fi
exec .venv/bin/python -m flasher
