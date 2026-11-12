let poolRequest = null;
let poolKey = '';
let wifiData = {};
let cloudData = {};
let report = null;
let reportRequest = 0;
let reportHalted;
let privatePageRequest = 0;
let privatePageController;
let reportController;
let networkTrialRequest = null;

function poolHealth(pool) {
  if (!online) return ['Controller unavailable', 'neutral'];
  if (online && lastStatus?.halted) return ['Mining paused', 'warning'];
  if (pool.mining) return ['Mining here', 'positive'];
  if (pool.connected) return ['Standby', 'neutral'];
  if (pool.telemetry_available) return ['Disconnected', 'warning'];
  return ['No connection data', 'neutral'];
}

function renderPoolSummary() {
  const primary = poolView?.[0];
  const active = poolView?.find(pool => pool.mining);
  const address = primary?.url?.replace(/^stratum\+(?:tcp|ssl):\/\//, '');
  setText('primary-pool', address || (poolView?.length === 0 ? 'No pool configured' : 'Unavailable'));
  setText('pool-activity', online && lastStatus?.halted ? 'Mining paused'
    : active ? (active === primary ? 'Mining on primary' : 'Mining on a backup')
    : primary ? poolHealth(primary)[0] : '—');
}

function setPoolForm(open) {
  $('pool-form').hidden = !open;
  $('pool-toggle').setAttribute('aria-expanded', String(open));
  $('pool-error').hidden = true;
  if (open) $('pool-url').focus();
}
$('pool-toggle').onclick = () => setPoolForm($('pool-form').hidden);
$('pool-cancel').onclick = () => {
  $('pool-form').reset();
  setPoolForm(false);
  $('pool-toggle').focus();
};

function sortedPools(pools) {
  return [...pools].sort((a, b) => (a['pool-priority'] ?? 0) - (b['pool-priority'] ?? 0));
}

function renderPools(force = false) {
  renderPoolSummary();
  $('pool-toggle').hidden = !unlocked || poolView === null;
  if (!unlocked) setPoolForm(false);
  $('pool-state').hidden = Boolean(poolView?.length);
  setText('pool-state', poolView === null ? 'Saved pools are unavailable.'
    : 'No pools configured. Add the pool you want this miner to use.');

  const list = $('pools');
  const paused = online && Boolean(lastStatus?.halted);
  const key = JSON.stringify([(poolView || []).map(({url, user, mining, connected, telemetry_available}) => [url, user, mining, connected, telemetry_available]), unlocked, paused, online]);
  if (!force && (key === poolKey || list.contains(document.activeElement))) return;
  poolKey = key;
  list.replaceChildren();

  const shownPools = poolView || [];
  shownPools.forEach((pool, index) => {
    const row = document.createElement('article');
    row.className = 'pool';
    const title = document.createElement('h3');
    title.textContent = index === 0 ? 'Primary pool' : 'Backup pool ' + index;
    const badge = document.createElement('span');
    badge.className = 'status-label';
    [badge.textContent, badge.dataset.tone] = poolHealth(pool);

    const info = document.createElement('div');
    const address = document.createElement('span');
    address.className = 'pool-address';
    address.textContent = pool.url || 'Address unavailable';
    const worker = document.createElement('span');
    worker.className = 'pool-worker';
    worker.textContent = pool.user ? 'Worker ' + pool.user : 'Worker unavailable';
    info.append(address, worker);
    row.append(title, badge, info);

    if (unlocked) {
      const controls = document.createElement('div');
      controls.className = 'actions';
      for (const [label, disabled, callback] of [
        ['Move up', index === 0, () => editPool(index, index - 1, shownPools)],
        ['Move down', index === poolView.length - 1, () => editPool(index, index + 1, shownPools)],
        ['Remove', poolView.length === 1, () => {
          if (confirm('Remove this pool from the miner?')) editPool(index, null, shownPools);
        }],
      ]) {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'button secondary';
        button.textContent = label;
        button.disabled = disabled;
        button.setAttribute('aria-label', label + ' ' + title.textContent.toLowerCase());
        button.onclick = callback;
        controls.append(button);
      }
      row.append(controls);
    }
    list.append(row);
  });
}

function loadPools(force = false) {
  if (poolRequest) return poolRequest;
  const revision = sessionRevision;
  poolRequest = (async () => {
    try {
      const view = await api('pool-status');
      if (!Array.isArray(view?.pools)) throw Error('Invalid pool response.');
      poolView = sortedPools(view.pools.slice(0, 10));
      if (revision === sessionRevision && typeof lastStatus?.controls_unlocked !== 'boolean') {
        setUnlocked(view.editable === true);
      }
    } catch (error) {
      if (error.name === 'AbortError') return;
      poolView = null;
    } finally {
      poolRequest = null;
      renderPools(force);
      renderStatus();
    }
  })();
  return poolRequest;
}

function displayedAddress(address) {
  try {
    const url = new URL(address);
    url.username = '';
    url.password = '';
    url.search = '';
    url.hash = '';
    return url.href;
  } catch { return ''; }
}

function editPool(index, destination, expected = poolView) {
  action(async () => {
    const current = sortedPools(await api('pools'));
    if (current.length !== expected.length || current.some((pool, i) =>
      displayedAddress(pool.url) !== displayedAddress(expected[i].url) || pool.user !== expected[i].user)) {
      await loadPools(true);
      throw Error('The pool list changed. Review the latest order and try again.');
    }
    if (destination === null) await api('pool-delete', current[index]);
    else {
      [current[index], current[destination]] = [current[destination], current[index]];
      current.forEach((pool, i) => { pool['pool-priority'] = i; });
      await api('pools', current);
    }
    await loadPools(true);
  }, destination === null ? 'Pool removed.' : 'Pool order saved.');
}

function normalizePoolURL(input) {
  const address = input.trim();
  if (!address) throw Error('Enter a pool hostname and port, such as pool.example:3333.');
  if (/\s/.test(address)) throw Error('Remove spaces from the pool address.');
  const match = address.match(/^(?:([a-z][a-z0-9+.-]*):\/\/)?(.*)$/i);
  let scheme = (match[1] || 'stratum+tcp').toLowerCase();
  if (scheme === 'tcp') scheme = 'stratum+tcp';
  if (scheme === 'ssl') scheme = 'stratum+ssl';
  if (!['stratum+tcp', 'stratum+ssl'].includes(scheme)) throw Error('Use a Stratum TCP or SSL pool address.');
  const target = match[2].replace(/\/$/, '');
  if (/[/@?#]/.test(target)) throw Error('Enter only the pool hostname and port. Put worker credentials in their own fields.');
  const endpoint = target.match(/^(\[[^\]]+\]|[^:]+):([^:]*)$/);
  if (!endpoint) throw Error('Include a port, such as :3333. Put IPv6 addresses in brackets.');
  const [, host, port] = endpoint;
  if (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535) throw Error('Enter a port from 1 to 65535.');
  if (!host.startsWith('[') && !host.replace(/\.$/, '').split('.').every(label =>
    /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(label))) {
    throw Error('Enter a valid hostname or IP address.');
  }
  try { new URL('http://' + host + ':' + port); }
  catch { throw Error('Enter a valid hostname or IP address.'); }
  if (address.length > 512) throw Error('The pool address is too long.');
  return scheme + '://' + host + ':' + port;
}

$('pool-url').addEventListener('input', () => { $('pool-error').hidden = true; });
$('pool-form').onsubmit = event => {
  event.preventDefault();
  let url;
  try {
    url = normalizePoolURL($('pool-url').value);
    if (!$('pool-user').value.trim()) throw Error('Enter the payout address or worker name supplied by your pool.');
  } catch (error) {
    $('pool-error').textContent = error.message;
    $('pool-error').hidden = false;
    return;
  }
  $('pool-url').value = url;
  action(async () => {
    await api('pool-add', {url, user: $('pool-user').value.trim(), pass: $('pool-pass').value});
    $('pool-form').reset();
    setPoolForm(false);
    await loadPools(true);
  }, 'Pool saved.');
};

const networkFields = [
  ['ip', 'IPv4addr'], ['mask', 'Subnetmask'], ['gateway', 'Router'], ['dns1', 'DNS1'], ['dns2', 'DNS2'],
];
function showNetworkFields() {
  const dhcp = $('dhcp').checked;
  const auto = $('auto-dns').checked;
  $('manual-address').hidden = dhcp;
  $('manual-dns').hidden = auto;
  for (const id of ['ip', 'mask', 'gateway']) $(id).required = !dhcp;
  $('dns1').required = !auto;
}
$('dhcp').onchange = showNetworkFields;
$('auto-dns').onchange = showNetworkFields;
function loadNetworkTrial(signal) {
  if (networkTrialRequest) return networkTrialRequest;
  const revision = sessionRevision;
  networkTrialRequest = (async () => {
    try {
      const trial = await api('network-trial', undefined, {signal});
      if (revision !== sessionRevision || !unlocked || pageName() !== 'network') return;
      $('network-state').hidden = !trial;
      $('network-confirm').hidden = !trial;
      if (trial) setText('network-state', 'A network change is awaiting confirmation. Confirm it after reconnecting to keep it.');
    } finally { networkTrialRequest = null; }
  })();
  return networkTrialRequest;
}
$('network-form').onsubmit = event => {
  event.preventDefault();
  if (!confirm('Apply network changes? Reconnect and confirm within two minutes to keep them.')) return;
  action(async () => {
    const setIP = {};
    for (const [id, key] of networkFields) {
      const value = $(id).value.trim();
      if (value) setIP[key] = value;
    }
    await api('network', {useDHCP: $('dhcp').checked, useAuto: $('auto-dns').checked, setIP});
    $('network-state').hidden = false;
    $('network-confirm').hidden = false;
    setText('network-state', 'Reconnect at the new address and confirm within two minutes.');
  }, 'Network change sent. Reconnect and confirm to keep it.');
};
$('network-confirm').onclick = () => action(async () => {
  await api('network-confirm', {});
  await loadNetworkTrial();
}, 'Network settings kept.');

async function loadWifi(signal) {
  const revision = sessionRevision;
  const data = await api('wifi', undefined, {signal});
  if (revision !== sessionRevision || !unlocked || pageName() !== 'network') return;
  wifiData = data;
  setText('wifi-state', wifiData.exist ? (wifiData.enable ? 'Wi-Fi is enabled.' : 'Wi-Fi is disabled.') : 'No Wi-Fi adapter detected.');
  $('wifi-controls').hidden = !wifiData.exist;
  $('wifi-enable').checked = Boolean(wifiData.enable);
  const list = $('wifi-list');
  list.replaceChildren();
  for (const entry of Array.isArray(wifiData.status) ? wifiData.status.slice(0, 128) : []) {
    if (!entry.ssid) continue;
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'button secondary';
    button.textContent = entry.ssid;
    button.onclick = () => {
      $('ssid').value = entry.ssid;
      $('wifi-password').focus();
    };
    list.append(button);
  }
}
$('wifi-enable').onchange = () => action(async () => {
  try { await api('wifi', {enable: $('wifi-enable').checked}); }
  finally { await loadWifi(); }
}, 'Wi-Fi setting saved.');
$('wifi-scan').onclick = () => action(async () => {
  await api('wifi', {scan: true});
  await loadWifi();
}, 'Wi-Fi scan complete.');
$('wifi-form').onsubmit = event => {
  event.preventDefault();
  action(async () => {
    await api('wifi', {connect: {ssid: $('ssid').value, password: $('wifi-password').value}});
    $('wifi-password').value = '';
    await loadWifi();
  }, 'Wi-Fi connection requested.');
};
for (const operation of ['disconnect', 'forget']) {
  $('wifi-' + operation).onclick = () => {
    const ssid = $('ssid').value.trim();
    if (!ssid) {
      showFeedback('Select or enter a Wi-Fi network name first.', 'warning');
      $('ssid').focus();
      return;
    }
    action(async () => {
      await api('wifi', {[operation]: {ssid}});
      await loadWifi();
    }, operation === 'forget' ? 'Wi-Fi network forgotten.' : 'Wi-Fi network disconnected.');
  };
}

function systemDetails(details) {
  const labels = {model: 'Model', firmware: 'Stock firmware', mcbversion: 'Control board', hardware: 'Hardware'};
  const facts = $('system-info');
  facts.replaceChildren();
  for (const [key, value] of Object.entries(details)) {
    const row = document.createElement('div');
    const term = document.createElement('dt');
    const description = document.createElement('dd');
    term.textContent = labels[key] || key.replace(/_/g, ' ');
    description.textContent = typeof value === 'object' && value !== null ? JSON.stringify(value) : (value ?? '—');
    row.append(term, description);
    facts.append(row);
  }
}
$('identify').onchange = () => action(async () => {
  try { await api('identify', {enabled: $('identify').checked}); }
  finally { $('identify').checked = Boolean((await api('device-settings')).ledcontrol); }
}, 'Identification LED updated.');
$('password-form').onsubmit = event => {
  event.preventDefault();
  action(async () => {
    await api('password', {old: $('old-password').value, new: $('new-password').value, confirm: $('confirm-password').value});
    $('password-form').reset();
    setUnlocked(false);
  }, 'Password changed. Unlock with the new password.');
};
$('reboot').onclick = () => {
  if (prompt('Type REBOOT to restart the miner.') === 'REBOOT') {
    action(() => api('reboot', {confirm: 'REBOOT'}), 'Restart requested. The dashboard will reconnect when the miner is ready.');
  }
};
$('cloud-form').onsubmit = event => {
  event.preventDefault();
  action(() => api('cloud', {...cloudData, enable: $('cloud-enable').checked, userkey: $('cloud-key').value || null}), 'Cloud settings saved.');
};
$('rgb-save').onclick = () => action(() => api('rgb', JSON.parse($('rgb-json').value)), 'Lighting saved.');

function clearReport(message) {
  report = null;
  $('diagnostic-save').disabled = true;
  $('chips').replaceChildren();
  $('diagnostic-output').textContent = '';
  $('report-panel').hidden = true;
  setText('diagnostic-state', message);
}
async function diagnostic() {
  const kind = $('diagnostic-kind').value;
  const request = ++reportRequest;
  reportController?.abort();
  reportController = new AbortController();
  if (kind !== 'safety-stop' && !unlocked) {
    clearReport('Unlock controls to view this report.');
    return;
  }
  clearReport('Loading report…');
  try {
    const response = await api('diagnostics?kind=' + encodeURIComponent(kind), undefined, {signal: reportController.signal});
    if (request !== reportRequest) return;
    if (typeof response?.body !== 'string') throw Error('The miner returned an unreadable report.');
    if (!response.body.trim()) {
      setText('diagnostic-state', response.note || 'The stock firmware has no entries for this report.');
      return;
    }
    report = {kind, body: response.body.slice(-65536)};
    $('diagnostic-output').textContent = report.body;
    $('report-panel').hidden = false;
    $('diagnostic-save').disabled = false;
    setText('diagnostic-state', (response.note || 'Report loaded.') + (response.truncated ? ' Showing the most recent entries.' : ''));
    if (kind === 'icinfo') {
      let rows;
      try { rows = JSON.parse(report.body).drawdata; } catch {}
      const chips = Array.isArray(rows) ? rows.flat().filter(chip => chip?.chipindex != null).slice(0, 16) : [];
      for (const chip of chips) {
        const card = document.createElement('div');
        card.className = 'chip';
        const name = document.createElement('strong');
        const temp = document.createElement('span');
        const errors = document.createElement('span');
        name.textContent = 'Chip ' + chip.chipindex;
        temp.textContent = typeof chip.temp === 'number' && chip.temp > 0 && chip.temp <= 200
          ? chip.temp + '°C' : 'Temperature unavailable';
        errors.textContent = chip.hwerr == null ? 'Error count unavailable' : chip.hwerr + ' hardware errors';
        card.append(name, temp, errors);
        $('chips').append(card);
      }
      $('report-panel').open = !chips.length;
    } else $('report-panel').open = true;
  } catch (error) {
    if (request === reportRequest && error.name !== 'AbortError') clearReport(failureMessage(error));
  }
}
$('diagnostic-form').onsubmit = event => {
  event.preventDefault();
  if ($('diagnostic-kind').value !== 'safety-stop' && !unlocked) openLogin();
  else void diagnostic();
};
$('diagnostic-kind').onchange = diagnostic;
$('diagnostic-save').onclick = () => {
  if (!report || (report.kind !== 'safety-stop' && !unlocked)) return;
  const url = URL.createObjectURL(new Blob([report.body], {type: 'text/plain'}));
  const link = document.createElement('a');
  link.href = url;
  link.download = 'kdbox-xbt-' + report.kind + '.txt';
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
};

function clearPrivateViews() {
  $('password-form').reset();
  privatePageController?.abort();
  if (report?.kind !== 'safety-stop') reportController?.abort();
  wifiData = {};
  cloudData = {};
  $('wifi-controls').hidden = true;
  $('wifi-list').replaceChildren();
  $('wifi-form').reset();
  setText('wifi-state', 'Unlock to check for a wireless adapter.');
  $('network-form').reset();
  showNetworkFields();
  $('network-state').hidden = true;
  $('network-confirm').hidden = true;
  systemDetails({'Device details': 'Unlock to view'});
  $('identify').checked = false;
  $('cloud-form').reset();
  $('rgb-panel').hidden = true;
  $('rgb-json').value = '';
  if ($('diagnostic-kind').value !== 'safety-stop') {
    reportRequest++;
    clearReport('Unlock controls to view this report.');
  }
}

async function loadPrivatePage() {
  const request = ++privatePageRequest;
  privatePageController?.abort();
  privatePageController = new AbortController();
  const signal = privatePageController.signal;
  const read = path => api(path, undefined, {signal});
  if (!unlocked || !dashboardVisible()) return;
  const page = pageName();
  try {
    if (page === 'network') {
      const network = await read('network');
      if (request !== privatePageRequest || !unlocked) return;
      $('dhcp').checked = network.useDHCP;
      $('auto-dns').checked = network.useAuto;
      for (const [id, key] of networkFields) $(id).value = network.setIP?.[key] || '';
      showNetworkFields();
      await Promise.all([loadNetworkTrial(signal), loadWifi(signal)]);
    }
    if (page === 'system') {
      const [info, settings, cloud, rgb] = await Promise.all([
        read('system'), read('device-settings'), read('cloud').catch(() => null), read('rgb').catch(() => null),
      ]);
      if (request !== privatePageRequest || !unlocked) return;
      systemDetails(info);
      $('identify').checked = Boolean(settings.ledcontrol);
      cloudData = cloud || {};
      $('cloud-enable').checked = Boolean(cloudData.enable);
      $('cloud-key').value = cloudData.userkey || '';
      $('rgb-panel').hidden = !rgb;
      if (rgb) $('rgb-json').value = JSON.stringify(rgb, null, 2);
    }
  } catch (error) {
    if (request === privatePageRequest && error.status !== 401 && error.name !== 'AbortError') showFeedback(failureMessage(error), 'danger');
  }
}

document.addEventListener('controlschange', () => {
  renderPools(true);
  if (!unlocked) clearPrivateViews();
  if (pageName() === 'diagnostics' && $('diagnostic-kind').value !== 'safety-stop') void diagnostic();
  void loadPrivatePage();
});
document.addEventListener('minerstatus', () => {
  renderPools();
  if (online && lastStatus.halted !== reportHalted) {
    reportHalted = lastStatus.halted;
    if (pageName() === 'diagnostics' && ['safety-stop', 'icinfo'].includes($('diagnostic-kind').value)) void diagnostic();
  }
});
window.addEventListener('hashchange', () => {
  reportRequest++;
  reportController?.abort();
  if (pageName() === 'mining' || pageName() === 'overview') void loadPools();
  if (pageName() === 'diagnostics') void diagnostic();
  void loadPrivatePage();
});
void loadPools();
void loadPrivatePage();
registerPolling(() => {
  if (!busy) void loadPools();
  if (!busy && unlocked && pageName() === 'network') void loadNetworkTrial().catch(() => {});
}, 10000);

document.addEventListener('dashboardpause', () => {
  reportRequest++;
  privatePageRequest++;
  reportController?.abort();
  privatePageController?.abort();
});
document.addEventListener('dashboardresume', () => {
  void loadPrivatePage();
  if (pageName() === 'diagnostics') void diagnostic();
});
