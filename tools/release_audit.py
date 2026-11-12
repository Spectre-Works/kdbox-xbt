"""Fail closed on vendor firmware, unknown binaries, keys and private workspaces."""
import hashlib
from pathlib import PurePosixPath
import re
import zipfile

FORBIDDEN_SUFFIXES = {'.cpb', '.ius', '.img', '.ext4', '.squashfs', '.bin', '.so', '.dll',
                      '.exe', '.a', '.key', '.pem', '.crt', '.cer', '.der', '.sig', '.cpbdelta'}
FORBIDDEN_PARTS = {'.git', '.venv', 'venv', 'site-packages', 'backups', '__pycache__',
                   '.installer', 'firmware-workspaces', 'customer-firmware', '.local-firmware', 'dist'}
VENDOR_NAMES = {'minerd', 'xminerd', 'intminer', 'kdaminer', 'upgrade', 'firmware.ius', 'rootfs',
                'config.json', 'device.json', 'approved.json', 'algorithm.json', 'pending.json', 'halted.json',
                'authorized_keys', 'sshd_config'}
SOURCE_SUFFIXES = {'.py', '.go', '.md', '.json', '.html', '.css', '.js', '.svg', '.yml',
                   '.yaml', '.sh', '.command', '.ps1', '.cmd', '.txt', '.mod', '.sum', '.patch', '.diff'}
REVIEWED_FONTS = {
    'controller/web/ChakraPetch-Regular.woff2': 'd7a6ac30cad36f9889c40b6ac13ea7627f2248bc8855e039cc582996d7c446c2',
    'controller/web/ChakraPetch-Bold.woff2': '037a2b8323f00b31fec4425c53154259bf5d13248348092bc182112c60cfa852',
}
REVIEWED_IMAGES = {
    'controller/web/spectre-logo.png': 'bd9c80c2f80b0feefaba12392b18b7436ae29c44c2d70117934348e77c281c2b',
    'controller/web/favicon-32.png': 'e49d3e3cab434cbdf2f0a4300d2e66d647bfd26178f717859d67c1dc3c7c466d',
    'controller/web/favicon-48.png': '4e4ebb7b8554dc79806b94091c800f884b81db36e5b2a763b725064d14f91056',
    'flasher/web/spectre-wordmark.png': 'a28984d912c29e8bef982caadb0d28c9f4dc5f6f3009f238eaeb4f51fe0b2db1',
}


def audit_contents(contents, *, controller_sha256=None):
    checked = []
    for name, data in sorted(contents.items()):
        path = PurePosixPath(name)
        if (path.is_absolute() or '..' in path.parts or '\\' in name or
                FORBIDDEN_PARTS.intersection(path.parts) or path.suffix.lower() in FORBIDDEN_SUFFIXES or
                path.name.lower() in VENDOR_NAMES):
            raise RuntimeError('Public release audit rejected forbidden/private file: ' + name)
        sha = hashlib.sha256(data).hexdigest()
        if name == 'build/kdbox-xbt-controller' and controller_sha256 is not None:
            if sha != controller_sha256 or not data.startswith(b'\x7fELF'):
                raise RuntimeError('Public controller does not match the verified application build.')
        elif name in REVIEWED_FONTS:
            if sha != REVIEWED_FONTS[name] or not data.startswith(b'wOF2'):
                raise RuntimeError('Dashboard font does not match the reviewed Chakra Petch asset.')
        elif name in REVIEWED_IMAGES:
            if sha != REVIEWED_IMAGES[name] or not data.startswith(b'\x89PNG\r\n\x1a\n'):
                raise RuntimeError('Dashboard image does not match the reviewed SPECTRE WORKS asset.')
        else:
            if (data.startswith((b'\x7fELF', b'MZ', b'+ius', b'\x1f\x8b', b'PK\x03\x04')) or
                    (data.startswith(b'firmware-') and b'.tgz' in data[:128])):
                raise RuntimeError('Public release audit rejected an unapproved binary/archive: ' + name)
            if path.suffix.lower() not in SOURCE_SUFFIXES and path.name not in {
                    'LICENSE', 'NOTICE', '.gitignore', '.gitattributes'}:
                raise RuntimeError('Public release file type requires review: ' + name)
            try:
                text = data.decode('utf-8')
            except UnicodeDecodeError as exc:
                raise RuntimeError('Public release audit rejected non-text source: ' + name) from exc
            if '\0' in text or re.search(r'^-----BEGIN [A-Z0-9 ]+(?:KEY|CERTIFICATE|SIGNATURE)-----\s*$', text, re.M):
                raise RuntimeError('Public release audit rejected key/certificate/binary material: ' + name)
            if path.suffix in ('.patch', '.diff') and ('GIT binary patch' in text or 'Binary files ' in text):
                raise RuntimeError('Binary delta requires separate content/rights review: ' + name)
        checked.append({'path': name, 'sha256': sha})
    return {'schema': 1, 'result': 'passed', 'checked': checked,
            'scope': 'Known application controller, reviewed SPECTRE WORKS image, Chakra Petch fonts, and text/source.'}


def audit_source_zip(path, prefix='kdbox-xbt/'):
    with zipfile.ZipFile(path) as archive:
        contents = {}
        for entry in archive.infolist():
            if entry.is_dir():
                continue
            if not entry.filename.startswith(prefix) or entry.filename in contents:
                raise RuntimeError('Unexpected source archive layout.')
            relative = entry.filename[len(prefix):]
            if relative in contents:
                raise RuntimeError('Duplicate source archive path.')
            contents[relative] = archive.read(entry)
    return audit_contents(contents)
