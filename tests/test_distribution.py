import hashlib
import json
from pathlib import Path
import struct
import tempfile
import unittest
from xml.etree import ElementTree

from flasher import __version__
from flasher.payload import PAYLOAD_FILES, WEB_ASSETS, payload_hashes, validate_controller, validate_payload
from flasher.recovery import BACKUP_PATHS, LEGACY_PATHS, validate_backup
from tools.build import prepare_notices

ROOT = Path(__file__).resolve().parents[1]


def arm_header():
    data = bytearray(52)
    data[:7] = b'\x7fELF\x01\x01\x01'
    struct.pack_into('<HHI', data, 16, 2, 40, 1)
    struct.pack_into('<H', data, 40, 52)
    return bytes(data)


def payload_fixture(root):
    for name in PAYLOAD_FILES:
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b'fixture')
    (root / 'build/paperclip-controller').write_bytes(arm_header())
    (root / 'controller/web/open-source.txt').write_text(
        'Copyright (c) 2026 Paperclip Flasher contributors\nGo toolchain notices\n')
    manifest = {'schema': 1, 'version': __version__, 'target': 'linux/arm/v7',
                'cgo': False, 'files': payload_hashes(root)}
    (root / 'build/payload.json').write_text(json.dumps(manifest))
    return manifest


def backup_fixture(root, version=2):
    paths = BACKUP_PATHS if version == 2 else LEGACY_PATHS
    manifest = {p: {'absent': True} for p in paths}
    for remote in ('/etc/inittab', '/root/yottamc/index.html'):
        path = root / remote.lstrip('/')
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b'original')
        manifest[remote] = {'sha256': hashlib.sha256(b'original').hexdigest(), 'mode': 0o644}
    return {'manifest_version': version}, manifest


