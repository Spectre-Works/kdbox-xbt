import hashlib
import json
from pathlib import Path
import shlex
import tempfile
import unittest
from unittest.mock import Mock, patch

from flasher.device import Device
from flasher.payload import WEB_ASSETS, SCRIPTS
from flasher.update import (PROTECTED, RUNTIME, WEBROOT, UPDATE_PAUSE, compare_build,
                            package_build, protected_hashes, update_device)
from flasher.wizard import Wizard
from test_distribution import payload_fixture
from test_installer import MINER, wait_for


def update_fixture(root):
    manifest = payload_fixture(root)
    manifest.update(source_commit='a'*40, source_dirty=False)
    (root/'build/payload.json').write_text(json.dumps(manifest))
    return package_build(root)


class BuildComparison(unittest.TestCase):
    def test_compares_revision_version_dirty_state_and_binary_even_at_same_version(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);target=update_fixture(root)
            for changes in ({}, {'commit':'b'*40}, {'dirty':True}, {'version':'old'},
                            {'commit':None}, {'sha256':'0'*64}):
                with self.subTest(changes=changes):
                    installed=dict(target, **changes)
                    d=Mock()
                    d.api.return_value={'software':installed}
                    d.command.return_value=installed['sha256']+'  controller'
                    result=compare_build(d,root)
                    self.assertEqual(result['matches'],not changes)
                    self.assertEqual(result['package'],target)
                    d.write.assert_not_called();d.backup.assert_not_called()


class UpdateWizard(unittest.TestCase):
    def test_different_hash_offers_confirmed_update_not_repair_or_new_install(self):
        with tempfile.TemporaryDirectory() as folder, patch('flasher.wizard.local_networks',return_value=[]):
            root=Path(folder);target=update_fixture(root)
            with patch('flasher.wizard.backup_root',return_value=root/'backups'):
                d=Mock();d.verify_base.return_value=MINER
                d.api.return_value={'JWT Token':'private'};d.installation_state.return_value='installed'
                d.build_comparison.return_value={'matches':False,'installed':dict(target,commit='b'*40),'package':target}
                w=Wizard(root,Mock(return_value=d));self.addCleanup(w.close);w.update(devices=[MINER])
                w.action('connect',{'host':MINER['host'],'authorized':True,'web_password':'owner'})
                state=wait_for(w,'update')
                self.assertEqual(state['builds']['package']['commit'],'a'*40)
                d.update_software.assert_not_called();d.install.assert_not_called()
                with self.assertRaises(ValueError):w.action('install',{'authorized':False})
                w.action('install',{'authorized':True});wait_for(w,'done')
                self.assertEqual(d.update_software.call_args.kwargs,{'owner_authorized':True,'target':target})
                self.assertEqual(w.snapshot()['operation'],'update')
                d.install.assert_not_called();self.assertNotIn('owner',json.dumps(w.snapshot()))


