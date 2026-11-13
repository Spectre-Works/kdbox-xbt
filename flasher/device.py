"""One explicit, supported board adapter. No firmware images or vendor keys."""
import base64
import hashlib
import http.client
import ipaddress
import json
import os
from pathlib import Path
import shlex
import socket
import errno
import time
from urllib.parse import urlencode
import paramiko
from .payload import validate_payload, WEB_ASSETS
from .recovery import ALGORITHM_PATH, BACKUP_PATHS, BACKUP_SCHEMA, WEB_PATHS, LEGACY_FAVICON, validate_backup
from . import __version__
from .branding import PRODUCT

PROFILE = json.loads((Path(__file__).resolve().parent.parent / 'controller' / 'compatibility.json').read_text(encoding='utf-8'))

def supported_device(info):
    return (info.get('model') == PROFILE['model']
            and info.get('mcbversion') == PROFILE['board']
            and info.get('firmware') in PROFILE['firmwares'])


def transient_connection_error(exc):
    if isinstance(exc, (paramiko.AuthenticationException, paramiko.BadHostKeyException)):
        return False
    if isinstance(exc, (paramiko.ssh_exception.NoValidConnectionsError, socket.timeout, ConnectionError, EOFError)):
        return True
    return isinstance(exc, paramiko.SSHException) and 'error reading ssh protocol banner' in str(exc).lower()


READ_ONLY_FIRMWARES = {'2.2.3'}

def startup_config(original, firmware):
    updated = original.replace('ttyAMA3::respawn:xminerd', 'ttyAMA3::respawn:/usr/config/kdbox-xbt/launcher.sh')
    if 'respawn:/usr/config/kdbox-xbt/launcher.sh' not in updated:
        raise RuntimeError('Unexpected startup layout; no changes made')
    if firmware in READ_ONLY_FIRMWARES:
        fan_line = 'ttyAMA3::respawn:fanctrl'
        owned = '# fan control is owned by KD BOX XBT safeguards'
        if fan_line not in updated and owned not in updated:
            raise RuntimeError('Unexpected fan startup layout; no changes made')
        updated = updated.replace(fan_line, owned)
    return updated


def installation_algorithm(backup, native_menu):
    """Default new miners to XBT; retain an existing owner's explicit choice."""
    if (not isinstance(native_menu, dict) or type(native_menu.get('algo_select')) is not int
            or native_menu['algo_select'] not in (0, 1)):
        raise RuntimeError('Stock firmware returned an invalid algorithm selection. No application files were changed.')
    path = Path(backup) / ALGORITHM_PATH.lstrip('/')
    if path.exists():
        chosen = json.loads(path.read_text(encoding='utf-8'))
    elif (Path(backup) / 'usr/config/kdbox-xbt/config.json').exists():
        chosen = native_menu.get('algo_select')
    else:
        chosen = 1  # Bitcoin (XBT); native firmware calls this blake2b(SC).
    if type(chosen) is not int or chosen not in (0, 1):
        raise RuntimeError('Saved or native algorithm selection is invalid. No application files were changed.')
    return chosen


