"""Validate a complete local payload before the installer modifies a miner."""
import hashlib
import json
from pathlib import Path
import struct
from . import __version__

WEB_FONTS = ('ChakraPetch-Regular.woff2', 'ChakraPetch-Bold.woff2')
FONT_LICENSE = 'licenses/chakra-petch-OFL.txt'
LEGAL_FILES = ('LICENSE', 'NOTICE', 'BRANDING.md', 'licenses/upstream-MIT.txt', FONT_LICENSE)
WEB_ASSETS = (
    'index.html', 'app.js', 'settings.js', 'style.css', 'favicon-32.png', 'favicon-48.png', 'spectre-logo.png',
    'licenses.html', 'open-source.txt', *WEB_FONTS,
)
SCRIPTS = ('launcher.sh', 'supervisor.sh', 'boot.sh')
RECOVERY_SCRIPT = 'recovery-bootstrap.sh'
RECOVERY_PATH = '/root/kdbox-xbt-recovery.sh'
PAYLOAD_FILES = (
    *LEGAL_FILES,
    'build/kdbox-xbt-controller', 'controller/compatibility.json',
    'flasher/web/spectre-wordmark.png',
    *(f'controller/{name}' for name in SCRIPTS),
    'controller/'+RECOVERY_SCRIPT,
    *(f'controller/web/{name}' for name in WEB_ASSETS),
)


def validate_controller(binary):
    """Check ELF32, little endian, ARM and a valid executable header.

    This is an architecture/integrity check, not a signature or malware check.
    The build fixes GOARM=7 and CGO_ENABLED=0; do not substitute vendor binaries.
    """
    if len(binary) < 52 or binary[:7] != b'\x7fELF\x01\x01\x01':
        raise RuntimeError('Controller must be a complete little-endian ELF32 ARM build.')
    kind, machine, version = struct.unpack_from('<HHI', binary, 16)
    if kind not in (2, 3) or machine != 40 or version != 1 or struct.unpack_from('<H', binary, 40)[0] != 52:
        raise RuntimeError('Controller is not a supported ARM executable. Rebuild with tools/build.py.')


def _pairs(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError('Duplicate manifest key')
        result[key] = value
    return result


def payload_hashes(root):
    root = Path(root).resolve()
    result = {}
    for relative in PAYLOAD_FILES:
        path = root / relative
        if path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(root):
            raise RuntimeError(f'Missing or unsafe payload file: {relative}')
        result[relative] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result


def validate_payload(root):
    root = Path(root).resolve()
    try:
        if {path.name for path in (root / 'controller/web').iterdir()} != set(WEB_ASSETS):
            raise ValueError('Unexpected embedded web asset')
        path = root / 'build/payload.json'
        if path.is_symlink() or not path.resolve().is_relative_to(root) or path.stat().st_size > 65536:
            raise ValueError('Invalid manifest file')
        manifest = json.loads(path.read_text(encoding='utf-8'), object_pairs_hook=_pairs)
        if (manifest.get('schema') != 1 or manifest.get('version') != __version__ or manifest.get('target') != 'linux/arm/v7'
                or manifest.get('cgo') is not False
                or manifest.get('files') != payload_hashes(root)):
            raise ValueError('Payload files do not match the manifest')
        notices = (root / 'controller/web/open-source.txt').read_text(encoding='utf-8')
        if (any((root / name).read_text(encoding='utf-8').strip() not in notices for name in LEGAL_FILES)
                or 'Go toolchain notices' not in notices):
            raise ValueError('Required distribution notices are missing')
        binary = (root / 'build/kdbox-xbt-controller').read_bytes()
        validate_controller(binary)
        return binary
    except (OSError, ValueError, TypeError, AttributeError) as exc:
        raise RuntimeError('Incomplete or mismatched installer payload. Extract the complete package or run python tools/build.py. No installation files were changed.') from exc
