"""Bounded, read-only discovery on directly connected private IPv4 networks."""
from concurrent.futures import ThreadPoolExecutor, as_completed
import http.client
import ipaddress
import json
import platform
import re
import socket
import subprocess

FIRMWARE_URL = 'https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.1.cpb'


def private_address(value):
    address = ipaddress.IPv4Address(value)
    if not any(address in ipaddress.ip_network(n) for n in ('10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16')):
        raise ValueError('Use a private LAN IPv4 address, such as 192.168.1.100.')
    return address


def local_networks():
    system = platform.system()
    command = (['ip', '-j', '-4', 'address', 'show', 'up'] if system == 'Linux' else
               ['powershell.exe', '-NoProfile', '-NonInteractive', '-Command',
                'Get-NetIPAddress -AddressFamily IPv4 | Select-Object IPAddress,PrefixLength | ConvertTo-Json -Compress'] if system == 'Windows' else ['ifconfig'])
    interfaces = []
    try:
        output = subprocess.run(command, capture_output=True, text=True, timeout=8, check=True).stdout
        if system == 'Linux':
            interfaces = [(a['local'], a['prefixlen']) for entry in json.loads(output)
                          for a in entry.get('addr_info', []) if a.get('family') == 'inet']
        elif system == 'Windows':
            rows = json.loads(output)
            interfaces = [(a['IPAddress'], a['PrefixLength']) for a in (rows if isinstance(rows, list) else [rows])]
        else:
            for address, mask in re.findall(r'inet (\d+\.\d+\.\d+\.\d+).*?netmask (0x[0-9a-f]+|[\d.]+)', output):
                mask = str(ipaddress.IPv4Address(int(mask, 16))) if mask.startswith('0x') else mask
                interfaces.append((address, ipaddress.IPv4Network('0.0.0.0/' + mask).prefixlen))
    except (OSError, subprocess.SubprocessError, ValueError, KeyError, TypeError):
        # A UDP route lookup sends no packet and needs no privileged interface API.
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
                sock.connect(('192.168.0.1', 9))
                interfaces.append((sock.getsockname()[0], 24))
        except OSError:
            pass
    networks = set()
    for address, prefix in interfaces:
        try:
            address = private_address(address)
            # Do not sweep a /8 or /16. Start with the local /24; manual lookup
            # remains available for a miner elsewhere on a larger owner LAN.
            network = ipaddress.ip_network(f'{address}/{max(24, int(prefix))}', strict=False)
            if network.prefixlen <= 30:
                networks.add(str(network))
        except (ValueError, TypeError):
            continue
    return sorted(networks)[:4]


def probe(host, timeout=0.8):
    host = str(private_address(host))
    connection = http.client.HTTPConnection(host, timeout=timeout)
    try:
        connection.request('GET', '/mcb/status', headers={'Accept': 'application/json'})
        response = connection.getresponse()
        body = response.read(65537)
        if response.status != 200 or len(body) > 65536:
            return None
        data = json.loads(body)
        if not isinstance(data, dict) or data.get('model') != 'Goldshell-KDBox':
            return None
        # Never return arbitrary dashboard fields or strings as HTML.
        return {'host': host, 'model': 'Goldshell-KDBox',
                'firmware': str(data.get('firmware', 'Unknown'))[:32],
                'board': str(data.get('mcbversion', 'Verified after connecting'))[:64],
                'compatible': data.get('firmware') == '2.2.1'}
    except (OSError, http.client.HTTPException, ValueError):
        return None
    finally:
        connection.close()


def discover(networks, progress=lambda done, total: None):
    hosts = set()
    for value in networks:
        network = ipaddress.ip_network(value)
        if network.version != 4 or network.prefixlen < 24:
            raise ValueError('Discovery is limited to local IPv4 subnets of at most 254 hosts each.')
        private_address(network.network_address)
        private_address(network.broadcast_address)
        hosts.update(str(host) for host in network.hosts())
    if len(hosts) > 1024:
        raise ValueError('Too many addresses for one discovery pass.')
    devices = []
    with ThreadPoolExecutor(max_workers=32) as pool:
        pending = [pool.submit(probe, host) for host in sorted(hosts)]
        for done, future in enumerate(as_completed(pending), 1):
            result = future.result()
            if result:
                devices.append(result)
            progress(done, len(hosts))
    return sorted(devices, key=lambda d: int(ipaddress.IPv4Address(d['host'])))
