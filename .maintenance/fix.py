"""Apply the final reviewed backup-permissions correction before CI."""
import hashlib
from pathlib import Path
import subprocess
p = Path('flasher/device.py')
before = '275517bc0c19e2235f73f05be853fcba0134279190d1d76362ab93796ccb9df8'
after = '8fcca95e05bca162517b0dab6de309735a9873280c9756c877424ea38cd0122f'
data = p.read_bytes()
if hashlib.sha256(data).hexdigest() != after:
    if hashlib.sha256(data).hexdigest() != before:
        raise RuntimeError('Concurrent change; refusing to overwrite device.py')
    needle = "        (destination / 'device.json').write_text(json.dumps(identity),encoding='utf-8')\n"
    text = data.decode('utf-8')
    if text.count(needle) != 1:
        raise RuntimeError('Expected identity-write site missing')
    data = text.replace(needle, needle + "        (destination / 'device.json').chmod(0o600)\n").encode('utf-8')
    if hashlib.sha256(data).hexdigest() != after:
        raise RuntimeError('Correction checksum mismatch')
    p.write_bytes(data)
    subprocess.run(['git', 'add', '--', str(p)], check=True)
