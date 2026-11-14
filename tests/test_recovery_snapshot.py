import base64
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import MagicMock, Mock, patch

from flasher.device import Device, PROFILE
from flasher import __version__
from flasher.recovery import BACKUP_PATHS, BACKUP_SCHEMA
from test_distribution import arm_header, backup_fixture, payload_fixture


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
            self.assertEqual(identity['manifest_version'], BACKUP_SCHEMA)
            self.assertIn('/usr/config/kdbox-xbt/controller', manifest)
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
                'host_key': base64.b64encode(b'host-key').decode(), 'manifest_version': BACKUP_SCHEMA,
            }))
            (root / 'manifest.json').write_text('{}')
            with self.assertRaises(RuntimeError):
                device.restore(root)
            device.write.assert_not_called()
            self.assertEqual(device.command.call_count, 1)  # Read-only identity check only.

    @unittest.skipIf(os.name == 'nt', 'POSIX backup directory permissions')
    def test_backup_stops_when_private_permissions_cannot_be_set(self):
        device = self.device()
        with tempfile.TemporaryDirectory() as folder:
            backup = Path(folder) / 'snapshot'
            chmod = Path.chmod
            with patch.object(Path, 'chmod', lambda path, mode: chmod(path, mode | 0o070)):
                with self.assertRaisesRegex(RuntimeError, 'did not enforce private permissions'):
                    device.backup(backup)
            self.assertFalse(backup.exists())
        device.client.open_sftp.assert_not_called()

    @unittest.skipIf(os.name == 'nt', 'POSIX backup permissions')
    def test_permissions_widened_during_backup_block_completion(self):
        device = self.device()
        sftp = device.client.open_sftp.return_value.__enter__.return_value
        sftp.stat.return_value = SimpleNamespace(st_mode=0o100644)
        sftp.get.side_effect = lambda remote, local: Path(local).write_bytes(b'private')
        with tempfile.TemporaryDirectory() as folder:
            backup = Path(folder) / 'snapshot'
            chmod = Path.chmod
            def widen(path, mode):
                chmod(path, mode)
                if path.name == 'device.json':
                    chmod(backup, 0o770)
            with patch.object(Path, 'chmod', widen):
                with self.assertRaisesRegex(RuntimeError, 'permissions are not private'):
                    device.backup(backup)

    def test_backup_does_not_publish_manifest_with_missing_stock_files(self):
        device = self.device()
        sftp = device.client.open_sftp.return_value.__enter__.return_value
        sftp.stat.side_effect = FileNotFoundError
        with tempfile.TemporaryDirectory() as folder:
            backup = Path(folder) / 'snapshot'
            with self.assertRaisesRegex(RuntimeError, 'missing its original startup configuration or page'):
                device.backup(backup)
            self.assertFalse((backup / 'manifest.json').exists())

    def test_stock_restore_disables_fork_hook_and_restores_boot_last(self):
        device = self.device()
        device.write = Mock()
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            identity, manifest = backup_fixture(root)
            identity.update(firmware='2.2.1', mac='aa:bb:cc:dd:ee:ff',
                            host_key=base64.b64encode(b'host-key').decode())
            (root / 'device.json').write_text(json.dumps(identity))
            (root / 'manifest.json').write_text(json.dumps(manifest))
            device.restore(root)
        self.assertEqual(device.write.call_args_list[-1].args[0], '/etc/inittab')
        commands = [call.args[0] for call in device.command.call_args_list]
        self.assertTrue(any('mv /etc/init.d/S99kdbox-xbt /usr/config/kdbox-xbt/' in cmd
                            for cmd in commands))
        self.assertEqual(commands[-1], 'sync; reboot')

    def test_install_wires_new_controller_scripts_and_startup_namespace(self):
        device = self.device()
        device.host = '192.168.1.2'
        device.inspect.return_value = {'firmware': '2.2.1'}
        device.require_space = Mock()
        device.write = Mock()
        device.api = Mock(side_effect=[{'JWT Token': 'owner'}, {'algo_select':0}, [{'url':'old-pool'}], {}, [], {}, {'algo_select':1}])
        device._api_once = Mock(return_value={
            'device': None, 'approved': {}, 'stale': True, 'halted': False, 'algorithm':1,
            'startup': 'fan-calibration', 'software': {'name': 'KD BOX XBT Mod', 'version': __version__}})
        digest = hashlib.sha256(arm_header()).hexdigest()
        def command(cmd):
            if cmd == 'cat /root/version':
                return '2.2.1\n'
            if cmd == 'sha256sum /usr/bin/minerd':
                return PROFILE['install_base']['minerd_sha256'] + '  /usr/bin/minerd'
            if cmd.startswith('sha256sum '):
                return digest + '  controller'
            if cmd.startswith('test ! -f '):
                return 'clear'
            return ''
        device.command = Mock(side_effect=command)
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            payload_fixture(root)
            backup = root / 'backup'
            (backup / 'etc').mkdir(parents=True)
            (backup / 'root/yottamc').mkdir(parents=True)
            (backup / 'etc/inittab').write_text(
                'null::sysinit:/bin/mount -o remount,rw /\n'
                'ttyAMA3::respawn:xminerd\n')
            (backup / 'root/yottamc/index.html').write_text('stock dashboard')
            device.backup = Mock(return_value=backup)
            device.inspect.return_value.update(model='Goldshell-KDBox', mcbversion='MCB_V5_4')
            device.install(root, '123456789', backup, progress=lambda text: None,
                           owner_authorized=True)
        writes = {call.args[0]: call.args[1] for call in device.write.call_args_list}
        self.assertEqual(json.loads(writes['/usr/config/kdbox-xbt/config.json'])['admin_password'], '123456789')
        self.assertFalse(any('/user/updatepd' in call.args[0] for call in device.api.call_args_list))
        self.assertEqual(writes['/usr/config/kdbox-xbt/controller'], arm_header())
        self.assertIn(b'/root/kdbox-xbt-recovery.sh mining', writes['/etc/inittab'])
        self.assertIn(b'/usr/config/kdbox-xbt/boot.sh', writes['/etc/init.d/S99kdbox-xbt'])
        self.assertIn('/usr/config/kdbox-xbt/approved.json', writes)
        self.assertEqual(json.loads(writes['/usr/config/kdbox-xbt/algorithm.json']), 1)
        self.assertEqual(json.loads(writes['/usr/config/minerd/settings/algosetting.json'])['algo_select'],1)


if __name__ == '__main__':
    unittest.main()