class FakeUpdateDevice:
    def __init__(self, target, stopped):
        self.target=target
        self.files={path:b'original setting' for path in PROTECTED}
        self.files[RUNTIME+'/config.json']=b'{"admin_password":"owner"}'
        self.files[RUNTIME+'/controller']=b'old controller'
        for name in SCRIPTS:self.files[RUNTIME+'/'+name]=b'old script'
        for name in WEB_ASSETS:self.files[WEBROOT+'/'+name]=b'old asset'
        if stopped:self.files[RUNTIME+'/halted.json']=b'{"reason":"original thermal stop"}'
        else:del self.files[RUNTIME+'/halted.json']
        self.original=dict(self.files);self.events=[];self.writes=[]
        self.verify_base=Mock();self.require_space=Mock();self.installation_state=Mock(return_value='installed')

    def api(self,path,data=None):
        if path.startswith('/user/login?'):return {'JWT Token':'native'}
        if path=='/api/status':
            new=hashlib.sha256(self.files[RUNTIME+'/controller']).hexdigest()==self.target['sha256']
            return {'halted':RUNTIME+'/halted.json' in self.files,
                    'software':self.target if new else dict(self.target,commit='b'*40)}
        raise AssertionError('unexpected native operation: '+path)

    def write(self,path,data,mode=0o600):
        self.writes.append(path);self.files[path]=data

    def backup(self,destination):
        self.events.append('backup');destination=Path(destination);destination.mkdir()
        manifest={}
        for path,data in self.files.items():
            local=destination/path.lstrip('/');local.parent.mkdir(parents=True,exist_ok=True);local.write_bytes(data)
            manifest[path]={'mode':0o700 if path.startswith(RUNTIME+'/') else 0o644,
                            'sha256':hashlib.sha256(data).hexdigest()}
        (destination/'manifest.json').write_text(json.dumps(manifest))
        return destination

    def command(self,command):
        if command.startswith('if test -f '):
            rows=[]
            for path in PROTECTED:
                rows.append((hashlib.sha256(self.files[path]).hexdigest() if path in self.files else 'absent')+'  '+path)
            return '\n'.join(rows)
        args=shlex.split(command)
        if args[:2]==['sha256sum',RUNTIME+'/controller']:return hashlib.sha256(self.files[args[1]]).hexdigest()+'  controller'
        if args[:2]==['sha256sum','/proc/123/exe']:return hashlib.sha256(self.files[RUNTIME+'/controller']).hexdigest()+'  exe'
        if command.startswith('test ! -e '):return 'clear'
        if args[:2]==['mktemp','-d']:return args[2].replace('XXXXXX','abc123')
        if command.endswith(' -h 2>&1'):return 'listen address'
        if args[0]=='ln':
            if args[2] in self.files:raise RuntimeError('stop exists')
            self.files[args[2]]=self.files[args[1]];return ''
        if args[0]=='mv':self.files[args[2]]=self.files.pop(args[1]);return ''
        if args[:2]==['kill','-KILL']:self.events.append('restart');return ''
        if args[:2]==['rm','-f']:return ''
        raise AssertionError('unexpected command: '+command)


class UpdateExecution(unittest.TestCase):
    def test_updates_only_application_and_keeps_existing_stop_or_creates_review_pause(self):
        for stopped in (True,False):
            with self.subTest(stopped=stopped), tempfile.TemporaryDirectory() as folder:
                root=Path(folder);target=update_fixture(root);d=FakeUpdateDevice(target,stopped)
                def pause(device,pid):
                    self.assertEqual(pid,'123');self.assertIn(RUNTIME+'/halted.json',device.files)
                    self.assertIn('backup',device.events);device.events.append('pause')
                with patch('flasher.update.controller_pid',return_value='123'),patch('flasher.update.pause_mining',side_effect=pause):
                    update_device(d,root,'owner',root/'backup',lambda m:None,owner_authorized=True,target=target)
                self.assertEqual(d.files[RUNTIME+'/controller'],(root/'build/kdbox-xbt-controller').read_bytes())
                for path in PROTECTED:
                    if path!=RUNTIME+'/halted.json':self.assertEqual(d.files[path],d.original[path])
                if stopped:self.assertEqual(d.files[RUNTIME+'/halted.json'],d.original[RUNTIME+'/halted.json'])
                else:self.assertEqual(json.loads(d.files[RUNTIME+'/halted.json'])['reason'],UPDATE_PAUSE)
                self.assertEqual(d.events,['backup','pause','restart'])
                self.assertTrue(all(path not in PROTECTED for path in d.writes))

    def test_refuses_changed_package_without_touching_device(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);target=update_fixture(root);d=FakeUpdateDevice(target,True)
            with self.assertRaisesRegex(RuntimeError,'changed after review'):
                update_device(d,root,'owner',root/'backup',lambda m:None,owner_authorized=True,target=dict(target,commit='c'*40))
            self.assertEqual(d.files,d.original);self.assertEqual(d.events,[])

    def test_verification_failure_restores_application_and_never_clears_stop(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);target=update_fixture(root);d=FakeUpdateDevice(target,True)
            with patch('flasher.update.controller_pid',return_value='123'),patch('flasher.update.pause_mining'),patch('flasher.update.wait_build',side_effect=RuntimeError('verification failed')):
                with self.assertRaisesRegex(RuntimeError,'verification failed'):
                    update_device(d,root,'owner',root/'backup',lambda m:None,owner_authorized=True,target=target)
            for path,data in d.original.items():self.assertEqual(d.files[path],data)

    def test_authorization_required_before_device_reads_or_writes(self):
        d=Mock()
        with self.assertRaisesRegex(RuntimeError,'Confirm'):
            update_device(d,'unused','owner','unused',lambda m:None)
        d.verify_base.assert_not_called();d.backup.assert_not_called();d.write.assert_not_called()
