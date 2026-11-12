import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

from flasher.device import Device, installation_algorithm
from test_distribution import payload_fixture, arm_header


class InstallationAlgorithm(unittest.TestCase):
    def test_new_install_defaults_to_xbt_and_repairs_keep_owner_choice(self):
        with tempfile.TemporaryDirectory() as folder:
            backup=Path(folder)
            self.assertEqual(installation_algorithm(backup,{'algo_select':0}),1)
            for invalid in (None, {}, {'algo_select':True}, {'algo_select':2}):
                with self.assertRaises(RuntimeError):installation_algorithm(backup,invalid)
            runtime=backup/'usr/config/kdbox-xbt';runtime.mkdir(parents=True)
            (runtime/'config.json').write_text('{}')
            self.assertEqual(installation_algorithm(backup,{'algo_select':0}),0)
            (runtime/'algorithm.json').write_text('1')
            self.assertEqual(installation_algorithm(backup,{'algo_select':0}),1)
            for bad in ('true','2','null','"1"'):
                (runtime/'algorithm.json').write_text(bad)
                with self.assertRaises(RuntimeError):installation_algorithm(backup,{'algo_select':0})

    def test_unconfirmed_native_write_does_not_start_application_or_claim_success(self):
        import hashlib
        d=Device.__new__(Device);d.host='192.168.1.50'
        d.verify_base=Mock(return_value={'firmware':'2.2.1','model':'Goldshell-KDBox','mcbversion':'MCB_V5_4'})
        d.installation_state=Mock(return_value='new');d.require_space=Mock();d.write=Mock()
        d.api=Mock(side_effect=[{'JWT Token':'owner'},{'algo_select':0},{},{'algo_select':0}])
        d._api_once=Mock()
        digest=hashlib.sha256(arm_header()).hexdigest()
        d.command=Mock(side_effect=lambda cmd:digest+'  controller' if cmd.startswith('sha256sum ') else 'clear' if cmd.startswith('test ! -f ') else '')
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);payload_fixture(root);backup=root/'backup'
            for name,data in [('etc/inittab','ttyAMA3::respawn:xminerd\n'),('root/yottamc/index.html','stock')]:
                p=backup/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(data)
            d.backup=Mock(return_value=backup)
            with self.assertRaisesRegex(RuntimeError,'did not confirm'):
                d.install(root,'owner',backup,progress=lambda m:None,owner_authorized=True)
        d._api_once.assert_not_called()
        self.assertFalse(any('nohup /usr/config/kdbox-xbt/boot.sh' in c.args[0] for c in d.command.call_args_list))
        self.assertTrue(d.installation_started)
        self.assertEqual(json.loads(next(c.args[1] for c in d.write.call_args_list if c.args[0].endswith('/algorithm.json'))),1)
