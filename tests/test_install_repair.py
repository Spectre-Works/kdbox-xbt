import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

from flasher.device import Device
from flasher.payload import WEB_ASSETS
from flasher.wizard import Wizard
from test_installer import MINER, ROOT, wait_for
from test_distribution import payload_fixture


FLAGS = ['directory', 'config', 'controller', 'startup', 'hook', 'dashboard',
         'launcher.sh', 'supervisor.sh', 'boot.sh', *WEB_ASSETS]
HEALTH = {'software': {'name': 'KD BOX XBT Mod', 'version': '1.0'},
          'approved': {}, 'halted': False, 'algorithm':1, 'stale': True, 'startup': 'fan-calibration'}


class InstallationDetection(unittest.TestCase):
    def device(self, flags=FLAGS, health=HEALTH):
        d = Device.__new__(Device)
        d.command = Mock(return_value='\n'.join(flags))
        d.api = Mock(return_value=health)
        return d

    def test_absent_mod_and_reflash_remnants_are_distinct(self):
        for flags, result in [([], 'new'), (['directory', 'config'], 'repair'),
                              ([f for f in FLAGS if f != 'startup'], 'repair'),
                              ([f for f in FLAGS if f != 'dashboard'], 'repair'),
                              ([f for f in FLAGS if f != 'app.js'], 'repair')]:
            with self.subTest(flags=flags):
                d = self.device(flags)
                self.assertEqual(d.installation_state(), result)
                d.api.assert_not_called()

    def test_stock_dashboard_and_generic_assets_are_not_previous_installations(self):
        for flags in (['index.html'], ['previous-favicon'],
                      [name for name in WEB_ASSETS if name != 'spectre-logo.png'] + ['previous-favicon']):
            with self.subTest(flags=flags):
                d = self.device(flags)
                self.assertEqual(d.installation_state(), 'new')
                d.api.assert_not_called()

    def test_mod_specific_remnants_still_require_repair(self):
        for marker in ('directory', 'config', 'controller', 'startup', 'hook', 'dashboard', 'spectre-logo.png'):
            with self.subTest(marker=marker):
                d = self.device([marker, 'index.html'])
                self.assertEqual(d.installation_state(), 'repair')
                d.api.assert_not_called()

    def test_working_previous_favicon_version_is_still_installed(self):
        flags=[f for f in FLAGS if not f.startswith('favicon-')]+['previous-favicon']
        self.assertEqual(self.device(flags).installation_state(),'installed')

    def test_complete_files_need_a_real_responding_controller(self):
        for health in ({}, {'software': {'name': 'another service'}},
                       dict(HEALTH, halted='false'), dict(HEALTH, approved=None)):
            self.assertEqual(self.device(health=health).installation_state(), 'repair')
        d = self.device()
        d.api.side_effect = RuntimeError('Connection refused')
        self.assertEqual(d.installation_state(), 'repair')

    def test_calibration_and_safety_stop_are_installed_not_repair(self):
        for halted in (True, False):
            self.assertEqual(self.device(health=dict(HEALTH, halted=halted)).installation_state(), 'installed')

    def test_direct_install_cannot_overwrite_running_or_unconfirmed_repair(self):
        for state in ('installed', 'repair'):
            d = self.device()
            d.verify_base = Mock(return_value={'firmware': '2.2.1', 'model':'Goldshell-KDBox', 'mcbversion':'MCB_V5_4'})
            d.command.return_value = ''
            d.installation_state = Mock(return_value=state)
            d.backup = Mock()
            d.write = Mock()
            with self.assertRaises(RuntimeError):
                d.install('unused', 'owner', 'backup', owner_authorized=True)
            d.backup.assert_not_called()
            d.write.assert_not_called()
            self.assertFalse(d.installation_started)


