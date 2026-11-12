import contextlib
import http.client
import io
import threading
import unittest
from urllib.parse import urlsplit
from unittest.mock import Mock, patch

from flasher.browser import open_installer_browser
from flasher.wizard import Wizard, make_server, main


class BrowserStartup(unittest.TestCase):
    def test_text_browser_is_not_started(self):
        browser=Mock(name='text-browser');browser.name='/usr/bin/w3m'
        with patch('flasher.browser.webbrowser.get', return_value=browser), contextlib.redirect_stdout(io.StringIO()) as output:
            worker=open_installer_browser('http://127.0.0.1:12345/');worker.join(2)
            self.assertFalse(worker.is_alive())
        browser.open.assert_not_called()
        self.assertIn('text browser cannot run', output.getvalue())

    def test_blocking_browser_cannot_block_caller(self):
        entered,release=threading.Event(),threading.Event()
        browser=Mock();browser.name='graphical-browser'
        browser.open.side_effect=lambda *a,**k:(entered.set(),release.wait(3),True)[-1]
        with patch('flasher.browser.webbrowser.get', return_value=browser):
            worker=open_installer_browser('http://127.0.0.1:12345/')
            try:
                self.assertTrue(entered.wait(1));self.assertTrue(worker.daemon)
            finally:release.set();worker.join(2)
        self.assertFalse(worker.is_alive())

    def test_main_serves_before_opening_browser_and_closes_cleanly(self):
        with patch('flasher.wizard.local_networks',return_value=[]):wizard=Wizard()
        server=make_server(wizard)
        def browser(url):
            u=urlsplit(url)
            conn=http.client.HTTPConnection(u.hostname,u.port,timeout=2)
            conn.request('GET','/api/state',headers={'X-Installer-Token':u.fragment})
            response=conn.getresponse();self.assertEqual(response.status,200);response.read();conn.close()
            conn=http.client.HTTPConnection(u.hostname,u.port,timeout=2)
            conn.request('POST','/api/close','{}',{'X-Installer-Token':u.fragment,'Content-Type':'application/json'})
            response=conn.getresponse();self.assertEqual(response.status,200);response.read();conn.close()
        with patch('flasher.wizard.Wizard',return_value=wizard),patch('flasher.wizard.make_server',return_value=server),patch('flasher.wizard.open_installer_browser',side_effect=browser),contextlib.redirect_stdout(io.StringIO()):
            main()
        self.assertEqual(server.fileno(),-1)

    def test_shared_legal_page_is_local_and_cannot_expose_arbitrary_paths(self):
        with patch('flasher.wizard.local_networks',return_value=[]):wizard=Wizard()
        server=make_server(wizard);worker=threading.Thread(target=server.serve_forever);worker.start()
        try:
            for path,mime in [('/licenses.html','text/html'),('/style.css','text/css'),('/spectre-logo.png','image/png'),('/ChakraPetch-Regular.woff2','font/woff2')]:
                c=http.client.HTTPConnection('127.0.0.1',server.server_port,timeout=2);c.request('GET',path);r=c.getresponse();body=r.read();self.assertEqual(r.status,200);self.assertTrue(r.getheader('Content-Type').startswith(mime));self.assertIn("font-src 'self'",r.getheader('Content-Security-Policy'))
                if path=='/licenses.html':self.assertIn(b'Close this tab to return',body);self.assertIn(b'Apache License 2.0',body)
                c.close()
            c=http.client.HTTPConnection('127.0.0.1',server.server_port);c.request('GET','/../LICENSE');r=c.getresponse();self.assertEqual(r.status,404);r.read();c.close()
        finally:server.shutdown();worker.join();server.server_close();wizard.close()
