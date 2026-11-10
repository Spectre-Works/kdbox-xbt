import base64
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import MagicMock, Mock, patch

from flasher.device import Device
from flasher.recovery import BACKUP_PATHS


class SnapshotTests(unittest.TestCase):
    def device(self):
        device = Device.__new__(Device)
        device.inspect = Mock(return_value={'firmware': '2.2.1'})
        device.client = MagicMock()
        device.client.get_transport().get_remote_server_key().asbytes.return_value = b'host-key'
        device.command = Mock(side_effect=lambda cmd: hashlib.sha256(b'private').hexdigest() + '  file'
                              if cmd.startswith('sha256sum ') else 'aa:bb:cc:dd:ee:ff\n')
        return device

    def test_snapshot_captures_all_managed_files_with_private_permissions(self):
        device = self.device()
        sftp = device.client.open_sftp.return_value.__enter__.return_value
        sftp.stat.return_value = SimpleNamespace(st_mode=0o100644)
        sftp.get.side_effect = lambda remote, local: Path(local).write_bytes(b'private')
        with tempfile.TemporaryDirectory() as folder:
            backup = Path(folder) / 'snapshot'
            device.backup(backup)
            manifest = json.loads((backup / 'manifest.json').read_text())
            identity = json.loads((backup / 'device.json').read_text())
            self.assertEqual(set(manifest), set(BACKUP_PATHS))
            self.assertEqual(identity['manifest_version'], 2)
            self.assertIn('/usr/config/paperclip/controller', manifest)
            self.assertIn('/root/yottamc/settings.js', manifest)
            if os.name != 'nt':
                self.assertEqual(backup.stat().st_mode & 0o777, 0o700)
                for path in backup.rglob('*'):
                    if path.is_file():
                        self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_incomplete_backup_never_reaches_restore_writes(self):
        device = self.device()
        device.write = Mock()
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / 'device.json').write_text(json.dumps({
                'firmware': '2.2.1', 'mac': 'aa:bb:cc:dd:ee:ff',
                'host_key': base64.b64encode(b'host-key').decode(), 'manifest_version': 2,
            }))
            (root / 'manifest.json').write_text('{}')
            with self.assertRaises(RuntimeError):
                device.restore(root)
            device.write.assert_not_called()
            self.assertEqual(device.command.call_count, 1)  # Read-only identity check only.

    def test_custom_logo_is_never_removed(self):
        device = self.device()
        with tempfile.TemporaryDirectory() as folder:
            logo = Path(folder) / 'root/yottamc/paperclip.svg'
            logo.parent.mkdir(parents=True)
            logo.write_bytes(b'custom owner artwork')
            device.retire_upstream_logo(folder)
            device.command.assert_not_called()

    def test_known_logo_removal_checks_current_remote_hash(self):
        device = self.device()
        with tempfile.TemporaryDirectory() as folder:
            logo = Path(folder) / 'root/yottamc/paperclip.svg'
            logo.parent.mkdir(parents=True)
            logo.write_bytes(b'upstream fixture')
            digest = hashlib.sha256(logo.read_bytes()).hexdigest()
            with patch('flasher.device.LEGACY_LOGO_SHA256', digest):
                device.retire_upstream_logo(folder)
            command = device.command.call_args.args[0]
            self.assertIn('sha256sum /root/yottamc/paperclip.svg', command)
            self.assertIn(digest, command)
            self.assertTrue(logo.exists())  # The recovery copy is never removed.


if __name__ == '__main__':
    unittest.main()
