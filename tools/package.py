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
from flasher.payload import FONT_LICENSE, PAYLOAD_FILES, validate_payload
from tools.build import source_hashes
from tools.release_audit import audit_contents

FORBIDDEN_PARTS = {'.git', '.venv', 'venv', 'site-packages', 'backups', '__pycache__', 'dist', '.installer'}


def package(root):
    root = Path(root).resolve()
    validate_payload(root)
    build_info = subprocess.run(['go', 'version', '-m', str(root / 'build/kdbox-xbt-controller')],
                                check=True, text=True, stdout=subprocess.PIPE).stdout
    if '\tpath\tgithub.com/Spectre-Works/kdbox-xbt/controller' not in build_info:
        raise RuntimeError('Public controller is not identified as this project by Go build metadata.')
    manifest = json.loads((root / 'build/payload.json').read_text(encoding='utf-8'))
    if (manifest.get('source_dirty') is not False or manifest.get('source_files') != source_hashes(root)
            or manifest.get('source_commit') != subprocess.run(['git', 'rev-parse', 'HEAD'], cwd=root, check=True, text=True, stdout=subprocess.PIPE).stdout.strip()):
        raise RuntimeError('Commit source changes and rebuild before publishing; source must match the controller build.')
    # Go omits linker flags from build metadata when -trimpath is enabled.
    # Check the embedded revision itself; the manifest above separately binds
    # this payload's checksum to the complete, clean source inventory.
    if manifest['source_commit'].encode('ascii') not in (root / 'build/kdbox-xbt-controller').read_bytes():
        raise RuntimeError('The compiled controller revision does not match the clean source commit.')
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
    for required in ('LICENSE', 'NOTICE', 'BRANDING.md', 'licenses/upstream-MIT.txt', 'docs/licensing.md', 'install.sh', 'install.cmd',
                     'install.ps1', 'install.command', 'flasher/wizard.py',
                     'flasher/discovery.py', 'flasher/defaults.py',
                     'flasher/web/index.html', 'flasher/web/wizard.js', 'flasher/web/wizard.css'):
        if required not in contents:
            raise RuntimeError(f'Missing distribution notice or entry point: {required}')
    controller_sha = manifest['files']['build/kdbox-xbt-controller']
    audit = audit_contents(contents, controller_sha256=controller_sha)
    contents['PUBLIC-RELEASE-AUDIT.json'] = (json.dumps(audit, indent=2) + '\n').encode()
    contents['DISTRIBUTION-COMPONENTS.json'] = (json.dumps({
        'schema': 1, 'components': [
            {'name': 'SPECTRE WORKS installer, controller and dashboard contributions', 'license': 'Apache-2.0',
             'notices': ['LICENSE', 'NOTICE']},
            {'name': 'Retained upstream application code', 'license': 'MIT',
             'notices': ['licenses/upstream-MIT.txt', 'NOTICE']},
            {'name': 'SPECTRE WORKS company emblem and product identity', 'license': 'LicenseRef-SPECTRE-WORKS-Brand-Policy',
             'notices': ['BRANDING.md'], 'scope': 'Separate brand-asset permission; not a restriction added to the software license.'},
            {'name': 'Go standard library/runtime in the controller', 'version': manifest['go_version'],
             'notices': ['controller/web/open-source.txt']},
            {'name': 'Chakra Petch dashboard fonts', 'license': 'OFL-1.1', 'notices': [FONT_LICENSE]},
        ],
        'not_bundled': ['Goldshell firmware/binaries/keys', 'uv runtime manager', 'Python', 'Paramiko'],
    }, indent=2) + '\n').encode()
    contents['FILES.sha256'] = ''.join(
        f'{hashlib.sha256(data).hexdigest()}  {name}\n'
        for name, data in sorted(contents.items())
    ).encode('utf-8')
    out = root / 'dist'
    out.mkdir(exist_ok=True)
    archive = out / f'KDBoxXBT-{__version__}.zip'
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as z:
        for name, data in sorted(contents.items()):
            item = zipfile.ZipInfo('KDBoxXBT/' + name, date_time=(1980, 1, 1, 0, 0, 0))
            item.create_system = 3
            item.external_attr = (0o100755 if name in ('install.sh', 'install.command') else 0o100644) << 16
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
