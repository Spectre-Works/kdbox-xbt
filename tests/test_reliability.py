import unittest, socket, errno, hashlib
from unittest.mock import Mock, patch
import paramiko
from flasher.device import Device
from flasher.messages import error_message

class Reliability(unittest.TestCase):
    def connect(self, failures, confirm=lambda *a:True):
        clients=[Mock() for _ in failures]
        for client, failure in zip(clients,failures):client.connect.side_effect=failure
        with patch('flasher.device.paramiko.SSHClient',side_effect=clients), patch('flasher.device.time.sleep'):
            device=Device('192.168.1.2','secret',confirm)
        return device,clients

    def test_banner_retry_uses_new_client_and_explicit_timeouts(self):
        d,clients=self.connect([paramiko.SSHException('Error reading SSH protocol banner'),None])
        clients[0].close.assert_called_once()
        self.assertIs(d.client,clients[1])
        self.assertEqual(clients[1].connect.call_args.kwargs['banner_timeout'],30)
        clients[1].get_transport().set_keepalive.assert_called_once_with(15)

    def test_wrong_password_never_retried(self):
        c=Mock();c.connect.side_effect=paramiko.AuthenticationException('bad')
        with patch('flasher.device.paramiko.SSHClient',return_value=c) as factory, patch.object(Device,'enable_ssh') as enable:
            with self.assertRaisesRegex(RuntimeError,'password was rejected'):Device('192.168.1.2','bad',Mock(),web_password='web')
        self.assertEqual(factory.call_count,1);enable.assert_not_called();c.close.assert_called_once()

    def test_exhaustion_is_bounded_and_does_not_enable_ssh_on_banner_failure(self):
        clients=[Mock() for _ in range(4)]
        for c in clients:c.connect.side_effect=paramiko.SSHException('Error reading SSH protocol banner')
        with patch('flasher.device.paramiko.SSHClient',side_effect=clients),patch('flasher.device.time.sleep'),patch.object(Device,'enable_ssh') as enable:
            with self.assertRaisesRegex(RuntimeError,'four attempts'):Device('192.168.1.2','secret',Mock(),web_password='web')
        enable.assert_not_called()
        for c in clients:c.close.assert_called_once()

    def test_refused_enables_only_once(self):
        a,b=Mock(),Mock();a.connect.side_effect=paramiko.ssh_exception.NoValidConnectionsError({('192.168.1.2',22):ConnectionRefusedError(errno.ECONNREFUSED,'refused')})
        with patch('flasher.device.paramiko.SSHClient',side_effect=[a,b]),patch('flasher.device.time.sleep'),patch.object(Device,'enable_ssh') as enable:
            Device('192.168.1.2','secret',Mock(),web_password='web')
        enable.assert_called_once_with('192.168.1.2','web')

    def test_changed_identity_on_retry_is_rejected(self):
        a,b=Mock(),Mock();key1,key2=Mock(),Mock();key1.asbytes.return_value=b'one';key2.asbytes.return_value=b'two';confirm=Mock(return_value=True)
        def first(*args,**kwargs):
            a.set_missing_host_key_policy.call_args.args[0].missing_host_key(a,'miner',key1)
            raise socket.timeout()
        def second(*args,**kwargs):b.set_missing_host_key_policy.call_args.args[0].missing_host_key(b,'miner',key2)
        a.connect.side_effect=first;b.connect.side_effect=second
        with patch('flasher.device.paramiko.SSHClient',side_effect=[a,b]),patch('flasher.device.time.sleep'):
            with self.assertRaisesRegex(RuntimeError,'identity changed'):Device('192.168.1.2','secret',confirm)
        confirm.assert_called_once();b.close.assert_called_once()

    def test_read_only_query_retries_but_login_and_write_do_not(self):
        for path,data,count in [('/mcb/status',None,3),('/user/login?password=x',None,1),('/mcb/algosetting',{},1)]:
            d=Device.__new__(Device);d._api_once=Mock(side_effect=socket.timeout())
            with patch('flasher.device.time.sleep'),self.assertRaises(RuntimeError):d.api(path,data)
            self.assertEqual(d._api_once.call_count,count)

    def test_transfer_mismatch_never_replaces_destination(self):
        d=Device.__new__(Device);d.client=Mock();d.client.open_sftp.return_value.__enter__=Mock(return_value=Mock());d.client.open_sftp.return_value.__exit__=Mock(return_value=False)
        s=d.client.open_sftp.return_value.__enter__.return_value;s.open.return_value.__enter__=Mock();s.open.return_value.__exit__=Mock(return_value=False)
        d.command=Mock(return_value='incorrect  file')
        with self.assertRaisesRegex(RuntimeError,'verification failed'):d.write('/test',b'data')
        self.assertEqual(d.command.call_count,1)

    def test_partial_install_is_not_retried(self):
        d=Device.__new__(Device)
        def fail(*a):d.installation_started=True;raise socket.timeout('lost')
        d._install=Mock(side_effect=fail)
        with self.assertRaisesRegex(RuntimeError,'Recovery backup: backup'):d.install('root','secret','backup')
        d._install.assert_called_once()

    def test_credentials_are_redacted(self):
        self.assertNotIn('secret',error_message(ValueError('secret and a%26b'), 'secret','a&b'))
        self.assertNotIn('a%26b',error_message(ValueError('a%26b'),'a&b'))

    def test_bad_input_before_connect(self):
        with patch('flasher.device.paramiko.SSHClient') as c:
            with self.assertRaisesRegex(ValueError,'IP only'):Device('http://192.168.1.2','secret',Mock())
        c.assert_not_called()

    def test_controller_health_uses_controller_port_and_stock_api_uses_port_80(self):
        for path, port in [('/api/status', 8088), ('/mcb/status', 80)]:
            d=Device.__new__(Device);d.client=Mock();d.token=None
            response=Mock(status=200);response.read.return_value=b'{}'
            with patch('flasher.device.http.client.HTTPConnection') as connection:
                connection.return_value.getresponse.return_value=response
                self.assertEqual(d._api_once(path), {})
            d.client.get_transport().open_channel.assert_called_once_with(
                'direct-tcpip', ('127.0.0.1', port), ('127.0.0.1', 0), timeout=10)
            connection.return_value.close.assert_called_once()

    def test_existing_safety_stop_blocks_install_before_backup_or_writes(self):
        d=Device.__new__(Device);d.inspect=Mock(return_value={'firmware':'2.2.3'})
        d.command=Mock(return_value='halted');d.backup=Mock();d.write=Mock()
        with self.assertRaisesRegex(RuntimeError,'active safety stop'):
            d.install('nonexistent-root','secret','backup')
        d.backup.assert_not_called();d.write.assert_not_called()
        self.assertFalse(d.installation_started)
