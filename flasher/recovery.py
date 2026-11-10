"""Versioned, allowlisted recovery snapshots; never accept arbitrary write paths."""
import hashlib
from pathlib import Path
import re
from .payload import WEB_ASSETS, SCRIPTS

LEGACY_PATHS = (
    '/etc/inittab', '/root/yottamc/index.html', '/root/settings/algosetting.json',
    '/usr/config/minerd/settings/algosetting.json', '/usr/config/minerd/settings/.algosetting.json',
    '/usr/config/minerd/settings/cpbsetting.json', '/usr/config/minerd/settings/.cpbsetting.json',
    '/usr/config/minerd/settings/pools.json', '/usr/config/minerd/settings/.pools.json',
    '/usr/config/bfgminer/.bfgminer.json', '/usr/config/bfgminer/bfgminer.json',
    '/etc/init.d/S99paperclip', '/usr/config/paperclip/config.json',
    '/usr/config/paperclip/approved.json',
)
WEB_PATHS = tuple('/root/yottamc/' + name for name in (*WEB_ASSETS, 'paperclip.svg', 'legacy.html'))
BACKUP_PATHS = tuple(dict.fromkeys((*LEGACY_PATHS, *WEB_PATHS,
    '/usr/config/paperclip/controller',
    *('/usr/config/paperclip/' + name for name in SCRIPTS))))
LEGACY_LOGO_SHA256 = '7246a071c24a3602a302505c8e6774ef426e0ba336ff072ffe83431b3f9c9725'


def validate_backup(destination, identity, manifest):
    root = Path(destination).resolve()
    version = identity.get('manifest_version', 1)
    if type(version) is not int or version not in (1, 2) or not isinstance(manifest, dict) or not manifest:
        raise RuntimeError('Unsupported or empty recovery manifest')
    allowed = set(BACKUP_PATHS if version == 2 else LEGACY_PATHS)
    if not set(manifest) <= allowed or (version == 2 and set(manifest) != allowed):
        raise RuntimeError('Unexpected or incomplete recovery paths')
    for essential in ('/etc/inittab', '/root/yottamc/index.html'):
        meta = manifest.get(essential)
        if not isinstance(meta, dict) or meta.get('absent'):
            raise RuntimeError('Recovery backup is missing its original startup configuration or page')
    for remote, meta in manifest.items():
        if not isinstance(meta, dict):
            raise RuntimeError('Invalid recovery metadata')
        if meta.get('absent') is True:
            if set(meta) != {'absent'}:
                raise RuntimeError('Invalid absent-file recovery metadata')
            continue
        if set(meta) != {'mode', 'sha256'}:
            raise RuntimeError('Invalid present-file recovery metadata')
        mode, digest = meta.get('mode'), meta.get('sha256')
        if (type(mode) is not int or mode < 0 or mode > 0o777
                or not isinstance(digest, str) or not re.fullmatch('[0-9a-f]{64}', digest)):
            raise RuntimeError('Invalid recovery mode or checksum')
        local = root / remote.lstrip('/')
        if local.is_symlink() or not local.is_file() or not local.resolve().is_relative_to(root):
            raise RuntimeError('Missing or unsafe recovery file')
        if hashlib.sha256(local.read_bytes()).hexdigest() != digest:
            raise RuntimeError('Recovery backup checksum mismatch')
    return version
