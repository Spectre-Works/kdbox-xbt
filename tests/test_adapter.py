import unittest
import json
import tempfile
from pathlib import Path
from unittest.mock import Mock
from flasher.device import Device, PROFILE, startup_config

class Compatibility(unittest.TestCase):
    def test_newer_startup_preserves_read_only_root_and_single_fan_owner(self):
        original = 'null::sysinit:/bin/mount -o remount,ro /\nttyAMA3::respawn:xminerd\nttyAMA3::respawn:fanctrl\n'
        for version in ('2.2.3',):
            updated = startup_config(original, version)
            self.assertIn('remount,ro /', updated)
            self.assertNotIn('respawn:fanctrl', updated)
            self.assertIn('respawn:/root/kdbox-xbt-recovery.sh mining', updated)
            self.assertEqual(startup_config(updated, version), updated)
            with self.assertRaises(RuntimeError): startup_config(original.replace('ttyAMA3::respawn:fanctrl', ''), version)


    def test_insufficient_or_unreadable_disk_space_rejected(self):
        for output in ('Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/mmcblk0p2 60000 60000 0 100% /usr/config\n', 'unavailable'):
            device=Device.__new__(Device)
            device.command=Mock(return_value=output)
            with self.assertRaises(RuntimeError): device.require_space('/usr/config',1024)

    def test_space_check_uses_available_kib(self):
        device=Device.__new__(Device)
        device.command=Mock(return_value='Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/mmcblk0p2 60000 10000 50000 17% /usr/config\n')
        device.require_space('/usr/config',8*1024*1024)
    def test_unsupported_firmware_never_reaches_mutation(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':'99.0.0','mcbversion':'MCB_V5_4'})
        device.command = Mock()
        with self.assertRaises(RuntimeError): device.inspect()
        device.command.assert_not_called()

    def test_different_model_is_rejected(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-HSBox','firmware':'2.2.1','mcbversion':'MCB_V5_4'})
        with self.assertRaises(RuntimeError): device.inspect()

    def test_supported_board(self):
        for version in PROFILE['firmwares']:
            with self.subTest(version=version):
                device = Device.__new__(Device)
                device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':version,'mcbversion':'MCB_V5_4'})
                device.command = Mock(return_value='armv7l\n')
                self.assertEqual(device.inspect()['firmware'],version)

    def test_wrong_architecture_is_rejected(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':'2.2.1','mcbversion':'MCB_V5_4'})
        device.command = Mock(return_value='aarch64\n')
        with self.assertRaises(RuntimeError): device.inspect()

    def test_missing_api_board_is_verified_over_ssh(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':PROFILE['firmwares'][0]})
        device.command = Mock(side_effect=['MCB_V5_4\n', 'armv7l\n'])
        self.assertEqual(device.inspect()['mcbversion'], 'MCB_V5_4')
        self.assertEqual(device.command.call_args_list[0].args, ('cat /root/mcbversion',))

    def test_missing_api_board_does_not_accept_wrong_board(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':PROFILE['firmwares'][0]})
        device.command = Mock(return_value='MCB_V6\n')
        with self.assertRaises(RuntimeError): device.inspect()

    def test_api_board_mismatch_is_not_overridden(self):
        device = Device.__new__(Device)
        device.api = Mock(return_value={'model':'Goldshell-KDBox','firmware':PROFILE['firmwares'][0],'mcbversion':'MCB_V6'})
        device.command = Mock()
        with self.assertRaises(RuntimeError): device.inspect()
        device.command.assert_not_called()

    def test_cross_firmware_restore_rejected_before_writes(self):
        for identity in ({'firmware':'2.2.1'}, {}):
            with self.subTest(identity=identity), tempfile.TemporaryDirectory() as folder:
                (Path(folder)/'device.json').write_text(json.dumps(identity))
                device = Device.__new__(Device)
                device.inspect = Mock(return_value={'firmware':'2.2.0'})
                device.command = Mock()
                device.write = Mock()
                with self.assertRaisesRegex(RuntimeError, 'different firmware'): device.restore(folder)
                device.command.assert_not_called()
                device.write.assert_not_called()


    def test_224_is_rejected_before_any_change(self):
        device=Device.__new__(Device)
        device.api=Mock(return_value={'model':'Goldshell-KDBox','firmware':'2.2.4','mcbversion':'MCB_V5_4'})
        device.command=Mock()
        with self.assertRaises(RuntimeError): device.inspect()
        device.command.assert_not_called()

if __name__=='__main__':unittest.main()
