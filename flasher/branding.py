"""Product identity. Runtime paths deliberately retain upstream compatibility."""
from pathlib import Path

PRODUCT = "KD BOX XBT Mod"
INSTALLER = "KD BOX XBT Installer"
PUBLISHER = "SPECTRE WORKS"
SOURCE_URL = "https://github.com/Spectre-Works/kdbox-xbt"

def backup_root():
    return Path.home() / "KDBoxXBT" / "backups"

def recovery_start_directory():
    for path in (backup_root(), Path.home() / "PaperclipFlasher" / "backups"):
        if path.is_dir():
            return path
    return Path.home()