class RepairWizard(unittest.TestCase):
    def test_stock_dashboard_reaches_new_install_review(self):
        with patch('flasher.wizard.local_networks', return_value=[]), patch('flasher.wizard.backup_root', return_value=ROOT/'.not-a-real-backup-dir'), patch('flasher.wizard.validate_payload', return_value=b'controller'):
            d = Mock()
            d.verify_base.return_value = MINER
            d.api.return_value = {'JWT Token': 'private'}
            d.command.return_value = 'index.html\n'
            d.installation_state = lambda: Device.installation_state(d)
            w = Wizard(ROOT, Mock(return_value=d))
            self.addCleanup(w.close)
            w.update(devices=[MINER])
            w.action('connect', dict(host=MINER['host'], authorized=True, web_password='owner'))
            state = wait_for(w, 'review')
            self.assertNotIn('repair', state['message'].lower())
            d.install.assert_not_called()

    def test_repair_requires_separate_confirmation_and_passes_authorization(self):
        with patch('flasher.wizard.local_networks', return_value=[]), patch('flasher.wizard.backup_root', return_value=ROOT/'.not-a-real-backup-dir'), patch('flasher.wizard.validate_payload', return_value=b'controller'):
            d = Mock()
            d.verify_base.return_value = MINER
            d.api.return_value = {'JWT Token': 'private'}
            d.installation_state.return_value = 'repair'
            w = Wizard(ROOT, Mock(return_value=d))
            self.addCleanup(w.close)
            w.update(devices=[MINER])
            w.action('connect', dict(host=MINER['host'], authorized=True, web_password='owner'))
            wait_for(w, 'repair')
            d.install.assert_not_called()
            with self.assertRaises(ValueError):
                w.action('install', {'authorized': False})
            w.action('install', {'authorized': True})
            wait_for(w, 'done')
            self.assertEqual(d.install.call_args.kwargs, {'owner_authorized': True, 'repair_authorized': True})
            self.assertNotIn('owner', json.dumps(w.snapshot()))

    def test_missing_payload_still_offers_repair_backup_recovery_but_cannot_install(self):
        with tempfile.TemporaryDirectory() as folder, patch('flasher.wizard.local_networks', return_value=[]), patch('flasher.wizard.backup_root', return_value=Path(folder)/'backups'):
            d = Mock()
            d.verify_base.return_value = MINER
            d.api.return_value = {'JWT Token': 'private'}
            d.installation_state.return_value = 'repair'
            w = Wizard(folder, Mock(return_value=d))
            self.addCleanup(w.close)
            w.update(devices=[MINER])
            w.action('connect', dict(host=MINER['host'], authorized=True, web_password='owner'))
            state = wait_for(w, 'repair')
            self.assertFalse(state['payload_ready'])
            with self.assertRaises(ValueError):
                w.action('install', {'authorized': True})
            d.install.assert_not_called()
            d.require_space.assert_not_called()


class RepairPreservation(unittest.TestCase):
    def test_repair_keeps_exact_config_and_approved_algorithm_and_accepts_startup(self):
        # Exercise the production install path with a fake device filesystem.
        from flasher import __version__
        from test_distribution import arm_header
        import hashlib
        d = Device.__new__(Device)
        d.host = MINER['host']
        d.verify_base = Mock(return_value={'firmware': '2.2.1', 'model':'Goldshell-KDBox', 'mcbversion':'MCB_V5_4'})
        d.installation_state = Mock(return_value='repair')
        d.require_space = Mock()
        d.write = Mock()
        d.api = Mock(side_effect=[{'JWT Token': 'owner'}, {'algo_select': 1}, {}, {'algo_select':1}])
        d._api_once = Mock(return_value=dict(HEALTH, software={'name': 'KD BOX XBT Mod', 'version': __version__}))
        digest = hashlib.sha256(arm_header()).hexdigest()
        d.command = Mock(side_effect=lambda cmd: digest+'  controller' if cmd.startswith('sha256sum ') else 'clear' if cmd.startswith('test ! -f ') else '')
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            payload_fixture(root)
            backup = root/'backup'
            for rel, data in {'etc/inittab':'ttyAMA3::respawn:xminerd\n', 'root/yottamc/index.html':'stock',
                'usr/config/kdbox-xbt/config.json':'{ "admin_password":"owner", "chip_target":78, "automatic_limits":{"maximum_clock":755,"minimum_millivolts":430} }',
                'usr/config/kdbox-xbt/approved.json':'{"clock":740,"millivolts":430}',
                'usr/config/kdbox-xbt/algorithm.json':'1'}.items():
                p=backup/rel;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)
            d.backup = Mock(return_value=backup)
            d.install(root, 'owner', backup, progress=lambda m:None, owner_authorized=True, repair_authorized=True)
        writes = {c.args[0]:c.args[1] for c in d.write.call_args_list}
        for name in ('config.json','approved.json','algorithm.json','halted.json'):
            self.assertNotIn('/usr/config/kdbox-xbt/'+name,writes)
        self.assertEqual(json.loads(writes['/usr/config/minerd/settings/algosetting.json'])['algo_select'],1)
        self.assertIn('/root/yottamc/legacy.html', writes)
        self.assertEqual(d._api_once.call_count,1)


class PrivateBackupLocation(unittest.TestCase):
    def test_private_destination_override_and_reject_relative_paths(self):
        from flasher.branding import backup_root
        with patch.dict('os.environ', {'KDBOX_XBT_BACKUP_DIR': str(ROOT/'private-backups')}):
            self.assertEqual(backup_root(), ROOT/'private-backups')
        with patch.dict('os.environ', {'KDBOX_XBT_BACKUP_DIR':'relative-backups'}):
            with self.assertRaisesRegex(ValueError,'absolute'):
                backup_root()
