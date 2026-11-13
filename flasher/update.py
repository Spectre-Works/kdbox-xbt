"""Compare and update a working installation without changing owner settings."""
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import secrets
import shlex
import time
from urllib.parse import urlencode

from .payload import SCRIPTS, WEB_ASSETS, validate_payload

RUNTIME = '/usr/config/kdbox-xbt'
WEBROOT = '/root/yottamc'
UPDATE_PAUSE = 'Paused for a software update; review settings before resuming'
PROTECTED = (
    '/etc/shadow', '/etc/passwd', '/etc/inittab', '/etc/init.d/S99kdbox-xbt',
    *(RUNTIME+'/'+name for name in ('config.json', 'approved.json', 'algorithm.json', 'halted.json')),
    *('/usr/config/minerd/settings/'+name for name in (
        'login.json', '.login.json', 'wifi.json', '.wifi.json', 'pools.json', '.pools.json',
        'algosetting.json', '.algosetting.json', 'cpbsetting.json', '.cpbsetting.json')),
)


def commit_id(value):
    return value if isinstance(value, str) and re.fullmatch(r'[0-9a-f]{40}|[0-9a-f]{64}', value) else 'unversioned'


def package_build(root):
    binary = validate_payload(root)
    manifest = json.loads((Path(root)/'build/payload.json').read_text(encoding='utf-8'))
    return dict(version=manifest['version'], commit=commit_id(manifest.get('source_commit')),
                dirty=manifest.get('source_dirty') is not False, sha256=hashlib.sha256(binary).hexdigest())


def compare_build(device, root):
    target = package_build(root)
    status = device.api('/api/status')
    software = status.get('software') if isinstance(status, dict) else None
    if not isinstance(software, dict):
        raise RuntimeError('Could not read the installed build. No files were changed.')
    digest = device.command('sha256sum '+RUNTIME+'/controller').split()
    if not digest or not re.fullmatch(r'[0-9a-f]{64}', digest[0]):
        raise RuntimeError('Could not verify the installed controller build. No files were changed.')
    digest = digest[0]
    installed = dict(version=str(software.get('version', 'unknown'))[:64],
                     commit=commit_id(software.get('commit')), dirty=software.get('dirty') is not False,
                     sha256=digest)
    return dict(matches=installed == target, installed=installed, package=target)


def protected_hashes(device):
    commands = [f'if test -f {shlex.quote(p)}; then sha256sum {shlex.quote(p)}; else echo absent {shlex.quote(p)}; fi' for p in PROTECTED]
    result = {}
    for line in device.command('\n'.join(commands)).splitlines():
        digest, path = line.split(maxsplit=1)
        if path not in PROTECTED or (digest != 'absent' and not re.fullmatch(r'[0-9a-f]{64}', digest)):
            raise RuntimeError('Could not verify protected settings.')
        result[path] = digest
    if set(result) != set(PROTECTED):
        raise RuntimeError('Protected settings verification was incomplete.')
    return result


def controller_pid(device):
    pids = device.command('pidof controller || true').split()
    if len(pids) != 1 or not pids[0].isdigit():
        raise RuntimeError('Expected one running KD BOX XBT controller.')
    pid = pids[0]
    if device.command(f'tr "\\000" " " < /proc/{pid}/cmdline').strip() != RUNTIME+'/controller':
        raise RuntimeError('The running controller has an unexpected command line.')
    parent = device.command(f"awk '/^PPid:/ {{print $2}}' /proc/{pid}/status").strip()
    if not parent.isdigit() or device.command(f'cat /proc/{parent}/comm').strip() != 'supervisor.sh':
        raise RuntimeError('The controller is not owned by its expected watchdog.')
    return pid


def pause_mining(device, pid):
    # All large transfers are staged first. Keep the watchdog running, and hold
    # the old controller only for the short atomic application-file replacement.
    device.command(f'''set -e
test -s {RUNTIME}/halted.json
kill -STOP {pid}
for p in $(pidof xminerd); do
 case "$(readlink /proc/$p/exe)" in /usr/bin/xminerd) kill -STOP "$p";; *) exit 1;; esac
done
for p in $(pidof intminer kdaminer); do
 state=$(awk '/^State:/ {{print $2}}' /proc/$p/status)
 test "$state" = Z && continue
 case "$(readlink /proc/$p/exe)" in /usr/bin/intminer|/usr/bin/kdaminer) kill -TERM "$p";; *) exit 1;; esac
done
sleep 1
for p in $(pidof intminer kdaminer); do
 state=$(awk '/^State:/ {{print $2}}' /proc/$p/status)
 test "$state" = Z && continue
 case "$(readlink /proc/$p/exe)" in /usr/bin/intminer|/usr/bin/kdaminer) kill -KILL "$p";; *) exit 1;; esac
done
echo 100 >/sys/class/fans/fan0/lvl
echo 100 >/sys/class/fans/fan1/lvl
''')


def wait_build(device, target, protected):
    deadline = time.monotonic()+60
    while time.monotonic() < deadline:
        try:
            status = device.api('/api/status')
            software = status.get('software', {})
            if (status.get('halted') is True and software.get('version') == target['version']
                    and commit_id(software.get('commit')) == target['commit']
                    and (software.get('dirty') is not False) == target['dirty']):
                pid = controller_pid(device)
                if device.command(f'sha256sum /proc/{pid}/exe').split()[0] == target['sha256']:
                    if protected_hashes(device) != protected:
                        raise RuntimeError('Protected settings changed during the update. Mining remains paused.')
                    return
        except (OSError, ValueError, RuntimeError):
            pass
        time.sleep(2)
    raise RuntimeError('The updated controller could not be verified. Mining remains paused.')


