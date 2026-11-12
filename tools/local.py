#!/usr/bin/env python3
"""Local test/build entry points. No device is contacted or artifact uploaded."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from flasher import __version__
from tools import CONTROLLER_BUILD_TAGS
from tools.release_audit import audit_source_zip


def run(args, *, env=None):
    print('+ ' + ' '.join(map(str, args)), flush=True)
    subprocess.run(args, cwd=ROOT, env=env, check=True)


def require(*commands):
    for command in commands:
        if shutil.which(command) is None:
            raise RuntimeError(f'{command} is required on PATH; see README.md for setup.')


def build():
    require('git', 'go')
    run([sys.executable, 'tools/build.py'])


def test():
    require('git', 'go', 'node')
    build()
    # Test the desktop host, even if the shell was configured to cross-compile.
    env = dict(os.environ)
    for name in ('GOOS', 'GOARCH', 'GOARM', 'GOARM64', 'GOAMD64', 'GO386'):
        env.pop(name, None)
    env.update(GOWORK='off', GOFLAGS='')
    run(['go', 'test', f'-tags={CONTROLLER_BUILD_TAGS}', './...'], env=env)
    run(['go', 'vet', f'-tags={CONTROLLER_BUILD_TAGS}', './...'], env=env)
    if platform.system() == 'Linux':
        # The Go race detector requires CGO and a host C compiler.
        run(['go', 'test', f'-tags={CONTROLLER_BUILD_TAGS}', '-race', './...'], env=dict(env, CGO_ENABLED='1'))
    else:
        print('Linux race check skipped on this host.', flush=True)
    run([sys.executable, '-m', 'unittest', 'discover', '-s', 'tests', '-v'])
    for asset in ('app.js', 'settings.js'):
        run(['node', '--check', f'controller/web/{asset}'])

    run(['node', '--check', 'flasher/web/wizard.js'])

def package():
    run([sys.executable, 'tools/package.py'])


def source():
    require('git')
    output = ROOT / 'dist' / 'kdbox-xbt-source.zip'
    output.parent.mkdir(exist_ok=True)
    run(['git', 'archive', '--format=zip', '--prefix=kdbox-xbt/',
         '-o', str(output), 'HEAD'])
    try:
        audit_source_zip(output)
    except BaseException:
        output.unlink(missing_ok=True)
        raise
    checksums = output.parent / 'SHA256SUMS.txt'
    entries = []
    product = output.parent / f'KDBoxXBT-{__version__}.zip'
    for artifact in (product, output):
        if artifact.is_file():
            entries.append(f'{hashlib.sha256(artifact.read_bytes()).hexdigest()}  {artifact.name}\n')
    checksums.write_text(''.join(entries), encoding='utf-8')
    print(f'Source archive of committed HEAD only: {output}', flush=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('build', 'test', 'package', 'source', 'all'),
                        help='all runs build + tests + product package + source archive')
    args = parser.parse_args(argv)
    if sys.version_info < (3, 12):
        parser.error('Python 3.12 or newer is required.')
    try:
        if args.command == 'build':
            build()
        elif args.command == 'test':
            test()
        elif args.command == 'package':
            build()
            package()
        elif args.command == 'source':
            source()
        else:
            test()
            package()
            source()
    except (OSError, RuntimeError, subprocess.CalledProcessError) as exc:
        print(f'Local {args.command} stopped: {exc}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
