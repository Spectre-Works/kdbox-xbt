"""Versioned, allowlisted recovery snapshots; never accept arbitrary write paths."""
import hashlib
from pathlib import Path
import re
from .payload import WEB_ASSETS, WEB_FONTS, SCRIPTS, RECOVERY_PATH

BACKUP_SCHEMA = 7
ALGORITHM_PATH = '/usr/config/kdbox-xbt/algorithm.json'
CONFIG_PATHS = (
    '/etc/inittab', '/root/yottamc/index.html', '/root/settings/algosetting.json',
    '/usr/config/minerd/settings/algosetting.json', '/usr/config/minerd/settings/.algosetting.json',
    '/usr/config/minerd/settings/cpbsetting.json', '/usr/config/minerd/settings/.cpbsetting.json',
    '/usr/config/minerd/settings/pools.json', '/usr/config/minerd/settings/.pools.json',
    '/usr/config/bfgminer/.bfgminer.json', '/usr/config/bfgminer/bfgminer.json',
    '/etc/init.d/S99kdbox-xbt', '/usr/config/kdbox-xbt/config.json',
    '/usr/config/kdbox-xbt/approved.json', ALGORITHM_PATH,
    RECOVERY_PATH,
)
LEGACY_FAVICON = '/root/yottamc/favicon.svg'
WEB_PATHS = (*('/root/yottamc/' + name for name in (*WEB_ASSETS, 'legacy.html')), LEGACY_FAVICON)
BACKUP_PATHS = tuple(dict.fromkeys((*CONFIG_PATHS, *WEB_PATHS,
    '/usr/config/kdbox-xbt/controller',
    *('/usr/config/kdbox-xbt/' + name for name in SCRIPTS))))
ADDED_BACKUP_PATHS = {ALGORITHM_PATH, *('/root/yottamc/' + name for name in WEB_FONTS)}
BRAND_BACKUP_PATHS = {'/root/yottamc/spectre-logo.png'}
FAVICON_BACKUP_PATHS = {'/root/yottamc/favicon-32.png', '/root/yottamc/favicon-48.png'}


def validate_backup(destination, identity, manifest):
    root = Path(destination).resolve()
    version = identity.get('manifest_version')
    if type(version) is not int or version not in (3, 4, 5, 6, BACKUP_SCHEMA) or not isinstance(manifest, dict) or not manifest:
        raise RuntimeError('Unsupported or empty recovery manifest')
    allowed = set(BACKUP_PATHS)
    if version < 7:
        allowed -= {RECOVERY_PATH}
    if version < 6:
        allowed -= FAVICON_BACKUP_PATHS
    if version < 5:
        allowed -= BRAND_BACKUP_PATHS
    if version == 3:
        allowed -= ADDED_BACKUP_PATHS
    if set(manifest) != allowed:
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
