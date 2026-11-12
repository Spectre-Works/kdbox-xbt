import unittest
from unittest.mock import Mock, call

from flasher.device import Device, PROFILE


class StockInstallBase(unittest.TestCase):
    def test_install_checks_stock_management_build_before_backup_or_writes(self):
        base = PROFILE['install_base']
        for version, local_version, digest in (
            ('2.1.3', '2.1.3', base['minerd_sha256']),
            ('2.2.3', '2.2.3', base['minerd_sha256']),
            ('2.2.1', '2.2.4', base['minerd_sha256']),
            ('2.2.1', '2.2.1', 'different-digest'),
        ):
            with self.subTest(version=version, local_version=local_version, digest=digest):
                device = Device.__new__(Device)
                device.inspect = Mock(return_value={'firmware': version})
                device.command = Mock(side_effect=[local_version, digest + '  /usr/bin/minerd'])
                device.backup = Mock()
                device.write = Mock()
                with self.assertRaisesRegex(RuntimeError, r'stock 2\.2\.1'):
                    device.install('root', 'owner-password', 'backup', owner_authorized=True)
                device.backup.assert_not_called()
                device.write.assert_not_called()
                self.assertFalse(device.installation_started)
                if version != base['version']:
                    device.command.assert_not_called()

    def test_stock_base_uses_live_board_architecture_version_and_hash(self):
        device = Device.__new__(Device)
        info = {'model': 'Goldshell-KDBox', 'mcbversion': 'MCB_V5_4', 'firmware': '2.2.1'}
        device.api = Mock(return_value=info)
        device.command = Mock(side_effect=[
            'armv7l\n', '2.2.1\n', PROFILE['install_base']['minerd_sha256'] + '  /usr/bin/minerd',
        ])
        self.assertEqual(device.verify_base(), info)
        device.api.assert_called_once_with('/mcb/status')
        self.assertEqual(device.command.call_args_list, [
            call('uname -m'), call('cat /root/version'), call('sha256sum /usr/bin/minerd'),
        ])


if __name__ == '__main__':
    unittest.main()
