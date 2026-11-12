import hashlib
from pathlib import Path
import unittest

from tools.release_audit import REVIEWED_FONTS, REVIEWED_IMAGES, audit_contents


class ReleaseAudit(unittest.TestCase):
    def test_rejects_vendor_files_keys_renamed_blobs_and_binary_deltas(self):
        cases = [('vendor.cpb', b'firmware'), ('rootfs.ext4', b'filesystem'),
                 ('minerd', b'executable'), ('customer-firmware/data.json', b'{}'),
                 ('notes.txt', b'\x7fELFbinary'), ('image.txt', b'+iusbinary'),
                 ('key.txt', b'-----BEGIN PRIVATE KEY-----\nfixture\n'),
                 ('change.patch', b'GIT binary patch\nliteral 12\n'),
                 ('../escape.md', b'text')]
        for name, data in cases:
            with self.subTest(name=name), self.assertRaises(RuntimeError):
                audit_contents({name: data})

    def test_only_the_verified_controller_is_allowed(self):
        controller = b'\x7fELFsynthetic-controller'
        sha = hashlib.sha256(controller).hexdigest()
        result = audit_contents({'README.md': b'Source', 'build/kdbox-xbt-controller': controller}, controller_sha256=sha)
        self.assertEqual(result['result'], 'passed')
        with self.assertRaises(RuntimeError):
            audit_contents({'elsewhere/controller': controller}, controller_sha256=sha)

    def test_only_the_reviewed_fonts_are_allowed(self):
        root = Path(__file__).resolve().parents[1]
        for name in REVIEWED_FONTS:
            data = (root / name).read_bytes()
            self.assertEqual(audit_contents({name: data})['result'], 'passed')
            for path, content in ((name, data + b'!'), ('elsewhere/font.woff2', data)):
                with self.assertRaises(RuntimeError):
                    audit_contents({path: content})

    def test_only_the_reviewed_brand_image_is_allowed(self):
        root = Path(__file__).resolve().parents[1]
        for name in REVIEWED_IMAGES:
            data = (root / name).read_bytes()
            self.assertEqual(audit_contents({name: data})['result'], 'passed')
            for path, content in ((name, data + b'!'), ('elsewhere/logo.png', data)):
                with self.assertRaises(RuntimeError):
                    audit_contents({path: content})


if __name__ == '__main__':
    unittest.main()
