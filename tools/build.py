#!/usr/bin/env python3
"""Build the ARM controller and its license-bearing, hash-checked payload.

No device is contacted. Python dependencies are not copied into the product.
"""
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from flasher import __version__
from flasher.payload import payload_hashes, validate_payload


def run(args, *, root=ROOT, env=None):
    return subprocess.run(args, cwd=root, env=env, check=True, text=True,
                          stdout=subprocess.PIPE).stdout.strip()


def prepare_notices(root, goroot, go_version):
    root, goroot = Path(root), Path(goroot)
    if not (goroot / 'LICENSE').is_file():
        raise RuntimeError('Go toolchain LICENSE is missing; refusing an incomplete distribution.')
    names = {'license', 'license.txt', 'license.md', 'copying', 'copying.txt',
             'copyright', 'copyright.txt', 'notice', 'notice.txt', 'patents', 'authors'}
    candidates = [p for p in goroot.rglob('*')
                  if p.is_file() and p.name.lower() in names]
    # Deliberately over-include the toolchain's notices rather than guessing
    # which vendored notice applies to every linker/runtime implementation.
    chunks = [
        'KD BOX XBT Mod — open-source notices\n',
        (root / 'NOTICE').read_text(encoding='utf-8'),
        '\n=== Application MIT license ===\n',
        (root / 'LICENSE').read_text(encoding='utf-8'),
        '\n=== Go toolchain notices ===\n',
        f'Toolchain: {go_version}\nTarget: linux/arm/v7, CGO disabled.\n',
        'The following conservative notice inventory includes toolchain components\n'
        'that may not be linked into this particular controller. No Go toolchain\n'
        'executable is redistributed in the installer package.\n',
    ]
    inventory = []
    for path in sorted(candidates):
        data = path.read_bytes()
        relative = path.relative_to(goroot).as_posix()
        chunks.extend([f'\n=== Go: {relative} ===\n', data.decode('utf-8')])
        inventory.append({'path': relative, 'sha256': hashlib.sha256(data).hexdigest()})
    (root / 'controller/web/open-source.txt').write_text('\n'.join(chunks), encoding='utf-8')
    return inventory


def git_identity(root):
    try:
        commit = run(['git', 'rev-parse', 'HEAD'], root=root)
        # This one generated notice file is intentionally replaced by the build.
        dirty = bool(run(['git', 'status', '--porcelain', '--untracked-files=no',
                          '--', '.', ':(exclude)controller/web/open-source.txt'], root=root))
        return commit, dirty
    except (OSError, subprocess.CalledProcessError):
        return 'source-archive-without-git', True


def source_hashes(root):
    try:
        names = run(['git', 'ls-files', '-z'], root=root).split('\0')
    except (OSError, subprocess.CalledProcessError):
        return None
    return {name: hashlib.sha256((Path(root) / name).read_bytes()).hexdigest()
            for name in names if name and name != 'controller/web/open-source.txt'
            and (Path(root) / name).is_file()}


def main():
    env = dict(os.environ, GOOS='linux', GOARCH='arm', GOARM='7',
               CGO_ENABLED='0', GOWORK='off', GOFLAGS='')
    info = json.loads(run(['go', 'env', '-json', 'GOROOT', 'GOVERSION'], env=env))
    version = re.fullmatch(r'go(\d+)\.(\d+)\.(\d+)', info['GOVERSION'])
    if not version or tuple(map(int, version.groups())) < (1, 27, 3):
        raise RuntimeError('Use stable Go 1.27.3 or newer; the build will not silently use an older toolchain.')
    modules = run(['go', 'list', '-m', 'all'], env=env).splitlines()
    if modules != ['github.com/Spectre-Works/kdbox-xbt']:
        raise RuntimeError('New Go modules require a dependency-license review before packaging.')
    commit, dirty = git_identity(ROOT)
    notices = prepare_notices(ROOT, info['GOROOT'], info['GOVERSION'])
    (ROOT / 'build').mkdir(exist_ok=True)
    run(['go', 'build', '-trimpath', '-buildvcs=false',
         f'-ldflags=-s -w -X main.buildVersion={__version__}',
         '-o', 'build/paperclip-controller', './controller'], env=env)
    manifest = {
        'schema': 1, 'product': 'KD BOX XBT Mod', 'version': __version__,
        'target': 'linux/arm/v7', 'cgo': False, 'go_version': info['GOVERSION'],
        'source_commit': commit, 'source_dirty': dirty,
        'source_files': source_hashes(ROOT),
        'go_notice_inventory': notices, 'files': payload_hashes(ROOT),
    }
    (ROOT / 'build/payload.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    # This is a record of the test/build environment, NOT code included in the ZIP
    # and NOT a claim that metadata alone is a full dependency-license audit.
    inventory = sorted(({
        'name': d.metadata.get('Name', ''), 'version': d.version,
        'license_expression': d.metadata.get('License-Expression'),
        'license_metadata': d.metadata.get('License'),
    } for d in importlib.metadata.distributions()), key=lambda d: d['name'].lower())
    (ROOT / 'build/python-environment.json').write_text(json.dumps({
        'purpose': 'Build/test environment inventory; Python packages are not bundled.',
        'python': sys.version, 'distributions': inventory,
    }, indent=2) + '\n', encoding='utf-8')
    validate_payload(ROOT)
    print(f'Built and checked KD BOX XBT {__version__}; no miner was contacted.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as exc:
        print(f'Build stopped: {exc}', file=sys.stderr)
        sys.exit(1)
