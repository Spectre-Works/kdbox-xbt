import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest

from flasher.device import startup_config, startup_hook
from flasher.payload import RECOVERY_PATH

ROOT = Path(__file__).resolve().parents[1]


class RecoveryBootstrap(unittest.TestCase):
    def fixture(self, folder):
        base=Path(folder)
        for name in ('root','usr/config','var/run','sys/class/fans/fan0','sys/class/fans/fan1','bin'):
            (base/name).mkdir(parents=True,exist_ok=True)
        for name,value in (('version','2.2.1'),('minername','Goldshell-KDBox'),('mcbversion','MCB_V5_4')):
            (base/'root'/name).write_text(value+'\n')
        script=(ROOT/'controller/recovery-bootstrap.sh').read_text()
        for name in ('/root/','/usr/config/','/var/run/','/sys/class/fans/'):
            script=script.replace(name,str(base/name.lstrip('/'))+'/')
        script=script.replace('/usr/sbin/sshd',str(base/'bin/sshd'))
        target=base/'root/kdbox-xbt-recovery.sh';target.write_text(script)
        for name,body in {'pidof':'exit 1','sleep':'/bin/sleep 0.05',
                          'sshd':'echo started >>"'+str(base/'ssh-started')+'"',
                          'xminerd':'echo started >"'+str(base/'mining-started')+'"'}.items():
            p=base/'bin'/name;p.write_text('#!/bin/sh\n'+body+'\n');p.chmod(0o700)
        return base,target,dict(os.environ,PATH=str(base/'bin')+':'+os.environ['PATH'])

    def wait(self, predicate):
        deadline=time.monotonic()+3
        while not predicate():
            if time.monotonic()>deadline:self.fail('bootstrap condition timed out')
            time.sleep(.02)

    def test_missing_runtime_holds_cooling_and_persistent_stop_blocks_mining(self):
        with tempfile.TemporaryDirectory() as folder:
            base,script,env=self.fixture(folder)
            p=subprocess.Popen(['sh',str(script),'mining'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            try:
                self.wait(lambda:(base/'sys/class/fans/fan0/lvl').exists())
                self.assertEqual((base/'sys/class/fans/fan0/lvl').read_text().strip(),'100')
                self.assertFalse((base/'mining-started').exists())
                runtime=base/'usr/config/kdbox-xbt';runtime.mkdir()
                for name in ('controller','supervisor.sh','boot.sh'):
                    f=runtime/name;f.write_text('#!/bin/sh\nexit 0\n');f.chmod(0o700)
                launcher=(ROOT/'controller/launcher.sh').read_text().replace('/usr/config/',str(base/'usr/config')+'/')
                f=runtime/'launcher.sh';f.write_text(launcher);f.chmod(0o700)
                for name in ('halted.json','config.json','approved.json'):(runtime/name).write_text('{}')
                time.sleep(.2);self.assertFalse((base/'mining-started').exists())
                (runtime/'halted.json').unlink()  # Simulates an explicit resume in this isolated fixture only.
                self.wait(lambda:(base/'mining-started').exists());self.assertEqual(p.wait(timeout=3),0)
            finally:
                if p.poll() is None:p.terminate();p.wait(timeout=3)

    def test_surviving_watcher_reopens_ssh_without_provisioning_and_is_singleton(self):
        with tempfile.TemporaryDirectory() as folder:
            base,script,env=self.fixture(folder)
            p=subprocess.Popen(['sh',str(script),'watch'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            try:
                self.wait(lambda:(base/'ssh-started').exists())
                second=subprocess.run(['sh',str(script),'watch'],env=env,timeout=3,capture_output=True)
                self.assertEqual(second.returncode,0)
                self.assertEqual((base/'var/run/kdbox-xbt-recovery.lock/pid').read_text().strip(),str(p.pid))
                self.assertFalse((base/'usr/config/kdbox-xbt').exists())
                self.assertEqual((base/'sys/class/fans/fan1/lvl').read_text().strip(),'100')
            finally:p.terminate();p.wait(timeout=3)

    def test_boot_migration_is_idempotent_and_hook_has_no_credentials(self):
        for line in ('ttyAMA3::respawn:xminerd\n', 'ttyAMA3::respawn:/usr/config/kdbox-xbt/launcher.sh\n'):
            updated=startup_config(line,'2.2.1')
            self.assertIn(RECOVERY_PATH+' mining',updated)
            self.assertEqual(startup_config(updated,'2.2.1'),updated)
        self.assertIn((RECOVERY_PATH+' watch').encode(),startup_hook())
        subprocess.run(['sh','-n',str(ROOT/'controller/recovery-bootstrap.sh')],check=True)
