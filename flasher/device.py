"""One explicit, supported board adapter. No firmware images or vendor keys."""
import base64
import hashlib
import http.client
import ipaddress
import json
from pathlib import Path
import shlex
import socket
import errno
import time
from urllib.parse import urlencode
import urllib.request
import urllib.error
import paramiko
from .payload import validate_payload, WEB_ASSETS
from .recovery import BACKUP_PATHS, WEB_PATHS, LEGACY_LOGO_SHA256, validate_backup
from . import __version__

PROFILE = json.loads((Path(__file__).resolve().parent.parent / 'controller' / 'compatibility.json').read_text(encoding='utf-8'))

def supported_device(info, allow_missing_board=False):
    return (info.get('model') == PROFILE['model']
            and (info.get('mcbversion') == PROFILE['board']
                 or (allow_missing_board and 'mcbversion' not in info))
            and info.get('firmware') in PROFILE['firmwares'])


def transient_connection_error(exc):
    if isinstance(exc, (paramiko.AuthenticationException, paramiko.BadHostKeyException)):
        return False
    if isinstance(exc, (paramiko.ssh_exception.NoValidConnectionsError, socket.timeout, ConnectionError, EOFError)):
        return True
    return isinstance(exc, paramiko.SSHException) and 'error reading ssh protocol banner' in str(exc).lower()


READ_ONLY_FIRMWARES = {'2.2.3'}

def startup_config(original, firmware):
    updated = original.replace('ttyAMA3::respawn:xminerd', 'ttyAMA3::respawn:/usr/config/paperclip/launcher.sh')
    if 'respawn:/usr/config/paperclip/launcher.sh' not in updated:
        raise RuntimeError('Unexpected startup layout; no changes made')
    if firmware in READ_ONLY_FIRMWARES:
        fan_line = 'ttyAMA3::respawn:fanctrl'
        owned = '# fan control is owned by Paperclip safeguards'
        if fan_line not in updated and owned not in updated:
            raise RuntimeError('Unexpected fan startup layout; no changes made')
        updated = updated.replace(fan_line, owned)
    return updated