class PayloadValidation(unittest.TestCase):
    def test_arm_header_and_wrong_architectures(self):
        validate_controller(arm_header())
        for offset, replacement in ((4, b'\x02'), (5, b'\x02'), (18, b'\xb7\x00'), (40, b'\x40\x00')):
            data = bytearray(arm_header())
            data[offset:offset + len(replacement)] = replacement
            with self.assertRaises(RuntimeError):
                validate_controller(bytes(data))
        for data in (b'', b'\x7fELF', arm_header()[:51]):
            with self.assertRaises(RuntimeError):
                validate_controller(data)

    def test_complete_payload_and_tamper_detection(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            payload_fixture(root)
            self.assertEqual(validate_payload(root), arm_header())
            (root / 'controller/web/app.js').write_text('changed after build')
            with self.assertRaisesRegex(RuntimeError, 'mismatched'):
                validate_payload(root)

    def test_no_manifest_or_missing_license_fails(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaises(RuntimeError):
                validate_payload(root)
            m = payload_fixture(root)
            (root / 'controller/web/open-source.txt').write_text('notices removed')
            m['files'] = payload_hashes(root)
            (root / 'build/payload.json').write_text(json.dumps(m))
            with self.assertRaises(RuntimeError):
                validate_payload(root)

    def test_extra_file_in_manifest_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            m = payload_fixture(root)
            m['files']['controller/web/private-password.txt'] = '0' * 64
            (root / 'build/payload.json').write_text(json.dumps(m))
            with self.assertRaises(RuntimeError):
                validate_payload(root)

    def test_payload_version_mismatch_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            m = payload_fixture(root)
            m['version'] = 'another build'
            (root / 'build/payload.json').write_text(json.dumps(m))
            with self.assertRaises(RuntimeError):
                validate_payload(root)

    def test_payload_symlink_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder, tempfile.TemporaryDirectory() as outside:
            root = Path(folder)
            payload_fixture(root)
            asset = root / 'controller/web/app.js'
            asset.unlink()
            other = Path(outside) / 'data'
            other.write_bytes(b'fixture')
            try:
                asset.symlink_to(other)
            except OSError:
                self.skipTest('Creating symlinks requires privileges on this platform')
            with self.assertRaises(RuntimeError):
                validate_payload(root)


class RecoveryValidation(unittest.TestCase):
    def test_new_and_legacy_snapshots(self):
        for version in (1, 2):
            with tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                identity, manifest = backup_fixture(root, version)
                self.assertEqual(validate_backup(root, identity, manifest), version)
                if version == 1:
                    self.assertEqual(validate_backup(root, {}, manifest), 1)

    def test_missing_new_snapshot_entry_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root)
            del manifest['/usr/config/paperclip/controller']
            with self.assertRaises(RuntimeError):
                validate_backup(root, identity, manifest)

    def test_unsafe_empty_and_malformed_snapshots(self):
        for mutation in ('empty', 'path', 'essential', 'checksum', 'mode', 'absent-string', 'schema'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                identity, manifest = backup_fixture(root)
                if mutation == 'empty': manifest = {}
                if mutation == 'path': manifest['/etc/shadow'] = {'absent': True}
                if mutation == 'essential': manifest['/etc/inittab'] = {'absent': True}
                if mutation == 'checksum': manifest['/etc/inittab']['sha256'] = '0' * 64
                if mutation == 'mode': manifest['/etc/inittab']['mode'] = 0o4777
                if mutation == 'absent-string': manifest['/etc/inittab']['absent'] = 'false'
                if mutation == 'schema': identity['manifest_version'] = True
                with self.assertRaises(RuntimeError):
                    validate_backup(root, identity, manifest)

    def test_recovery_symlink_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder, tempfile.TemporaryDirectory() as outside:
            root = Path(folder)
            identity, manifest = backup_fixture(root)
            path = root / 'etc/inittab'
            path.unlink()
            other = Path(outside) / 'data'
            other.write_bytes(b'original')
            try:
                path.symlink_to(other)
            except OSError:
                self.skipTest('Creating symlinks requires privileges on this platform')
            with self.assertRaises(RuntimeError):
                validate_backup(root, identity, manifest)


class BrandingAndNotices(unittest.TestCase):
    def test_frontend_and_product_identity(self):
        for name in ('index.html', 'app.js', 'settings.js', 'style.css'):
            text = (ROOT / 'controller/web' / name).read_text(encoding='utf-8').lower()
            self.assertNotIn('paperclip', text, name)
            self.assertNotIn('paperclippool.xyz', text, name)
        self.assertIn('KD BOX Control', (ROOT / 'controller/web/index.html').read_text(encoding='utf-8'))
        self.assertFalse((ROOT / 'controller/web/paperclip.svg').exists())
        self.assertIn('Copyright (c) 2026 Paperclip Flasher contributors', (ROOT / 'LICENSE').read_text(encoding='utf-8'))
        self.assertIn('Permission is hereby granted', (ROOT / 'LICENSE').read_text(encoding='utf-8'))

    def test_static_assets_are_flat_and_complete(self):
        actual = {p.name for p in (ROOT / 'controller/web').iterdir()}
        self.assertEqual(actual, set(WEB_ASSETS))
        for path in (ROOT / 'controller/web').iterdir():
            self.assertTrue(path.is_file())
        ElementTree.parse(ROOT / 'controller/web/favicon.svg')
        self.assertIn((ROOT / 'LICENSE').read_text(encoding='utf-8').strip(),
                      (ROOT / 'controller/web/open-source.txt').read_text(encoding='utf-8'))

    def test_go_notice_generation_retains_actual_license_files(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder) / 'project'
            goroot = Path(folder) / 'go'
            (root / 'controller/web').mkdir(parents=True)
            (root / 'LICENSE').write_text('original MIT terms')
            (root / 'NOTICE').write_text('upstream attribution')
            (goroot / 'src/vendor/example').mkdir(parents=True)
            (goroot / 'LICENSE').write_text('Go terms')
            (goroot / 'PATENTS').write_text('Go patent notice')
            (goroot / 'src/vendor/example/LICENSE').write_text('vendored terms')
            inventory = prepare_notices(root, goroot, 'go1.27.3')
            text = (root / 'controller/web/open-source.txt').read_text(encoding='utf-8')
            for expected in ('original MIT terms', 'upstream attribution', 'Go terms',
                             'Go patent notice', 'vendored terms', 'go1.27.3'):
                self.assertIn(expected, text)
            self.assertEqual(len(inventory), 3)

    def test_build_requires_go_license(self):
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaises(RuntimeError):
                prepare_notices(Path(folder), Path(folder), 'go1.27.3')

    def test_default_workflow_does_not_freeze_python(self):
        workflow = (ROOT / '.github/workflows/test.yml').read_text(encoding='utf-8')
        self.assertNotIn('PyInstaller', workflow)
        self.assertNotIn('pyinstaller', workflow)
        self.assertIn('python tools/build.py', workflow)
        self.assertIn('python tools/package.py', workflow)


if __name__ == '__main__':
    unittest.main()
