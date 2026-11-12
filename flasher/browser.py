"""Open a graphical browser without making the local server wait for it."""
from pathlib import Path
import threading
import webbrowser

TEXT_BROWSERS = {'w3m', 'lynx', 'links', 'links2', 'elinks'}


def open_installer_browser(url):
    def launch():
        try:
            browser = webbrowser.get()
        except webbrowser.Error:
            print('No browser launcher was found. Open the URL above in a JavaScript-capable browser.', flush=True)
            return
        if Path(str(browser.name)).name.lower() in TEXT_BROWSERS:
            print('A text browser cannot run this installer. Open the URL above in Firefox, Chrome, Edge or another JavaScript-capable browser.', flush=True)
            return
        try:
            opened = browser.open(url, new=1)
        except Exception:
            opened = False
        if not opened:
            print('The browser could not be opened automatically. The installer is ready; open the URL above in your browser.', flush=True)

    # Browser discovery and launchers may wait for a window or desktop service.
    # They must not block HTTP serving or keep a closed installer alive.
    worker = threading.Thread(target=launch, daemon=True)
    worker.start()
    return worker
