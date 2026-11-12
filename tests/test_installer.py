import http.client
import json
from pathlib import Path
import threading
import tempfile
import time
import unittest
from unittest.mock import Mock, patch

from flasher import discovery
from flasher.wizard import Wizard, make_server

ROOT = Path(__file__).resolve().parents[1]
MINER = dict(host='192.168.1.50', model='Goldshell-KDBox', firmware='2.2.1', board='MCB_V5_4', compatible=True)


def wait_for(wizard, stage):
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        state = wizard.snapshot()
        if state['stage'] == stage and (not state['busy'] or stage == 'identity'):
            return state
        time.sleep(.01)
    raise AssertionError(wizard.snapshot())


class DiscoveryTests(unittest.TestCase):
    def test_only_small_private_networks_are_scanned(self):
        for value in ('8.8.8.0/24', '10.0.0.0/8', '127.0.0.0/24', '169.254.0.0/24', '224.0.0.0/24'):
            with self.subTest(value=value), self.assertRaises(ValueError), patch.object(discovery, 'probe') as probe:
                discovery.discover([value])
                probe.assert_not_called()
        with patch.object(discovery, 'probe', return_value=MINER) as probe:
            self.assertEqual(len(discovery.discover(['192.168.1.0/30'])), 2)
            self.assertEqual(probe.call_count, 2)

    def test_linux_and_windows_networks_are_bounded_and_deduplicated(self):
        cases = [('Linux', '[{"addr_info":[{"family":"inet","local":"192.168.1.2","prefixlen":16},{"family":"inet","local":"127.0.0.1","prefixlen":8}]}]'),
                 ('Windows', '{"IPAddress":"192.168.1.2","PrefixLength":16}'),
                 ('Darwin', 'en0: flags\n inet 192.168.1.2 netmask 0xffff0000 broadcast 192.168.255.255\n')]
        for system, output in cases:
            with self.subTest(system=system), patch.object(discovery.platform, 'system', return_value=system), patch.object(discovery.subprocess, 'run', return_value=Mock(stdout=output)):
                self.assertEqual(discovery.local_networks(), ['192.168.1.0/24'])

    def test_probe_is_read_only_and_does_not_follow_redirects(self):
        connection = Mock()
        connection.getresponse.return_value.status = 200
        connection.getresponse.return_value.read.return_value = json.dumps(dict(MINER, mcbversion='MCB_V5_4', password='never-return-this')).encode()
        with patch.object(discovery.http.client, 'HTTPConnection', return_value=connection):
            self.assertEqual(discovery.probe(MINER['host']), MINER)
            connection.request.assert_called_once_with('GET', '/mcb/status', headers={'Accept': 'application/json'})
            self.assertNotIn('password', discovery.probe(MINER['host']))
            connection.getresponse.return_value.status = 302
            self.assertIsNone(discovery.probe(MINER['host']))
        connection.close.assert_called()


