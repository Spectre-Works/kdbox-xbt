"""Local browser installer. Credentials remain in memory; no telemetry service."""
from datetime import datetime
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import secrets
import threading
from urllib.parse import urlencode, urlsplit

from . import __version__
from .browser import open_installer_browser
from .branding import backup_root, SOURCE_URL
from .device import Device
from .defaults import DEFAULT_SSH_PASSWORD
from .discovery import discover, local_networks, probe, private_address, FIRMWARE_URL
from .messages import error_message
from .payload import validate_payload, WEB_ASSETS


class Wizard:
    def __init__(self, root=None, device_factory=Device):
        self.root = Path(root or Path(__file__).resolve().parents[1])
        self.factory = device_factory
        self.token = secrets.token_urlsafe(32)
        self.lock = threading.RLock()
        self.answer = threading.Event()
        self.accept_identity = False
        self.device = None
        self.passwords = []
        self.backups = {}
        self.state = dict(stage='welcome', busy=False, message='Ready to find your KD BOX.', devices=[],
                          networks=local_networks(), version=__version__, issues=SOURCE_URL + '/issues',
                          firmware_url=FIRMWARE_URL, backups=[])
        try:
            validate_payload(self.root)
            self.state['payload_ready'] = True
        except (OSError, ValueError, RuntimeError):
            self.state['payload_ready'] = False
        self.refresh_backups()

    def refresh_backups(self):
        backups, choices = {}, []
        root_path = backup_root()
        if root_path.is_dir():
            for directory in sorted(root_path.iterdir(), reverse=True):
                if not directory.is_dir() or directory.is_symlink():
                    continue
                try:
                    metadata = directory / 'device.json'
                    if metadata.is_symlink() or metadata.stat().st_size > 16384:
                        continue
                    info = json.loads(metadata.read_text(encoding='utf-8'))
                    if not isinstance(info, dict):
                        continue
                    key = secrets.token_urlsafe(12)
                    backups[key] = directory
                    choices.append({'id': key, 'label': directory.name + ' · ' + str(info.get('mac', 'Unknown device'))})
                except (OSError, ValueError):
                    continue
        with self.lock:
            self.backups = backups
            self.state['backups'] = choices

    def update(self, **values):
        with self.lock:
            self.state.update(values)

    def snapshot(self):
        with self.lock:
            return json.loads(json.dumps(self.state))

    def progress(self, message):
        self.update(message=error_message(message, *self.passwords))

    def start(self, operation):
        with self.lock:
            if self.state['busy']:
                raise ValueError('An operation is already running. Please wait.')
            self.state.update(busy=True, error='')
        def work():
            try:
                operation()
            except Exception as exc:
                self.update(stage='error', error=error_message(exc, *self.passwords), message='Stopped. Review the details before continuing.')
                if self.device:
                    self.device.close()
                    self.device = None
            finally:
                self.passwords.clear()
                if self.snapshot()['stage'] == 'error':
                    self.web_password = ''
                self.update(busy=False)
        threading.Thread(target=work, daemon=True).start()

    def confirm_host(self, host, fingerprint):
        self.answer.clear()
        self.update(stage='identity', identity={'host': host, 'fingerprint': fingerprint}, message='Confirm that this is your miner.')
        if not self.answer.wait(180):
            raise ValueError('Identity confirmation timed out. Connect again when ready.')
        accepted = self.accept_identity
        self.update(stage='connecting', identity=None)
        return accepted

    def action(self, action, data):
        if action == 'identity':
            if self.snapshot()['stage'] != 'identity':
                raise ValueError('No identity confirmation is waiting.')
            self.accept_identity = data.get('accept') is True
            self.answer.set()
        elif action == 'scan':
            if data.get('authorized') is not True:
                raise ValueError('Confirm that this is your own local network.')
            def scan():
                self.update(stage='searching', devices=[], message='Looking for original KD BOX miners…')
                manual = data.get('host', '').strip()
                if manual:
                    found = probe(str(private_address(manual)), timeout=3)
                    devices = [found] if found else []
                else:
                    devices = discover(self.snapshot()['networks'], lambda n, total: self.progress(f'Checking local network: {n} / {total}'))
                self.update(stage='select', devices=devices, message=f'Found {len(devices)} KD BOX miner(s).' if devices else 'No original KD BOX found. Check power, Ethernet and your Wi-Fi network, or use the IP lookup below.')
            self.start(scan)
        elif action == 'connect':
            host = str(private_address(data.get('host', '')))
            if data.get('authorized') is not True or not any(d['host'] == host and d['compatible'] for d in self.snapshot()['devices']):
                raise ValueError('Select and confirm a discovered miner on stock 2.2.1.')
            web = data.get('web_password', '')
            ssh = data.get('ssh_password', '') or DEFAULT_SSH_PASSWORD
            if not web or not ssh or len(web) > 256 or len(ssh) > 256:
                raise ValueError('Enter the current dashboard and SSH root passwords. Neither will be changed.')
            def connect():
                if self.device:
                    self.device.close()
                self.passwords[:] = [web, ssh]
                self.update(stage='connecting', host=host, message='Connecting and verifying the stock base…')
                self.device = self.factory(host, ssh, self.confirm_host, progress=self.progress, web_password=web)
                info = self.device.verify_base()
                # Authenticate before the final installation confirmation.
                auth = self.device.api('/user/login?' + urlencode({'username': 'admin', 'password': web}))
                if not auth.get('JWT Token'):
                    raise ValueError('Dashboard password was rejected. No application files were changed.')
                installation = self.device.installation_state()
                verified = {k: info.get(k) for k in ('model', 'firmware', 'mcbversion')}
                if installation == 'installed':
                    self.update(stage='existing', message='KD BOX XBT is already installed. Open its dashboard, or restore a recovery backup below.', verified=verified)
                    return
                if installation == 'repair' and not self.snapshot()['payload_ready']:
                    self.update(stage='repair', verified=verified, message='Previous installation files were found, but the dashboard or startup is incomplete. Use a complete installer package to repair, or restore a backup below.')
                    return
                binary = validate_payload(self.root)
                self.device.require_space('/usr/config', len(binary) + 1024 * 1024)
                self.device.require_space('/root', sum((self.root / 'controller/web' / n).stat().st_size for n in WEB_ASSETS) + 1024 * 1024)
                self.update(stage='repair' if installation == 'repair' else 'review', verified=verified,
                            message='Previous installation needs repair. Saved settings will be kept and a new private backup will be made.' if installation == 'repair' else 'Verified stock 2.2.1, board, ARM architecture, stock engine hash, credentials and free space.')
                # Retained only until install/disconnect, never exposed through state.
                self.web_password = web
            self.start(connect)
        elif action == 'install':
            if (self.snapshot()['stage'] not in ('review', 'repair') or not self.snapshot()['payload_ready']
                    or not self.device or data.get('authorized') is not True):
                raise ValueError('Review the verified miner and accept the installation conditions first.')
            repair = self.snapshot()['stage'] == 'repair'
            def install():
                self.passwords[:] = [self.web_password]
                self.update(stage='installing', operation='repair' if repair else 'install', message='Creating your private recovery backup…')
                backup = backup_root() / datetime.now().strftime('%Y%m%d-%H%M%S-%f')
                # Show the recovery location even if the install fails midway.
                self.update(backup=str(backup))
                try:
                    options = {'repair_authorized': True} if repair else {}
                    self.device.install(self.root, self.web_password, backup, self.progress, owner_authorized=True, **options)
                finally:
                    self.refresh_backups()
                self.update(stage='done', message='Repair complete. Saved settings were preserved. Open the dashboard and supervise startup.' if repair else 'Installation complete. Open the dashboard to configure and verify XBT mining.')
                self.web_password = ''
                self.device.close()
                self.device = None
            self.start(install)
        elif action == 'restore':
            if self.snapshot()['stage'] not in ('review', 'repair', 'existing') or not self.device or data.get('authorized') is not True:
                raise ValueError('Connect to your miner and confirm recovery first.')
            path = self.backups.get(data.get('backup'))
            if path is None:
                raise ValueError('Choose a recovery backup from this computer.')
            def restore():
                self.update(stage='installing', operation='restore', message='Verifying this backup belongs to this miner, then restoring…')
                self.device.restore(path)
                self.update(stage='done', message='Backup restored. The miner is restarting. Open its dashboard after startup.')
                self.device.close()
                self.device = None
            self.start(restore)
        else:
            raise ValueError('Unknown action.')

    def close(self):
        self.accept_identity = False
        self.answer.set()
        if self.device:
            self.device.close()
        self.passwords.clear()
        self.web_password = ''


