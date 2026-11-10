"""One-time, hash-checked transfer of reviewed source edits. No miner access."""
import base64
import hashlib
import json
from pathlib import Path
import subprocess
import zlib

ROOT = Path(__file__).resolve().parents[1]
EXPECTED = '3f9e146f17e66a975c37aa3a3add3cf1eac98b56f1a178914cc28f056acf7166'

def main():
    parts = [''.join((ROOT / f'.maintenance/updates-{i}.txt').read_text().split()) for i in range(4)]
    # Correct three transcription errors in the transfer encoding. The complete
    # decoded source document and every resulting file must match reviewed hashes.
    parts[0] = parts[0].replace('JAJ3Clgs', 'JAJ3mcS6S3Clgs').replace('cwQSfXJJPL98', 'cwQSfXJyfPL98')
    parts[1] = parts[1].replace('JFurJFur7ro', 'JFur7ro')
    raw = zlib.decompress(base64.b64decode(''.join(parts), validate=True))
    if hashlib.sha256(raw).hexdigest() != EXPECTED:
        raise RuntimeError('Source transfer checksum mismatch')
    updates = json.loads(raw)
    pending = []
    names = set()
    for update in updates:
        name = update['path']
        path = ROOT / name
        if (name in names or Path(name).is_absolute() or '..' in Path(name).parts
                or name.startswith(('.git/', '.maintenance/', '.github/workflows/'))
                or path.is_symlink() or not path.resolve().is_relative_to(ROOT)):
            raise RuntimeError(f'Unexpected transfer path: {name}')
        names.add(name)
        old = path.read_bytes() if path.is_file() else None
        digest = hashlib.sha256(old).hexdigest() if old is not None else None
        if digest == update['after']:
            continue
        if digest != update['before']:
            raise RuntimeError(f'Concurrent source change; refusing overwrite: {name}')
        if update['after'] is None:
            new = None
        elif 'content' in update:
            new = update['content'].encode('utf-8')
        else:
            text = old.decode('utf-8')
            last = len(text)
            for start, stop, replacement in reversed(update['edits']):
                if not 0 <= start <= stop <= last:
                    raise RuntimeError('Invalid edit offsets')
                text = text[:start] + replacement + text[stop:]
                last = start
            new = text.encode('utf-8')
        if new is not None and hashlib.sha256(new).hexdigest() != update['after']:
            raise RuntimeError(f'Result checksum mismatch: {name}')
        pending.append((name, new, update.get('executable', False)))
    # Validate every old/new hash before making any source change.
    for name, new, executable in pending:
        path = ROOT / name
        if new is None:
            path.unlink()
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(new)
            path.chmod(0o755 if executable else 0o644)
    if pending:
        subprocess.run(['git', 'add', '--all', '--', *(n for n, _, _ in pending)], cwd=ROOT, check=True)
    print(f'Applied {len(pending)} verified source changes; no device was contacted.')

if __name__ == '__main__':
    main()