class Device:
    def __init__(self, host, ssh_password, confirm_host, progress=lambda text: None, *, web_password=None):
        try:
            address = ipaddress.ip_address(host.strip())
        except ValueError:
            raise ValueError('Enter the miner IP only, for example 192.168.1.151. Do not include http:// or a port.') from None
        if address.version != 4 or not address.is_private or address.is_loopback or address.is_unspecified:
            raise ValueError('Enter the miner’s private LAN IPv4 address.')
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
                if not enabled and isinstance(exc, paramiko.ssh_exception.NoValidConnectionsError) and any(
                    getattr(error, 'errno', None) in (errno.ECONNREFUSED, 10061) for error in exc.errors.values()
                ):
                    if not web_password:
                        raise RuntimeError('The miner refused SSH on port 22. Supply the dashboard password to start SSH on stock 2.2.1.') from None
                    progress('Verifying the stock version and starting SSH through dashboard maintenance…')
                    self.enable_ssh(web_password)
                    enabled = True
                if attempt == 3:
                    raise RuntimeError('The miner did not complete its SSH connection after four attempts. Confirm the IP opens its dashboard, let the miner finish booting, and retry. If needed, power-cycle it once. Do not upgrade firmware to fix this. No installation files were changed.') from None
                progress('Miner not ready yet; retrying the connection…')
                time.sleep(2 ** attempt)

    def enable_ssh(self, password):
        """Use the stock owner's dashboard login and maintenance endpoint."""
        def request(method, path, body=None, token=None):
            headers = {'Content-Type': 'application/json'}
            if token:
                headers['Authorization'] = 'Bearer ' + token
            conn = http.client.HTTPConnection(self.host, timeout=8)
            try:
                conn.request(method, path, body=body, headers=headers)
                response = conn.getresponse()
                if response.status != 200:
                    raise RuntimeError(f'Stock dashboard returned HTTP {response.status}.')
                data = response.read(1024 * 1024)
                return json.loads(data) if data.strip() else None
            finally:
                conn.close()

        try:
            info = request('GET', '/mcb/status')
            if not supported_device(info or {}) or info['firmware'] != PROFILE['install_base']['version']:
                raise RuntimeError('Remote SSH setup requires the original KD BOX / MCB_V5_4 on stock 2.2.1.')
            query = urlencode({'username': 'admin', 'password': password})
            auth = request('GET', '/user/login?' + query)
            token = (auth or {}).get('JWT Token')
            if not token:
                raise RuntimeError('Dashboard login was rejected before SSH setup.')
            request('PUT', '/mcb/protect', '{}', token)
        except (OSError, ValueError, http.client.HTTPException):
            raise RuntimeError('Dashboard maintenance could not start SSH. Check LAN reachability and the dashboard password.') from None

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

    def verify_base(self):
        info = self.inspect()
        base = PROFILE['install_base']
        if info['firmware'] != base['version']:
            raise RuntimeError(f"New installations require stock {base['version']}.")
        version = self.command('cat /root/version').strip()
        digest = self.command('sha256sum /usr/bin/minerd').split()
        if version != base['version'] or not digest or digest[0] != base['minerd_sha256']:
            raise RuntimeError(f"The miner does not match the stock {base['version']} management build.")
        return info

    def installation_state(self):
        """Distinguish a working mod from files left by an interrupted install/reflash."""
        runtime = '/usr/config/kdbox-xbt'
        checks = {'directory': f'test -d {runtime}',
                  'config': f'test -s {runtime}/config.json',
                  'controller': f'test -x {runtime}/controller',
                  'startup': "grep -Fq 'ttyAMA3::respawn:/usr/config/kdbox-xbt/launcher.sh' /etc/inittab",
                  'hook': "grep -Fq '/usr/config/kdbox-xbt/boot.sh' /etc/init.d/S99kdbox-xbt",
                  'dashboard': "grep -Fq 'KD BOX Control' /root/yottamc/index.html",
                  'previous-favicon': 'test -s /root/yottamc/favicon.svg'}
        checks.update({name: f'test -x {runtime}/{name}' for name in ('launcher.sh', 'supervisor.sh', 'boot.sh')})
        checks.update({name: 'test -s ' + shlex.quote('/root/yottamc/' + name) for name in WEB_ASSETS})
        present = set(self.command('\n'.join(
            f'if {check}; then echo {shlex.quote(name)}; fi' for name, check in checks.items()) + '\ntrue').splitlines())
        # Stock firmware also has index.html and may share generic asset names.
        # Require a mod-specific marker before treating any incomplete files as
        # recovery evidence; the full asset list below checks completeness only.
        markers = {'directory', 'config', 'controller', 'startup', 'hook', 'dashboard', 'spectre-logo.png'}
        if not present.intersection(markers):
            return 'new'
        icon_names = {'favicon-32.png', 'favicon-48.png'}
        required = set(checks) - icon_names - {'previous-favicon'}
        if not required.issubset(present) or not (icon_names.issubset(present) or 'previous-favicon' in present):
            return 'repair'
        try:
            state = self.api('/api/status')
        except (OSError, RuntimeError, ValueError, paramiko.SSHException, http.client.HTTPException):
            return 'repair'
        # A safety-stopped or calibrating controller is still installed. Fresh
        # mining telemetry is deliberately not required for this identity check.
        if (isinstance(state, dict) and isinstance(state.get('software'), dict)
                and state['software'].get('name') == PRODUCT and state['software'].get('version')
                and isinstance(state.get('approved'), dict)
                and type(state.get('halted')) is bool and type(state.get('stale')) is bool):
            return 'installed'
        return 'repair'

    def write(self, remote, data, mode=0o600):
        with self.client.open_sftp() as s:
            with s.open(remote + '.kdbox-xbt-new', 'wb') as f:
                f.write(data)
            s.chmod(remote + '.kdbox-xbt-new', mode)
        expected = hashlib.sha256(data).hexdigest()
        actual = self.command('sha256sum ' + shlex.quote(remote + '.kdbox-xbt-new')).split()
        if not actual or actual[0] != expected:
            raise RuntimeError('File transfer verification failed. The destination file was not replaced; do not restart the miner.')
        self.command('mv ' + shlex.quote(remote + '.kdbox-xbt-new') + ' ' + shlex.quote(remote))

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
        if os.name != 'nt' and destination.stat().st_mode & 0o077:
            destination.rmdir()
            raise RuntimeError('The backup location did not enforce private permissions. No backup data was copied.')
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
                parent = local.parent
                while parent != destination:
                    parent.chmod(0o700)
                    parent = parent.parent
                s.get(remote, str(local))
                local.chmod(0o600)
                digest = hashlib.sha256(local.read_bytes()).hexdigest()
                if self.command('sha256sum ' + shlex.quote(remote)).split()[0] != digest:
                    raise RuntimeError('Backup verification failed: the source changed or the transfer was incomplete. No installation files were changed. Retry after the miner is idle.')
                manifest[remote] = {'sha256': digest, 'mode': info.st_mode & 0o777}
        identity={'manifest_version': BACKUP_SCHEMA, 'software_version': __version__, 'firmware':firmware, 'mac':self.command('cat /sys/class/net/eth0/address').strip(), 'host_key':base64.b64encode(self.client.get_transport().get_remote_server_key().asbytes()).decode()}
        validate_backup(destination, identity, manifest)
        (destination / 'manifest.json').write_text(json.dumps(manifest, indent=2), encoding='utf-8')
        (destination / 'manifest.json').chmod(0o600)
        (destination / 'device.json').write_text(json.dumps(identity),encoding='utf-8')
        (destination / 'device.json').chmod(0o600)
        if os.name != 'nt' and any(path.stat().st_mode & 0o077
                                  for path in (destination, *destination.rglob('*'))):
            raise RuntimeError('Recovery backup permissions are not private. No application files were changed. Choose a private local folder with KDBOX_XBT_BACKUP_DIR; see the host setup guide.')
        return destination

    def restore(self, destination):
        destination=Path(destination)
        info = self.inspect()
        identity=json.loads((destination/'device.json').read_text(encoding='utf-8'))
        if identity.get('firmware') != info['firmware']:
            raise RuntimeError('This backup was made on different firmware. Restore the matching stock firmware first.')
        if identity['mac']!=self.command('cat /sys/class/net/eth0/address').strip() or identity['host_key']!=base64.b64encode(self.client.get_transport().get_remote_server_key().asbytes()).decode():
            raise RuntimeError('This backup belongs to another miner')
        manifest=json.loads((destination/'manifest.json').read_text(encoding='utf-8'))
        validate_backup(destination, identity, manifest)
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('mount -o remount,rw /')
        # Restore boot configuration last. Do not restart until all files are written.
        for remote,meta in sorted(manifest.items(),key=lambda item:item[0]=='/etc/inittab'):
            if not meta.get('absent'):
                self.write(remote,(destination/remote.lstrip('/')).read_bytes(),meta['mode'])
            elif remote == ALGORITHM_PATH or remote in WEB_PATHS:
                # Remove only files proven absent in this snapshot.
                self.command('rm -f ' + shlex.quote(remote))
        if manifest.get('/etc/init.d/S99kdbox-xbt',{}).get('absent'):
            self.command('if test -e /etc/init.d/S99kdbox-xbt; then mv /etc/init.d/S99kdbox-xbt /usr/config/kdbox-xbt/disabled-startup-hook; fi')
        self.command('sync; reboot')

    def install(self, root, password, backup_dir, progress=print, *, owner_authorized=False, repair_authorized=False):
        self.installation_started = False
        try:
            return self._install(root, password, backup_dir, progress, owner_authorized, repair_authorized)
        except Exception as exc:
            if self.installation_started:
                raise RuntimeError('Installation stopped after changes began. Do not reboot or blindly repeat installation. Recovery backup: ' + str(backup_dir) + '. Check the miner dashboard and use Restore a recovery backup if recovery is needed. Details: ' + str(exc)) from None
            raise

    def _install(self, root, password, backup_dir, progress, owner_authorized, repair_authorized=False):
        root = Path(root)
        info = self.verify_base()
        progress(f"Verified stock {info['firmware']} on {info['model']} / {info['mcbversion']} over SSH.")
        if self.command('test -f /usr/config/kdbox-xbt/halted.json && echo halted; true').strip() == 'halted':
            raise RuntimeError('The miner has an active safety stop. Inspect its cooling and resolve the stop in the dashboard before installing. No installation files were changed.')
        if owner_authorized is not True:
            raise RuntimeError('Owner authorization and confirmation of power, cooling and recovery access are required.')
        if not password:
            raise RuntimeError('Enter the miner’s current dashboard password.')
        installation = self.installation_state()
        if installation == 'installed':
            raise RuntimeError('KD BOX XBT is already running. Open its dashboard; no files were changed.')
        if installation == 'repair' and not repair_authorized:
            raise RuntimeError('Previous installation files need repair. Reconnect and explicitly confirm repair; no files were changed.')
        binary = validate_payload(root)
        # Atomic replacement needs room for the new binary beside the old one.
        self.require_space('/usr/config', len(binary) + 1024 * 1024)
        self.require_space('/root', sum((root / 'controller/web' / name).stat().st_size for name in WEB_ASSETS) + 1024 * 1024)
        self.token = self.api('/user/login?' + urlencode({'username': 'admin', 'password': password})).get('JWT Token')
        if not self.token:
            raise RuntimeError('Miner dashboard password was not accepted')
        if self.command('test ! -f /usr/config/kdbox-xbt/pending.json && test ! -f /usr/config/kdbox-xbt/network-pending.json && echo clear').strip() != 'clear':
            raise RuntimeError('Resolve the pending performance or network adjustment before installing or repairing. Recovery journals were not cleared.')
        progress('Saving a private recovery backup…')
        backup = self.backup(backup_dir)

        init = startup_config((backup / 'etc/inittab').read_text(encoding='utf-8'), info['firmware'])
        saved_config = backup / 'usr/config/kdbox-xbt/config.json'
        if saved_config.exists():
            config = json.loads(saved_config.read_text(encoding='utf-8'))
            if (not isinstance(config, dict) or config.get('admin_password') != password
                    or config.get('fallback_password')):
                raise RuntimeError('Saved mod credentials need inspection. The current dashboard password must match the saved mod login before repair. No application files were changed.')
        menu = self.api('/mcb/algosetting')
        chosen_algorithm = installation_algorithm(backup, menu)
        self.installation_started = True
        if menu['algo_select'] == 0 and chosen_algorithm == 1:
            progress('Clearing the backed-up KDA pools before selecting Bitcoin (XBT)…')
            pools = self.api('/mcb/pools')
            if not isinstance(pools, list) or any(not isinstance(pool, dict) for pool in pools):
                raise RuntimeError('Could not read the KDA pool list. XBT was not applied.')
            while pools:
                self.api('/mcb/delpool', pools[0])
                remaining = self.api('/mcb/pools')
                if (not isinstance(remaining, list) or any(not isinstance(pool, dict) for pool in remaining)
                        or len(remaining) >= len(pools)):
                    raise RuntimeError('Stock firmware did not confirm clearing the KDA pools. XBT was not applied; inspect the miner and recovery backup before continuing.')
                pools = remaining
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('mount -o remount,rw /')
        progress('Installing controller and original-port dashboard…')
        self.command('mkdir -p /usr/config/kdbox-xbt; chmod 700 /usr/config/kdbox-xbt')
        self.write('/usr/config/kdbox-xbt/controller', binary, 0o700)
        expected = hashlib.sha256(binary).hexdigest()
        if self.command('sha256sum /usr/config/kdbox-xbt/controller').split()[0] != expected:
            raise RuntimeError('Controller transfer verification failed')
        for name in ['launcher.sh', 'supervisor.sh', 'boot.sh']:
            self.write('/usr/config/kdbox-xbt/' + name, (root / 'controller' / name).read_bytes(), 0o700)
        if not saved_config.exists():
            self.write('/usr/config/kdbox-xbt/config.json', json.dumps({'admin_password': password}).encode())
        approved = {'clock': PROFILE.get('firmware_settings', {}).get(info['firmware'], {}).get('default_clock', 740), 'millivolts': 430}
        if not (backup / 'usr/config/kdbox-xbt/approved.json').exists():
            self.write('/usr/config/kdbox-xbt/approved.json', json.dumps(approved).encode())
        if not (backup / ALGORITHM_PATH.lstrip('/')).exists():
            self.write(ALGORITHM_PATH, json.dumps(chosen_algorithm).encode())
        if self.command('test -f /root/yottamc/legacy.html && echo exists; true').strip() != 'exists':
            self.write('/root/yottamc/legacy.html', (backup / 'root/yottamc/index.html').read_bytes(), 0o644)
        for name in WEB_ASSETS:
            self.write('/root/yottamc/' + name, (root / 'controller/web' / name).read_bytes(), 0o644)
        self.command('rm -f ' + shlex.quote(LEGACY_FAVICON))
        menu['algos'] = [{'name': 'blake2s(KDA)', 'id': 0}, {'name': 'blake2b(SC)', 'id': 1}]
        menu['algo_select'] = chosen_algorithm
        for name in ['/root/settings/algosetting.json', '/usr/config/minerd/settings/algosetting.json', '/usr/config/minerd/settings/.algosetting.json']:
            self.write(name, json.dumps(menu).encode(), 0o644)
        self.api('/mcb/algosetting', menu)
        observed = self.api('/mcb/algosetting')
        if not isinstance(observed, dict) or type(observed.get('algo_select')) is not int or observed['algo_select'] != chosen_algorithm:
            raise RuntimeError('Stock firmware did not confirm the selected algorithm. Application startup was not started; inspect the miner and recovery backup before continuing.')
        self.write('/etc/inittab', init.encode(), 0o644)
        self.write('/etc/init.d/S99kdbox-xbt', b'#!/bin/sh\ncase "$1" in start) /usr/config/kdbox-xbt/boot.sh </dev/null >/tmp/kdbox-xbt-start.log 2>&1 & ;; esac\n', 0o755)
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('echo 100 >/sys/class/fans/fan0/lvl; echo 100 >/sys/class/fans/fan1/lvl; for p in $(pidof fanctrl); do kill -STOP "$p"; done')
        self.command('if pidof controller >/dev/null; then kill -TERM $(pidof controller); else nohup /usr/config/kdbox-xbt/boot.sh </dev/null >/tmp/kdbox-xbt-start.log 2>&1 & fi; echo started')
        if info['firmware'] in READ_ONLY_FIRMWARES:
            self.command('sync; mount -o remount,ro /')
        progress('Checking that the new dashboard is responding…')
        ready = False
        for attempt in range(30):
            try:
                state = self._api_once('/api/status')
                if (isinstance(state, dict) and isinstance(state.get('software'), dict)
                        and state['software'].get('name') == PRODUCT
                        and state['software'].get('version') == __version__
                        and state.get('algorithm') == chosen_algorithm and not state.get('algorithm_queued')
                        and isinstance(state.get('approved'), dict)
                        and type(state.get('stale')) is bool and type(state.get('halted')) is bool):
                    ready = True
                    break
            except (OSError, paramiko.SSHException, http.client.HTTPException, ValueError, RuntimeError):
                pass
            time.sleep(2)
        if not ready:
            raise RuntimeError('Files were installed, but dashboard startup could not be confirmed. Mining has not been verified.')
        progress('Installed. Configure mining pools at http://' + self.host + '/ . Recovery backup: ' + str(backup))
        return backup