def update_device(device, root, password, destination, progress, *, owner_authorized=False, target=None):
    device.update_started = False
    if owner_authorized is not True:
        raise RuntimeError('Confirm the software update before proceeding.')
    root = Path(root)
    expected = package_build(root)
    if target is not None and expected != target:
        raise RuntimeError('The installer package changed after review. Reconnect and review the current build.')
    device.verify_base()
    if device.installation_state() != 'installed':
        raise RuntimeError('This is not a complete working installation. Reconnect for installation or repair.')
    comparison = compare_build(device, root)
    if comparison['matches']:
        raise RuntimeError('This exact build is already installed. No files were changed.')
    device.token = device.api('/user/login?'+urlencode({'username':'admin','password':password})).get('JWT Token')
    if not device.token:
        raise RuntimeError('Miner dashboard password was not accepted.')
    check = f'test ! -e {RUNTIME}/pending.json && test ! -e {RUNTIME}/network-pending.json && echo clear'
    if device.command(check).strip() != 'clear':
        raise RuntimeError('Wait for the current performance or network adjustment before updating.')
    progress('Saving and verifying your private recovery backup…')
    backup = device.backup(destination)
    config = json.loads((backup/'usr/config/kdbox-xbt/config.json').read_text(encoding='utf-8'))
    if config.get('admin_password') != password or config.get('fallback_password'):
        raise RuntimeError('Saved credentials need inspection before updating. No application files were changed.')
    originals = json.loads((backup/'manifest.json').read_text(encoding='utf-8'))
    protected = protected_hashes(device)
    files = {RUNTIME+'/controller': ((root/'build/kdbox-xbt-controller').read_bytes(), 0o700)}
    files.update({RUNTIME+'/'+name: ((root/'controller'/name).read_bytes(), 0o700) for name in SCRIPTS})
    files.update({WEBROOT+'/'+name: ((root/'controller/web'/name).read_bytes(), 0o644) for name in WEB_ASSETS})
    device.require_space(RUNTIME, sum(len(data) for p,(data,_) in files.items() if p.startswith(RUNTIME+'/'))+1024*1024)
    device.require_space(WEBROOT, sum(len(data) for p,(data,_) in files.items() if p.startswith(WEBROOT+'/'))+1024*1024)
    stages = {}; changed = []; paused = False; pid = None
    try:
        progress('Staging and verifying the new controller and dashboard…')
        for directory in (RUNTIME, WEBROOT):
            stage = device.command('mktemp -d '+directory+'/.kdbox-update.XXXXXX').strip()
            if not re.fullmatch(re.escape(directory)+r'/\.kdbox-update\.[A-Za-z0-9]+', stage):
                raise RuntimeError('Unexpected staging path.')
            stages[directory] = stage
        for path,(data,mode) in files.items():
            device.write(stages[str(PurePosixPath(path).parent)]+'/'+PurePosixPath(path).name, data, mode)
        stage_binary = stages[RUNTIME]+'/controller'
        if 'listen address' not in device.command(shlex.quote(stage_binary)+' -h 2>&1'):
            raise RuntimeError('The staged controller could not be checked.')
        if protected_hashes(device) != protected or device.command(check).strip() != 'clear':
            raise RuntimeError('Settings changed while preparing the update. Reconnect and review them.')
        pid = controller_pid(device)
        if protected[RUNTIME+'/halted.json'] == 'absent':
            record = (json.dumps(dict(reason=UPDATE_PAUSE, at=datetime.now(timezone.utc).isoformat(),
                                     update_id=secrets.token_hex(16)))+'\n').encode()
            pending_stop = stages[RUNTIME]+'/update-stop.json'
            device.write(pending_stop, record)
            device.command('ln '+shlex.quote(pending_stop)+' '+RUNTIME+'/halted.json')
            protected[RUNTIME+'/halted.json'] = hashlib.sha256(record).hexdigest()
        device.update_started = True
        paused = True
        pause_mining(device, pid)
        if protected_hashes(device) != protected or device.command(check).strip() != 'clear':
            raise RuntimeError('Settings changed before the update. Mining remains paused.')
        progress('Updating application files. Mining remains paused for review…')
        for path in sorted(files, key=lambda p:p == RUNTIME+'/controller'):
            changed.append(path)
            device.command('mv '+shlex.quote(stages[str(PurePosixPath(path).parent)]+'/'+PurePosixPath(path).name)+' '+shlex.quote(path))
        device.command('kill -KILL '+pid)
        wait_build(device, expected, protected)
        progress('Update verified. Settings and existing stops were retained; review the dashboard before resuming.')
        return backup
    except Exception:
        if paused:
            # Restore application files only. Never overwrite settings, erase a
            # stop or resume mining after an unsuccessful update.
            for path in sorted(changed, key=lambda p:p != RUNTIME+'/controller'):
                meta = originals[path]
                if meta.get('absent'):
                    device.command('rm -f '+shlex.quote(path))
                else:
                    device.write(path, (backup/path.lstrip('/')).read_bytes(), meta['mode'])
            current = controller_pid(device)
            device.command('kill -KILL '+current)
        raise
    finally:
        for stage in stages.values():
            try:
                names = [PurePosixPath(p).name for p in files]+['update-stop.json']
                device.command('rm -f '+' '.join(shlex.quote(stage+'/'+name+suffix) for name in names for suffix in ('','.kdbox-xbt-new'))+'; rmdir '+shlex.quote(stage))
            except Exception:
                progress('An update staging folder remains for inspection: '+stage)
