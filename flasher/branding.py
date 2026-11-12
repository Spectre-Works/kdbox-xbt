"""SPECTRE WORKS product identity and private backup location."""
from pathlib import Path
import os

PRODUCT = "KD BOX XBT Mod"
INSTALLER = "KD BOX XBT Installer"
PUBLISHER = "SPECTRE WORKS"
SOURCE_URL = "https://github.com/Spectre-Works/kdbox-xbt"

def backup_root():
    custom = os.environ.get('KDBOX_XBT_BACKUP_DIR')
    if custom:
        path = Path(custom).expanduser()
        if not path.is_absolute():
            raise ValueError('KDBOX_XBT_BACKUP_DIR must be an absolute folder path.')
        return path
    return Path.home() / "KDBoxXBT" / "backups"

def recovery_start_directory():
    if backup_root().is_dir():
        return backup_root()
    return Path.home()