class WizardTests(unittest.TestCase):
    def setUp(self):
        self.patches = [patch('flasher.wizard.local_networks', return_value=['192.168.1.0/24']),
                        patch('flasher.wizard.backup_root', return_value=ROOT / '.not-a-real-backup-dir')]
        for p in self.patches:
            p.start()
            self.addCleanup(p.stop)

    def test_confirmed_first_install_and_credentials_never_exposed(self):
        device = Mock()
        device.verify_base.return_value = MINER
        device.api.return_value = {'JWT Token': 'private-session'}
        device.command.return_value = ''
        device.install.return_value = Path('/private/backup')
        def factory(host, ssh, confirm, **kwargs):
            self.assertEqual((host, ssh), (MINER['host'], 'private-ssh'))
            if not confirm(host, 'SHA256:verified-key'):
                raise ValueError('Identity not confirmed')
            return device
        wizard = Wizard(ROOT, factory)
        self.addCleanup(wizard.close)
        wizard.update(stage='select', devices=[MINER])
        with self.assertRaises(ValueError):
            wizard.action('install', {'authorized': True})
        wizard.action('connect', dict(host=MINER['host'], web_password='private-web', ssh_password='private-ssh', authorized=True))
        wait_for(wizard, 'identity')
        device.verify_base.assert_not_called()
        with patch('flasher.wizard.validate_payload', return_value=b'arm-controller'):
            wizard.action('identity', {'accept': True})
            state = wait_for(wizard, 'review')
        device.install.assert_not_called()
        for secret in ('private-web', 'private-ssh', 'private-session'):
            self.assertNotIn(secret, json.dumps(state))
        with self.assertRaises(ValueError):
            wizard.action('install', {'authorized': False})
        wizard.action('install', {'authorized': True})
        wait_for(wizard, 'done')
        self.assertEqual(device.install.call_count, 1)
        self.assertEqual(device.install.call_args.kwargs, {'owner_authorized': True})
        self.assertEqual(device.install.call_args.args[1], 'private-web')
        device.close.assert_called()

    def test_cancelled_identity_never_installs_and_secrets_redacted(self):
        def factory(host, password, confirm, **kwargs):
            confirm(host, 'SHA256:test')
            raise ValueError('Rejected private-ssh private-web')
        wizard = Wizard(ROOT, factory)
        wizard.update(stage='select', devices=[MINER])
        wizard.action('connect', dict(host=MINER['host'], ssh_password='private-ssh', web_password='private-web', authorized=True))
        wait_for(wizard, 'identity')
        wizard.action('identity', {'accept': False})
        state = wait_for(wizard, 'error')
        self.assertNotIn('private-ssh', json.dumps(state))
        self.assertNotIn('private-web', json.dumps(state))
        self.assertIn('[redacted]', state['error'])

    def test_existing_install_is_not_overwritten_and_restore_needs_confirmation(self):
        device = Mock()
        device.verify_base.return_value = MINER
        device.api.return_value = {'JWT Token': 'token'}
        device.installation_state.return_value = 'installed'
        wizard = Wizard(ROOT, Mock(return_value=device))
        wizard.update(stage='select', devices=[MINER])
        wizard.action('connect', dict(host=MINER['host'], ssh_password='ssh', web_password='web', authorized=True))
        wait_for(wizard, 'existing')
        with self.assertRaises(ValueError):
            wizard.action('install', {'authorized': True})
        with self.assertRaises(ValueError):
            wizard.action('restore', {'authorized': True, 'backup': '/etc/passwd'})
        wizard.backups['known'] = Path('/private/backup')
        with self.assertRaises(ValueError):
            wizard.action('restore', {'authorized': False, 'backup': 'known'})
        wizard.action('restore', {'authorized': True, 'backup': 'known'})
        wait_for(wizard, 'done')
        device.install.assert_not_called()
        device.restore.assert_called_once_with(Path('/private/backup'))

    def test_failed_install_exposes_new_backup_for_recovery_without_retry(self):
        with tempfile.TemporaryDirectory() as folder, patch('flasher.wizard.backup_root', return_value=Path(folder)):
            wizard = Wizard(ROOT)
            wizard.device = Mock()
            wizard.web_password = 'private-web'
            wizard.update(stage='review', host=MINER['host'])
            def fail_install(root, password, backup, progress, **kwargs):
                backup.mkdir()
                (backup / 'device.json').write_text('{"mac":"test-miner"}')
                raise ValueError('Write interrupted private-web')
            device = wizard.device
            device.install.side_effect = fail_install
            wizard.action('install', {'authorized': True})
            state = wait_for(wizard, 'error')
            self.assertNotIn('private-web', json.dumps(state))
            self.assertEqual(len(state['backups']), 1)
            self.assertEqual(Path(state['backup']), wizard.backups[state['backups'][0]['id']])
            self.assertEqual(device.install.call_count, 1)
            device.close.assert_called_once()
            with self.assertRaises(ValueError):
                wizard.action('install', {'authorized': True})

    def test_unconfirmed_scan_and_unsupported_selection_cannot_connect(self):
        factory = Mock()
        wizard = Wizard(ROOT, factory)
        with self.assertRaises(ValueError):
            wizard.action('scan', {'authorized': False})
        wizard.update(devices=[dict(MINER, compatible=False, firmware='2.2.4')])
        with self.assertRaises(ValueError):
            wizard.action('connect', dict(host=MINER['host'], web_password='web', authorized=True))
        factory.assert_not_called()

    def test_local_server_rejects_cross_origin_dns_rebinding_and_missing_token(self):
        wizard = Wizard(ROOT)
        server = make_server(wizard)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        def request(path='/api/state', headers=None, method='GET', body=None):
            connection = http.client.HTTPConnection('127.0.0.1', server.server_port)
            connection.request(method, path, body=body, headers=headers or {})
            response = connection.getresponse()
            result = (response.status, response.read())
            connection.close()
            return result
        self.assertEqual(request()[0], 403)
        good = {'X-Installer-Token': wizard.token}
        self.assertEqual(request(headers=good)[0], 200)
        for extra in ({'Origin': 'https://attacker.invalid'}, {'Host': 'attacker.invalid'}):
            self.assertEqual(request(headers=dict(good, **extra))[0], 403)
        self.assertEqual(request('/../flasher/defaults.py')[0], 404)
        self.assertEqual(request('/api/scan', dict(good, **{'Content-Type': 'text/plain'}), 'POST', '{}')[0], 400)
        self.assertEqual(request('/api/scan', dict(good, **{'Content-Type': 'application/json'}), 'POST', '[]')[0], 400)
        wizard.update(busy=True)
        self.assertEqual(request('/api/close', dict(good, **{'Content-Type': 'application/json'}), 'POST', '{}')[0], 400)

if __name__ == '__main__':
    unittest.main()