class Device:
    def __init__(self, host, ssh_password, confirm_host, web_password=None, progress=lambda text: None):
        try:
            address = ipaddress.ip_address(host.strip())
        except ValueError:
            raise ValueError('Enter the miner IP only, for example 192.168.1.151. Do not include http:// or a port.') from None
        if address.version != 4 or address.is_loopback or address.is_multicast or address.is_unspecified:
            raise ValueError('Enter the miner’s LAN IPv4 address.')
        if not ssh_password:
            raise ValueError('Enter the SSH root password. This is separate from the dashboard password.')
        self.host = host = str(address)
        self.token = None
        self.client = None
        trusted = []
        class Trust(paramiko.MissingHostKeyPolicy):
            def missing_host_key(self, client, hostname, key):
                fingerprint = 'SHA256:' + base64.b64encode(hashlib.sha256(key.asbytes()).digest()).decode().rstrip('=')
                if trusted and trusted[0] != fingerprint:
                    raise RuntimeError('The miner SSH identity changed during reconnect. Stopped; verify the IP and device before trying again.')
                if not trusted:
                    if not confirm_host(hostname, fingerprint):
                        raise RuntimeError('Connection canceled: miner identity was not confirmed.')
                    trusted.append(fingerprint)
        enabled = False
        for attempt in range(4):
            client = paramiko.SSHClient()
            client.set_missing_host_key_policy(Trust())
            progress(f'Connecting to {host}:22 — attempt {attempt + 1} of 4…')
            try:
                client.connect(host, username='root', password=ssh_password, timeout=10,
                               banner_timeout=30, auth_timeout=15, look_for_keys=False, allow_agent=False)
                client.get_transport().set_keepalive(15)
                self.client = client
                return
            except paramiko.AuthenticationException:
                client.close()
                raise RuntimeError('SSH password was rejected. Check the SSH root password for this firmware; the dashboard password is different. No installation files were changed.') from None
            except Exception as exc:
                client.close()
                if not transient_connection_error(exc):
                    raise
                if isinstance(exc, paramiko.ssh_exception.NoValidConnectionsError) and not enabled:
                    refused = any(getattr(e, 'errno', None) in (errno.ECONNREFUSED, 10061) for e in exc.errors.values())
                    if refused:
                        if not web_password:
                            raise RuntimeError('SSH is not accepting connections. Enter the dashboard password so the supported miner can enable SSH.') from None
                        progress('Checking the model and enabling its existing SSH service…')
                        self.enable_ssh(host, web_password)
                        enabled = True
                if attempt == 3:
                    raise RuntimeError('The miner did not complete its SSH connection after four attempts. Confirm the IP opens its dashboard, let the miner finish booting, and retry. If needed, power-cycle it once. Do not upgrade firmware to fix this. No installation files were changed.') from None
                progress('Miner not ready yet; retrying the connection…')
                time.sleep(2 ** attempt)

    @staticmethod
    def enable_ssh(host, password):
        address = ipaddress.ip_address(host)
        if address.version != 4 or not address.is_private or address.is_loopback:
            raise ValueError('Automatic SSH setup requires the miner’s private LAN IPv4 address')
        # The stock API is HTTP-only. Refuse redirects and never use system proxies.
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, headers, newurl):return None
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        base = 'http://' + host
        def request(path, data=None, token=None):
            headers = {'Content-Type':'application/json'}
            if token:headers['Authorization']='Bearer '+token
            req=urllib.request.Request(base+path, data=data, headers=headers,
                                       method='GET' if data is None else 'PUT')
            with opener.open(req,timeout=8) as response:
                body=response.read(1024*1024)
                return json.loads(body) if body.strip() else None
        info=request('/mcb/status')
        # Older APIs omit the board field. Only enable access here; inspect()
        # must verify the local board identity over SSH before installation.
        if not supported_device(info, allow_missing_board=True):
            raise RuntimeError('Unsupported device; SSH settings were not changed')
        try:
            auth=request('/user/login?'+urlencode({'username':'admin','password':password}))
            token=auth.get('JWT Token')
            if not token:raise ValueError('login rejected')
            if info['firmware'] in READ_ONLY_FIRMWARES:
                # These stock versions make /mcb/protect a no-op. Their service
                # login starts SSH but returns 500 because it is not a user login.
                # Require the owner's normal login above before using that hook.
                try:
                    request('/user/login?'+urlencode({'username':'admin','password':'^L3VR!t@H4yHX]HA_1Ve'}))
                except urllib.error.HTTPError as exc:
                    if exc.code != 500: raise
            else:
                request('/mcb/protect', b'{}', token)
        except Exception:
            raise RuntimeError('Could not enable SSH. Check the dashboard password and LAN connection.') from None

    def close(self):
        if self.client:
            self.client.close()

    def command(self, command):
        _, out, err = self.client.exec_command(command, timeout=20)
        data = out.read().decode('utf-8', 'replace')
        if out.channel.recv_exit_status():
            raise RuntimeError(err.read().decode('utf-8', 'replace') or 'Device command failed')
        return data

    def api(self, path, data=None):
        # Only these read-only queries may be repeated; login and PUT can mutate state.
        attempts = 3 if data is None and path in ('/mcb/status', '/mcb/algosetting', '/api/status') else 1
        for attempt in range(attempts):
            try:
                return self._api_once(path, data)
            except (socket.timeout, ConnectionError, EOFError, http.client.HTTPException):
                if attempt + 1 == attempts:
                    raise RuntimeError('The miner connection was interrupted. Read-only checks can be retried; settings and installation commands were not automatically repeated. Check the miner before retrying.') from None
                time.sleep(attempt + 1)

    def _api_once(self, path, data=None):
        # HTTP remains inside encrypted SSH and targets only the local native API.
        port = 8088 if path == '/api/status' else 80
        channel = self.client.get_transport().open_channel('direct-tcpip', ('127.0.0.1', port), ('127.0.0.1', 0), timeout=10)
        channel.settimeout(10)
        conn = http.client.HTTPConnection('127.0.0.1', timeout=10)
        conn.sock = channel
        headers = {'Content-Type': 'application/json'}
        if self.token:
            headers['Authorization'] = 'Bearer ' + self.token
        try:
            conn.request('GET' if data is None else 'PUT', path,
                         None if data is None else json.dumps(data), headers)
            res = conn.getresponse()
            body = res.read(1024 * 1024)
            if res.status != 200:
                raise RuntimeError(f'Device API returned HTTP {res.status}')
            return json.loads(body) if body.strip() else None
        finally:
            conn.close()

    def inspect(self):
        info = self.api('/mcb/status')
        if (info.get('model') == PROFILE['model']
                and info.get('firmware') in PROFILE['firmwares']
                and 'mcbversion' not in info):
            info['mcbversion'] = self.command('cat /root/mcbversion').strip()
        if not supported_device(info):
            raise RuntimeError('Unsupported KDBox firmware or board. See the firmware compatibility table. No firmware upgrade will be attempted.')
        if self.command('uname -m').strip() != PROFILE['architecture']:
            raise RuntimeError('Unsupported CPU architecture')
        return info

    def write(self, remote, data, mode=0o600):
        with self.client.open_sftp() as s:
            with s.open(remote + '.paperclip-new', 'wb') as f:
                f.write(data)
            s.chmod(remote + '.paperclip-new', mode)
        expected = hashlib.sha256(data).hexdigest()
        actual = self.command('sha256sum ' + shlex.quote(remote + '.paperclip-new')).split()
        if not actual or actual[0] != expected:
            raise RuntimeError('File transfer verification failed. The destination file was not replaced; do not restart the miner.')
        self.command('mv ' + shlex.quote(remote + '.paperclip-new') + ' ' + shlex.quote(remote))

    def require_space(self, path, required_bytes):
        try:
            available = int(self.command('df -Pk ' + shlex.quote(path)).splitlines()[-1].split()[3]) * 1024
        except (ValueError, IndexError):
            raise RuntimeError('Could not verify free space on ' + path) from None
        if available < required_bytes:
            raise RuntimeError('Not enough free space on ' + path + '. No installation files were changed.')

    def backup(self, destination):
        firmware = self.inspect()['firmware']
        destination = Path(destination)
        destination.mkdir(mode=0o700, parents=True, exist_ok=False)
        destination.chmod(0o700)
        paths = BACKUP_PATHS
        manifest = {}
        with self.client.open_sftp() as s:
            for remote in paths:
                try:
                    info = s.stat(remote)
                except FileNotFoundError:
                    manifest[remote] = {'absent': True}
                    continue
                local = destination / remote.lstrip('/')
                local.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
                s.get(remote, str(local))
                local.chmod(0o600)
                digest = hashlib.sha256(local.read_bytes()).hexdigest()
                if self.command('sha256sum ' + shlex.quote(remote)).split()[0] != digest:
                    raise RuntimeError('Backup verification failed: the source changed or the transfer was incomplete. No installation files were changed. Retry after the miner is idle.')
                manifest[remote] = {'sha256': digest, 'mode': info.st_mode & 0o777}
        (destination / 'manifest.json').write_text(json.dumps(manifest, indent=2), encoding='utf-8')
        (destination / 'manifest.json').chmod(0o600)
        identity={'manifest_version': 2, 'software_version': __version__, 'firmware':firmware, 'mac':self.command('cat /sys/class/net/eth0/address').strip(), 'host_key':base64.b64encode(self.client.get_transport().get_remote_server_key().asbytes()).decode()}
        (destination / 'device.json').write_text(json.dumps(identity),encoding='utf-8')
        (destination / 'device.json').chmod(0o600)
        return destination

    def restore(self, destination):
        destination=Path(destination)
        info = self.inspect()
        identity=json.loads((destination/'device.json').read_text(encoding='utf-8'))
        # Backups from 0.1.0 predate this field and only supported firmware 2.2.1.
        if identity.get('firmware', '2.2.1') != info['firmware']:
            raise RuntimeError('This backup was made on different firmware. Restore the matching stock firmware first.')
        if identity['mac']!=self.command('cat /sys/class/net/eth0/address').strip() or identity['host_key']!=base64.b64encode(self.client.get_transport().get_remote_server_key().asbytes()).decode():
            raise RuntimeError('This backup belongs to another miner')
        manifest=json.loads((destination/'manifest.json').read_text(encoding='utf-8'))
        manifest_version = validate_backup(destination, identity, manifest)
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('mount -o remount,rw /')
        # Restore boot configuration last. Do not restart until all files are written.
        for remote,meta in sorted(manifest.items(),key=lambda item:item[0]=='/etc/inittab'):
            if not meta.get('absent'):
                self.write(remote,(destination/remote.lstrip('/')).read_bytes(),meta['mode'])
            elif manifest_version == 2 and remote in WEB_PATHS:
                # Only remove allowlisted web files proven absent in this snapshot.
                self.command('rm -f ' + shlex.quote(remote))
        if manifest.get('/etc/init.d/S99paperclip',{}).get('absent'):
            self.command('if test -e /etc/init.d/S99paperclip; then mv /etc/init.d/S99paperclip /usr/config/paperclip/disabled-startup-hook; fi')
        self.command('sync; reboot')

    def retire_upstream_logo(self, backup):
        # Never delete an arbitrary/custom logo. Its original bytes are retained
        # in the verified recovery snapshot before a known upstream file is removed.
        saved = Path(backup) / 'root/yottamc/paperclip.svg'
        if saved.is_file() and hashlib.sha256(saved.read_bytes()).hexdigest() == LEGACY_LOGO_SHA256:
            remote = '/root/yottamc/paperclip.svg'
            self.command('if test -f ' + remote + '; then '
                         'set -- $(sha256sum ' + remote + '); '
                         'if test "$1" = ' + shlex.quote(LEGACY_LOGO_SHA256) + '; then rm ' + remote + '; fi; fi')

    def install(self, root, password, backup_dir, progress=print):
        self.installation_started = False
        try:
            return self._install(root, password, backup_dir, progress)
        except Exception as exc:
            if self.installation_started:
                raise RuntimeError('Installation stopped after changes began. Do not reboot or blindly repeat installation. Recovery backup: ' + str(backup_dir) + '. Check the miner dashboard and use Restore a recovery backup if recovery is needed. Details: ' + str(exc)) from None
            raise

    def _install(self, root, password, backup_dir, progress=print):
        root = Path(root)
        info = self.inspect()
        if self.command('test -f /usr/config/paperclip/halted.json && echo halted; true').strip() == 'halted':
            raise RuntimeError('The miner has an active safety stop. Inspect its cooling and resolve the stop in the dashboard before installing. No installation files were changed.')
        binary = validate_payload(root)
        # Atomic replacement needs room for the new binary beside the old one.
        self.require_space('/usr/config', len(binary) + 1024 * 1024)
        self.require_space('/root', sum((root / 'controller/web' / name).stat().st_size for name in WEB_ASSETS) + 1024 * 1024)
        self.token = self.api('/user/login?' + urlencode({'username': 'admin', 'password': password})).get('JWT Token')
        if not self.token:
            raise RuntimeError('Miner dashboard password was not accepted')
        if self.command('test ! -f /usr/config/paperclip/pending.json && echo clear').strip() != 'clear':
            raise RuntimeError('Finish the active tuning trial first')
        progress('Saving a private recovery backup…')
        backup = self.backup(backup_dir)

        init = startup_config((backup / 'etc/inittab').read_text(encoding='utf-8'), info['firmware'])
        self.installation_started = True
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('mount -o remount,rw /')
        progress('Installing controller and original-port dashboard…')
        self.command('mkdir -p /usr/config/paperclip; chmod 700 /usr/config/paperclip')
        self.write('/usr/config/paperclip/controller', binary, 0o700)
        expected = hashlib.sha256(binary).hexdigest()
        if self.command('sha256sum /usr/config/paperclip/controller').split()[0] != expected:
            raise RuntimeError('Controller transfer verification failed')
        for name in ['launcher.sh', 'supervisor.sh', 'boot.sh']:
            self.write('/usr/config/paperclip/' + name, (root / 'controller' / name).read_bytes(), 0o700)
        self.write('/usr/config/paperclip/config.json', json.dumps({'admin_password': password}).encode())
        approved = {'clock': PROFILE.get('firmware_settings', {}).get(info['firmware'], {}).get('default_clock', 740), 'millivolts': 430, 'fan': 100, 'mode': 'fixed'}
        if not (backup / 'usr/config/paperclip/approved.json').exists():
            self.write('/usr/config/paperclip/approved.json', json.dumps(approved).encode())
        if self.command('test -f /root/yottamc/legacy.html && echo exists; true').strip() != 'exists':
            self.write('/root/yottamc/legacy.html', (backup / 'root/yottamc/index.html').read_bytes(), 0o644)
        for name in WEB_ASSETS:
            self.write('/root/yottamc/' + name, (root / 'controller/web' / name).read_bytes(), 0o644)
        self.retire_upstream_logo(backup)
        menu = self.api('/mcb/algosetting')
        menu['algos'] = [{'name': 'blake2s(KDA)', 'id': 0}, {'name': 'blake2b(SC)', 'id': 1}]
        for name in ['/root/settings/algosetting.json', '/usr/config/minerd/settings/algosetting.json', '/usr/config/minerd/settings/.algosetting.json']:
            self.write(name, json.dumps(menu).encode(), 0o644)
        self.api('/mcb/algosetting', menu)
        self.write('/etc/inittab', init.encode(), 0o644)
        self.write('/etc/init.d/S99paperclip', b'#!/bin/sh\ncase "$1" in start) /usr/config/paperclip/boot.sh </dev/null >/tmp/paperclip-start.log 2>&1 & ;; esac\n', 0o755)
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('echo 100 >/sys/class/fans/fan0/lvl; echo 100 >/sys/class/fans/fan1/lvl; for p in $(pidof fanctrl); do kill -STOP "$p"; done')
        self.command('if pidof controller >/dev/null; then kill -TERM $(pidof controller); else nohup /usr/config/paperclip/boot.sh </dev/null >/tmp/paperclip-start.log 2>&1 & fi; echo started')
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('sync; mount -o remount,ro /')
        progress('Checking that the new dashboard is responding…')
        ready = False
        for attempt in range(10):
            try:
                state = self._api_once('/api/status')
                if isinstance(state, dict) and {'device', 'approved', 'stale', 'halted'} <= state.keys() and not state['stale']:
                    ready = True
                    break
            except (OSError, paramiko.SSHException, http.client.HTTPException, ValueError, RuntimeError):
                pass
            time.sleep(2)
        if not ready:
            raise RuntimeError('Files were installed, but dashboard startup could not be confirmed. Mining has not been verified.')
        progress('Installed. Configure mining pools at http://' + self.host + '/ . Recovery backup: ' + str(backup))
        return backup