def make_server(wizard):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass  # Never log passwords, query strings, tokens or request bodies.

        def reply(self, code, body, kind='application/json'):
            if kind == 'application/json':
                body = json.dumps(body).encode()
            self.send_response(code)
            self.send_header('Content-Type', kind + ('; charset=utf-8' if kind.startswith('text/') or kind == 'application/json' else ''))
            self.send_header('Content-Length', str(len(body)))
            self.send_header('Cache-Control', 'no-store')
            self.send_header('X-Content-Type-Options', 'nosniff')
            self.send_header('Content-Security-Policy', "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
            self.end_headers()
            self.wfile.write(body)

        def allowed(self, authenticated=False):
            origin = f'http://127.0.0.1:{self.server.server_port}'
            if self.headers.get('Host') != urlsplit(origin).netloc or self.headers.get('Origin', origin) != origin:
                self.reply(403, {'error': 'Use the installer window opened on this computer.'})
                return False
            if authenticated and not hmac.compare_digest(self.headers.get('X-Installer-Token', ''), wizard.token):
                self.reply(403, {'error': 'Reopen the original installer window.'})
                return False
            return True

        def do_GET(self):
            if not self.allowed(self.path.startswith('/api/')):
                return
            if self.path == '/api/state':
                self.reply(200, wizard.snapshot())
            elif self.path in ('/', '/wizard.js', '/wizard.css', '/spectre-wordmark.png'):
                name, kind = {'/': ('index.html', 'text/html'), '/wizard.js': ('wizard.js', 'text/javascript'), '/wizard.css': ('wizard.css', 'text/css'), '/spectre-wordmark.png': ('spectre-wordmark.png', 'image/png')}[self.path]
                self.reply(200, (Path(__file__).parent / 'web' / name).read_bytes(), kind)
            elif self.path in ('/licenses.html', '/style.css', '/favicon-32.png', '/favicon-48.png', '/spectre-logo.png',
                               '/ChakraPetch-Regular.woff2', '/ChakraPetch-Bold.woff2', '/open-source.txt'):
                name = self.path[1:]
                kinds = {'.html': 'text/html', '.css': 'text/css', '.svg': 'image/svg+xml',
                         '.png': 'image/png', '.woff2': 'font/woff2', '.txt': 'text/plain'}
                try:
                    body = (wizard.root / 'controller/web' / name).read_bytes()
                except OSError:
                    self.reply(404, {'error': 'Extract the complete installer package to read its local notices.'})
                    return
                if name == 'licenses.html':
                    body = body.replace(b'<a class="license-back" href="./index.html">\xe2\x86\x90 Back to application</a>',
                                        b'<p class="license-back">Close this tab to return to the installer.</p>')
                self.reply(200, body, kinds[Path(name).suffix])
            else:
                self.reply(404, {'error': 'Not found'})

        def do_POST(self):
            if not self.allowed(True):
                return
            try:
                length = int(self.headers.get('Content-Length', '0'))
                if not 0 < length <= 4096 or self.headers.get('Content-Type') != 'application/json':
                    raise ValueError('Invalid request body.')
                data = json.loads(self.rfile.read(length))
                if not isinstance(data, dict):
                    raise ValueError('Invalid request body.')
                if self.path == '/api/close':
                    if wizard.snapshot()['busy']:
                        raise ValueError('Leave the installer running until the operation finishes.')
                    self.reply(200, {'ok': True})
                    threading.Thread(target=self.server.shutdown, daemon=True).start()
                    return
                if not self.path.startswith('/api/'):
                    self.reply(404, {'error': 'Not found'})
                    return
                wizard.action(self.path.removeprefix('/api/'), data)
                self.reply(200, {'ok': True})
            except (ValueError, TypeError, KeyError) as exc:
                self.reply(400, {'error': str(exc)})
    server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    server.timeout = 1
    return server


def main():
    wizard = Wizard()
    server = make_server(wizard)
    url = f'http://127.0.0.1:{server.server_port}/#{wizard.token}'
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    print('KD BOX XBT Installer is ready. Keep this window open.\n' + url, flush=True)
    try:
        open_installer_browser(url)
        worker.join()
    except KeyboardInterrupt:
        print('Installer interrupted. Check the miner before retrying if installation had begun.')
    finally:
        server.shutdown()
        worker.join()
        server.server_close()
        wizard.close()
