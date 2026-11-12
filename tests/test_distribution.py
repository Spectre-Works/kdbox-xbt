import hashlib
import json
from pathlib import Path
import struct
import tempfile
import unittest

from flasher import __version__
from flasher.payload import FONT_LICENSE, LEGAL_FILES, PAYLOAD_FILES, WEB_ASSETS, payload_hashes, validate_controller, validate_payload
from flasher.recovery import ADDED_BACKUP_PATHS, BRAND_BACKUP_PATHS, FAVICON_BACKUP_PATHS, BACKUP_PATHS, BACKUP_SCHEMA, validate_backup
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
    (root / 'build/kdbox-xbt-controller').write_bytes(arm_header())
    for name in LEGAL_FILES:
        (root / name).write_bytes((ROOT / name).read_bytes())
    (root / 'NOTICE').write_text('Fixture attribution')
    (root / FONT_LICENSE).write_bytes((ROOT / FONT_LICENSE).read_bytes())
    (root / 'controller/web/open-source.txt').write_text(
        '\n'.join((root / name).read_text() for name in LEGAL_FILES) + '\nGo toolchain notices\n')
    manifest = {'schema': 1, 'version': __version__, 'target': 'linux/arm/v7',
                'cgo': False, 'files': payload_hashes(root)}
    (root / 'build/payload.json').write_text(json.dumps(manifest))
    return manifest


def backup_fixture(root, version=BACKUP_SCHEMA):
    manifest = {p: {'absent': True} for p in BACKUP_PATHS}
    if version < 6:
        for name in FAVICON_BACKUP_PATHS:
            del manifest[name]
    if version < 5:
        for name in BRAND_BACKUP_PATHS:
            del manifest[name]
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

    def test_untracked_embedded_file_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            payload_fixture(root)
            (root / 'controller/web/unknown.bin').write_bytes(b'unknown embedded content')
            with self.assertRaisesRegex(RuntimeError, 'mismatched'):
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
    def test_current_snapshot_and_unsupported_schemas(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root)
            self.assertEqual(validate_backup(root, identity, manifest), BACKUP_SCHEMA)
            for version in (None, 1, 2, 7, True):
                with self.subTest(version=version), self.assertRaises(RuntimeError):
                    validate_backup(root, {'manifest_version': version}, manifest)

    def test_original_schema_three_backups_remain_restorable(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root, version=3)
            for name in ADDED_BACKUP_PATHS:
                del manifest[name]
            self.assertEqual(validate_backup(root, identity, manifest), 3)

    def test_schema_four_backups_without_brand_asset_remain_restorable(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root, version=4)
            self.assertEqual(validate_backup(root, identity, manifest), 4)
            identity['manifest_version'] = BACKUP_SCHEMA
            with self.assertRaisesRegex(RuntimeError, 'incomplete recovery paths'):
                validate_backup(root, identity, manifest)

    def test_schema_five_backups_keep_their_original_asset_set(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);identity,manifest=backup_fixture(root,version=5)
            self.assertEqual(validate_backup(root,identity,manifest),5)

    def test_missing_new_snapshot_entry_is_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root)
            del manifest['/usr/config/kdbox-xbt/controller']
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
            self.assertTrue(text.strip(), name)
        self.assertIn('KD BOX Control', (ROOT / 'controller/web/index.html').read_text(encoding='utf-8'))
        self.assertIn('Apache License', (ROOT / 'LICENSE').read_text(encoding='utf-8'))
        self.assertIn('Permission is hereby granted', (ROOT / 'licenses/upstream-MIT.txt').read_text(encoding='utf-8'))

    def test_static_assets_are_flat_and_complete(self):
        actual = {p.name for p in (ROOT / 'controller/web').iterdir()}
        self.assertEqual(actual, set(WEB_ASSETS))
        for path in (ROOT / 'controller/web').iterdir():
            self.assertTrue(path.is_file())
        for size in (32, 48):
            data=(ROOT / f'controller/web/favicon-{size}.png').read_bytes()
            self.assertTrue(data.startswith(b'\x89PNG\r\n\x1a\n'))
            self.assertEqual(struct.unpack('>II',data[16:24]),(size,size))
        self.assertIn((ROOT / 'LICENSE').read_text(encoding='utf-8').strip(),
                      (ROOT / 'controller/web/open-source.txt').read_text(encoding='utf-8'))

    def test_go_notice_generation_retains_actual_license_files(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder) / 'project'
            goroot = Path(folder) / 'go'
            (root / 'controller/web').mkdir(parents=True)
            (root / 'LICENSE').write_text('standard Apache terms')
            (root / 'BRANDING.md').write_text('separate branding policy')
            (root / 'NOTICE').write_text('upstream attribution')
            (root / FONT_LICENSE).parent.mkdir()
            (root / FONT_LICENSE).write_text('Chakra Petch OFL terms')
            (root / 'licenses/upstream-MIT.txt').write_text('original MIT terms')
            (goroot / 'src/vendor/example').mkdir(parents=True)
            (goroot / 'LICENSE').write_text('Go terms')
            (goroot / 'PATENTS').write_text('Go patent notice')
            (goroot / 'src/vendor/example/LICENSE').write_text('vendored terms')
            inventory = prepare_notices(root, goroot, 'go1.27.3')
            text = (root / 'controller/web/open-source.txt').read_text(encoding='utf-8')
            for expected in ('standard Apache terms', 'original MIT terms', 'separate branding policy', 'upstream attribution', 'Go terms',
                             'Go patent notice', 'vendored terms', 'Chakra Petch OFL terms', 'go1.27.3'):
                self.assertIn(expected, text)
            self.assertEqual(len(inventory), 3)

    def test_build_requires_go_license(self):
        with tempfile.TemporaryDirectory() as folder:
            with self.assertRaises(RuntimeError):
                prepare_notices(Path(folder), Path(folder), 'go1.27.3')


if __name__ == '__main__':
    unittest.main()
