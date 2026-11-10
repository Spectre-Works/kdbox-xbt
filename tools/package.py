#!/usr/bin/env python3
"""Package tracked source + controller; never vendor a local Python environment."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from flasher import __version__
from flasher.payload import PAYLOAD_FILES, validate_payload
from tools.build import source_hashes

FORBIDDEN_PARTS = {'.git', '.venv', 'venv', 'site-packages', 'backups', '__pycache__', 'dist'}


def package(root):
    root = Path(root).resolve()
    validate_payload(root)
    manifest = json.loads((root / 'build/payload.json').read_text(encoding='utf-8'))
    if manifest.get('source_dirty') is not False or manifest.get('source_files') != source_hashes(root):
        raise RuntimeError('Commit source changes and rebuild before publishing; source must match the controller build.')
    tracked = subprocess.run(['git', 'ls-files', '-z'], cwd=root, check=True,
                             stdout=subprocess.PIPE).stdout.decode('utf-8').split('\0')
    paths = {p for p in tracked if p}
    paths.update(PAYLOAD_FILES)
    paths.update(('build/payload.json', 'build/python-environment.json'))
    contents = {}
    for relative in sorted(paths):
        path = root / relative
        if (Path(relative).is_absolute() or '..' in Path(relative).parts
                or FORBIDDEN_PARTS.intersection(Path(relative).parts)
                or path.is_symlink() or not path.resolve().is_relative_to(root)):
            raise RuntimeError(f'Unsafe distribution path: {relative}')
        if not path.is_file():
            raise RuntimeError(f'Missing distribution source file: {relative}')
        contents[relative] = path.read_bytes()
    for required in ('LICENSE', 'NOTICE', 'docs/licensing.md', 'install.sh', 'install.cmd'):
        if required not in contents:
            raise RuntimeError(f'Missing distribution notice or entry point: {required}')
    contents['FILES.sha256'] = ''.join(
        f'{hashlib.sha256(data).hexdigest()}  {name}\n'
        for name, data in sorted(contents.items())
    ).encode('utf-8')
    out = root / 'dist'
    out.mkdir(exist_ok=True)
    archive = out / f'KDBoxXBT-{__version__}-preview.zip'
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as z:
        for name, data in sorted(contents.items()):
            item = zipfile.ZipInfo('KDBoxXBT/' + name, date_time=(1980, 1, 1, 0, 0, 0))
            item.create_system = 3
            item.external_attr = (0o100755 if name == 'install.sh' else 0o100644) << 16
            item.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(item, data)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    (out / 'SHA256SUMS.txt').write_text(f'{digest}  {archive.name}\n', encoding='utf-8')
    return archive


if __name__ == '__main__':
    try:
        print(package(ROOT))
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as exc:
        print(f'Packaging stopped: {exc}', file=sys.stderr)
        sys.exit(1)
