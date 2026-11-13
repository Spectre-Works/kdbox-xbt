const $ = id => document.getElementById(id);
const apiBase = new URL('/api/', location.href);
apiBase.port = '8088';
if (location.port === apiBase.port) {
  const maintenance = $('original-maintenance');
  const stockURL = new URL(maintenance.href);
  stockURL.port = '80';
  maintenance.href = stockURL;
}

const pages = {
  overview: ['Miner overview', 'Check mining activity, cooling, and controller status.'],
  mining: ['Mining setup', 'Manage pools, select a coin, and adjust performance.'],
  network: ['Network', 'Manage how the miner connects to your local network.'],
  system: ['System', 'View the device and manage its settings.'],
  diagnostics: ['Diagnostics', 'Review safety stops, chip readings, and device reports.'],
};
const algorithms = ['Kadena (KDA)', 'Bitcoin (XBT)'];
let unlocked = false;
let sessionRevision = 0;
let busy = false;
let online = false;
let lastStatus = null;
let poolView = null;
let hasConnected = false;
let performanceEdited = false;
let performanceSaveGeneration = 0;
let coolingEdited = false;
let coolingRevision = 0;
let coolingEditedAt = 0;
let coolingSaveTimer;
let coolingSaving = false;
let coolingSaveGeneration = 0;
let feedbackTimer;
let feedbackHideTimer;
let algorithmEdited = false;
let statusRequest = null;
const activeReads = new Set();
const backgroundPolls = [];
let pageActive = true;
function dashboardVisible() { return pageActive && !document.hidden; }
function registerPolling(run, interval) {
  const poll = {run, interval, timer: null};
  backgroundPolls.push(poll);
  if (dashboardVisible()) { poll.timer = setInterval(run, interval); void run(); }
}
function updatePolling() {
  for (const poll of backgroundPolls) {
    if (!dashboardVisible()) { clearInterval(poll.timer); poll.timer = null; }
    else if (poll.timer === null) { poll.timer = setInterval(poll.run, poll.interval); void poll.run(); }
  }
  if (!dashboardVisible()) {
    for (const controller of activeReads) controller.abort();
    document.dispatchEvent(new Event('dashboardpause'));
  } else document.dispatchEvent(new Event('dashboardresume'));
}
document.addEventListener('visibilitychange', updatePolling);
window.addEventListener('pagehide', () => { pageActive = false; updatePolling(); });
window.addEventListener('pageshow', () => { pageActive = true; updatePolling(); });

function pageName() {
  const name = location.hash.slice(1);
  return Object.hasOwn(pages, name) ? name : 'overview';
}

function showPage() {
  const name = pageName();
  [$('page-title').textContent, $('page-description').textContent] = pages[name];
  document.querySelectorAll('[data-page]').forEach(page => {
    page.hidden = page.dataset.page !== name;
  });
  document.querySelectorAll('nav a').forEach(link => {
    if (link.hash === '#' + name) link.setAttribute('aria-current', 'page');
    else link.removeAttribute('aria-current');
  });
}
document.querySelector('.skip-link').onclick = event => {
  event.preventDefault();
  $('main').focus();
};

function setTheme(theme) {
  document.documentElement.dataset.theme = theme;
  $('theme').textContent = theme === 'dark' ? 'Light mode' : 'Dark mode';
}
try {
  const saved = localStorage.getItem('theme');
  if (saved === 'light' || saved === 'dark') setTheme(saved);
} catch {}
$('manage-mining').onclick = () => { location.hash = 'mining'; };
$('theme').onclick = () => {
  const theme = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  setTheme(theme);
  try { localStorage.setItem('theme', theme); } catch {}
};

function setUnlocked(value) {
  const changed = unlocked !== value;
  unlocked = value;
  if (changed && !value) {
    clearTimeout(coolingSaveTimer);
    coolingRevision++;
    coolingEdited = false;
    setText('cooling-state', '');
    $('cooling-retry').hidden = true;
  }
  $('unlock').textContent = value ? 'Lock controls' : 'Unlock controls';
  renderControls();
  if (changed) {
    sessionRevision++;
    document.dispatchEvent(new Event('controlschange'));
  }
}

async function api(path, data, {signal} = {}) {
  const controller = data === undefined ? new AbortController() : null;
  if (controller && !dashboardVisible()) throw new DOMException('Dashboard is hidden', 'AbortError');
  if (controller) activeReads.add(controller);
  const timeout = new AbortController();
  const timer = setTimeout(() => timeout.abort(new DOMException('Controller request timed out', 'TimeoutError')), 8000);
  try {
    const signals = [timeout.signal, ...(controller ? [controller.signal] : []), ...(signal ? [signal] : [])];
    const response = await fetch(new URL(path, apiBase), {
      credentials: 'include', cache: 'no-store', signal: AbortSignal.any(signals),
      ...(data === undefined ? {} : {
        method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(data),
      }),
    });
    const body = await response.json().catch(error => {
      if (['AbortError', 'TimeoutError'].includes(error.name)) throw error;
      if (response.ok) throw Error('The controller returned an unreadable response.');
      return null;
    });
    if (!response.ok) {
      if (response.status === 401 && path !== 'login') setUnlocked(false);
      const error = new Error(body?.error || 'Request failed (HTTP ' + response.status + ').');
      error.status = response.status;
      throw error;
    }
    return body;
  } finally { clearTimeout(timer); if (controller) activeReads.delete(controller); }
}

function dismissFeedback(immediate = false) {
  clearTimeout(feedbackTimer);
  clearTimeout(feedbackHideTimer);
  if (immediate) $('feedback').hidden = true;
  else if (!$('feedback').hidden) {
    $('feedback').dataset.leaving = 'true';
    feedbackHideTimer = setTimeout(() => { $('feedback').hidden = true; }, 240);
  }
}
function showFeedback(message, tone) {
  clearTimeout(feedbackTimer);
  clearTimeout(feedbackHideTimer);
  $('feedback-message').textContent = message;
  $('feedback').dataset.tone = tone;
  $('feedback').dataset.leaving = 'false';
  $('feedback').hidden = false;
  feedbackTimer = setTimeout(dismissFeedback, tone === 'danger' ? 7000 : 4500);
}
$('feedback-dismiss').onclick = () => dismissFeedback();

function failureMessage(error) {
  return ['TimeoutError', 'TypeError'].includes(error.name)
    ? 'Could not reach the controller. Check the miner’s connection.'
    : error.message;
}

async function action(run, message) {
  if (busy) return;
  busy = true;
  dismissFeedback(true);
  renderControls();
  try {
    await run();
    if (message) showFeedback(message, 'positive');
  } catch (error) {
    showFeedback(failureMessage(error), 'danger');
  } finally {
    busy = false;
    renderControls();
    void refresh();
  }
}

function openLogin() {
  $('login-error').hidden = true;
  $('password').value = '';
  $('login').showModal();
}
$('unlock').onclick = () => {
  if (!unlocked) openLogin();
  else action(async () => {
    await api('logout', {});
    setUnlocked(false);
  }, 'Controls locked.');
};
$('cancel').onclick = () => $('login').close();
$('login').addEventListener('close', () => { $('password').value = ''; });
$('login-form').onsubmit = async event => {
  event.preventDefault();
  const submit = $('login-submit');
  if (submit.disabled) return;
  submit.disabled = true;
  $('login-error').hidden = true;
  try {
    await api('login', {password: $('password').value});
    setUnlocked(true);
    $('login').close();
    showFeedback('Controls unlocked.', 'positive');
  } catch (error) {
    $('login-error').textContent = failureMessage(error);
    $('login-error').hidden = false;
    $('password').focus();
  } finally {
    submit.disabled = false;
  }
};

function renderControls() {
  const available = unlocked && online && !busy;
  document.querySelectorAll('[data-requires-unlock]').forEach(control => {
    control.disabled = !available;
  });
  document.querySelectorAll('[data-lock-note]').forEach(note => { note.hidden = unlocked; });

  const s = lastStatus;
  const pending = Boolean(s?.performance?.adjusting);
  const automatic = Boolean(s?.performance?.automatic);
  $('algo').disabled = !available || pending || automatic;
  $('switch').disabled = $('algo').disabled || Number($('algo').value) === s?.algorithm;
  $('switch').textContent = s?.halted ? 'Save for resume' : 'Apply algorithm';
  $('algorithm-help').textContent = automatic ? 'Turn off automatic performance before changing algorithms.'
    : s?.halted ? 'You can save the next algorithm now. It takes effect when you resume mining.'
    : pending ? 'Automatic performance is adjusting. Please wait.' : '';
  $('automatic-tuning').disabled = !available || (!automatic && !s?.performance?.available);

}

function queueCoolingSave() {
  clearTimeout(coolingSaveTimer);
  coolingSaveTimer = setTimeout(saveCooling, Math.max(0, coolingEditedAt + 1000 - Date.now()));
}
async function saveCooling() {
  if (!coolingEdited || !unlocked || !online) return;
  if (busy || coolingSaving) {
    coolingSaveTimer = setTimeout(saveCooling, 250);
    return;
  }
  const target = Number($('chip-target').value);
  const revision = coolingRevision;
  if (target === lastStatus?.cooling?.target) {
    coolingEdited = false;
    setText('cooling-state', 'Saved');
    return;
  }
  coolingSaving = true;
  $('cooling-retry').hidden = true;
  setText('cooling-state', 'Saving…');
  $('cooling-state').dataset.tone = 'neutral';
  try {
    await api('cooling', {target});
    coolingSaveGeneration++;
    if (lastStatus?.cooling) lastStatus.cooling.target = target;
    if (revision === coolingRevision) {
      coolingEdited = false;
      setText('cooling-state', 'Saved');
      $('cooling-state').dataset.tone = 'positive';
      showFeedback('Cooling limit saved at ' + target + '°C.', 'positive');
    }
  } catch (error) {
    if (revision === coolingRevision) {
      setText('cooling-state', 'Limit was not saved. ' + failureMessage(error));
      $('cooling-state').dataset.tone = 'danger';
      $('cooling-retry').hidden = false;
      showFeedback(failureMessage(error), 'danger');
    }
  } finally {
    coolingSaving = false;
    if (revision !== coolingRevision && coolingEdited) queueCoolingSave();
    void refresh();
  }
}
$('chip-target').oninput = () => {
  coolingEdited = true;
  coolingRevision++;
  coolingEditedAt = Date.now();
  setText('chip-target-value', $('chip-target').value + '°C');
  setText('cooling-state', 'Waiting to save…');
  $('cooling-state').dataset.tone = 'neutral';
  $('cooling-retry').hidden = true;
  queueCoolingSave();
};
$('cooling-form').onsubmit = event => { event.preventDefault(); queueCoolingSave(); };
$('cooling-retry').onclick = () => { coolingEditedAt = 0; queueCoolingSave(); };
$('algo').onchange = () => { algorithmEdited = true; renderControls(); };

$('performance-form').onsubmit = event => event.preventDefault();
$('automatic-tuning').onchange = () => {
  const automatic = $('automatic-tuning').checked;
  performanceEdited = true;
  action(async () => {
    try {
      await api('performance', {automatic});
      performanceSaveGeneration++;
      if (lastStatus?.performance) lastStatus.performance = {...lastStatus.performance, automatic, adjusting: false,
        detail: automatic ? 'Waiting for stable mining' : 'Using SPECTRE defaults'};
    }
    finally { performanceEdited = false; }
    renderStatus();
  }, automatic ? 'Automatic performance enabled.' : 'SPECTRE defaults restored.');
};
$('algorithm-form').onsubmit = event => {
  event.preventDefault();
  const id = Number($('algo').value);
  const queued = Boolean(lastStatus?.halted);
  action(async () => {
    await api('algorithm', {id});
    // Drain a status read that may have started before the write, then fetch
    // fresh controller readback. Do not display the requested value as applied.
    if (statusRequest) await statusRequest;
    await refresh();
    algorithmEdited = false;
    renderStatus();
  }, algorithms[id] + (queued ? ' saved for the next resume.' : ' selected. The mining engine is restarting.'));
};
$('resume').onclick = () => action(() => api('resume', {}), 'Resume requested. Watch for fresh mining telemetry.');

function formatNumber(value, digits = 0) {
  return typeof value === 'number' && Number.isFinite(value)
    ? value.toLocaleString(undefined, {maximumFractionDigits: digits})
    : '—';
}

function formatMiningDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return '—';
  seconds = Math.floor(seconds);
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor(seconds / 3600) % 24;
  const minutes = Math.floor(seconds / 60) % 60;
  const parts = [];
  if (days) parts.push(days + 'd');
  if (hours || days) parts.push(hours + 'h');
  if (minutes || parts.length) parts.push(minutes + 'm');
  parts.push((seconds % 60) + 's');
  return parts.join(' ');
}

function setText(id, text) {
  if ($(id).textContent !== text) $(id).textContent = text;
}

function stopReason(message) {
  if (!message) return 'Check diagnostics before resuming.';
  if (message.includes('sensor error=') && !message.endsWith('sensor error=<nil>')) {
    return 'The controller could not verify a chip temperature.';
  }
  if (message.startsWith('Temperature safety stop')) return 'A temperature reading reached a limit or became invalid.';
  if (message.startsWith('Mining telemetry unavailable')) return 'The controller stopped receiving mining readings.';
  return message;
}

function renderStatus() {
  const s = lastStatus;
  const hasPools = poolView?.length;
  const hasTelemetry = online && s && !s.stale && !s.halted && s.device;
  const d = hasTelemetry ? s.device : {};
  const indicator = $('miner-state');
  let title = 'Connecting to miner';
  let detail = 'Waiting for the controller to respond.';
  let tone = 'neutral';
  let next = null;
  if (!online && hasConnected) {
    title = 'Controller offline';
    detail = 'Check the miner’s power and network connection.';
    tone = 'danger';
  } else if (online && s.halted && s.message === 'No mining pools configured; add a pool before resuming') {
    title = 'Waiting for pool setup';
    detail = 'Add your mining pool, then resume mining. Mining is paused until setup is complete.';
    tone = 'warning';
    next = ['#mining', 'Set up a pool'];
  } else if (online && s.halted) {
    title = 'Mining paused by a safety stop';
    detail = stopReason(s.message);
    if (hasPools === 0) detail += ' Add a mining pool before resuming.';
    tone = 'danger';
    next = hasPools === 0 ? ['#mining', 'Set up a pool'] : ['#diagnostics', 'Review safety stop'];
  } else if (online && s.startup === 'fan-calibration') {
    tone = 'warning';
    title = 'Calibrating fans';
    detail = 'The stock firmware checks fan speeds before starting XBT mining.';
  } else if (online && s.startup === 'first-readings') {
    tone = 'warning';
    title = 'Checking chip temperatures';
    detail = 'The XBT engine has started. Waiting for its first valid readings.';
  } else if (online && hasPools === 0) {
    title = 'No mining pools configured';
    detail = 'Add a pool and select the matching algorithm to start mining.';
    tone = 'warning';
    next = ['#mining', 'Set up mining'];
  } else if (online && !hasTelemetry) {
    title = 'Waiting for mining telemetry';
    detail = 'The controller is online. Check the pool connection if readings stay unavailable.';
    tone = 'warning';
    next = ['#diagnostics', 'Open diagnostics'];
  } else if (online) {
    title = d.hashrate > 0 ? 'Mining active' : 'Miner online';
    detail = !s.message || s.message === 'KD BOX safeguards active' ? 'Receiving live telemetry.' : s.message;
    tone = 'positive';
  }

  const compact = {
    'Mining paused by a safety stop': 'Safety stop',
    'Waiting for pool setup': 'Pool setup',
    'Checking chip temperatures': 'Starting XBT',
    'No mining pools configured': 'No pool',
    'Waiting for mining telemetry': 'Telemetry delayed',
  };
  setText('status-title', compact[title] || title);
  setText('status-tooltip-title', title);
  indicator.href = next ? next[0] : '#diagnostics';
  indicator.setAttribute('aria-label', title + '. ' + detail + (next ? ' ' + next[1] + '.' : ' View diagnostics.'));
  setText('status-detail', detail);
  $('miner-state').dataset.tone = tone;
  $('connection').dataset.tone = online ? 'positive' : hasConnected ? 'danger' : 'neutral';
  setText('connection', online ? 'Controller online' : hasConnected ? 'Offline' : 'Connecting');

  setText('hash', typeof d.hashrate === 'number' ? formatNumber(d.hashrate / 1000, 1) + ' GH/s' : '—');
  setText('hash-average', typeof d.av_hashrate === 'number' ? 'Average ' + formatNumber(d.av_hashrate / 1000, 1) + ' GH/s' : 'Average unavailable');
  setText('temp', typeof d.board_temp === 'number' ? formatNumber(d.board_temp, 1) + '°C' : '—');
  setText('chip-peak', d.peak_temp > 0 ? 'Hottest chip ' + formatNumber(d.peak_temp, 1) + '°C' : 'Hottest chip unavailable');
  setText('shares', formatNumber(d.accepted));
  setText('shares-rejected', typeof d.rejected === 'number' ? 'Rejected ' + formatNumber(d.rejected) : 'Rejected shares unavailable');
  const rpm = online && Array.isArray(s.fan_rpm) ? s.fan_rpm : [];
  const duty = online && Array.isArray(s.fan_percent) ? s.fan_percent : [];
  for (let i = 0; i < 2; i++) {
    setText('fan-' + (i + 1) + '-percent', typeof duty[i] === 'number' ? duty[i] + '%' : '—');
    setText('fan-' + (i + 1) + '-rpm', typeof rpm[i] === 'number' ? formatNumber(rpm[i]) + ' RPM' : 'RPM unavailable');
  }
  setText('overview-clock', typeof d.clock === 'number' && Number.isFinite(d.clock) && d.clock > 0
    ? formatNumber(d.clock) + ' MHz' : '—');
  setText('overview-voltage', typeof d.voltage === 'number' && Number.isFinite(d.voltage) && d.voltage > 0
    ? formatNumber(d.voltage * 1000) + ' mV core voltage' : 'Core voltage unavailable');
  if (s?.cooling) {
    if (!coolingEdited) {
      $('chip-target').value = s.cooling.target;
      setText('chip-target-value', s.cooling.target + '°C');
    }
  }
  setText('mining-session', hasTelemetry && d.hashrate > 0 && s.mining_session?.active
    ? formatMiningDuration(s.mining_session.elapsed_seconds) : '—');
  setText('error-count', d.nonces > 0 && typeof d.hwerrors === 'number'
    ? formatNumber(100 * d.hwerrors / d.nonces, 3) + '%' : '—');
  setText('live-tuning', d.clock ? 'Live clock ' + d.clock + ' MHz · ' + d.voltage + ' V' : 'Live clock and voltage unavailable.');
  const algorithm = online ? algorithms[s.algorithm] || '—' : '—';
  setText('active-algorithm', algorithm + (online && s.algorithm_queued ? ' · on resume' : ''));
  $('recovery').hidden = !(online && s.halted);
  setText('recovery-reason', stopReason(s?.message));
  setText('recovery-raw', s?.message || 'No details recorded.');
  if (s?.performance) {
    const p = s.performance;
    if (!performanceEdited) $('automatic-tuning').checked = p.automatic;
    setText('default-tuning', (p.default?.clock ?? '—') + ' MHz · ' + (p.default?.millivolts ?? '—') + ' mV');
    setText('performance-detail', s.halted && p.automatic ? 'Automatic performance will start when mining resumes.' : p.detail);
    setText('chip-target-help', '70–80°C. Saves automatically after one second. ' + (p.automatic ? 'Automatic performance runs both fans at 100%.' : 'Fans stay at or above 30% during mining.'));
  }
  if (s?.algorithm != null && !algorithmEdited) $('algo').value = s.algorithm;
  if (s?.software?.version) {
    const software = s.software;
    const revision = /^[a-f0-9]{7}$/.test(software.short_commit || '') ? software.short_commit : 'unversioned';
    setText('software-version', '· ' + software.version + ' · ' + revision + (software.dirty && revision !== 'unversioned' ? '-dirty' : ''));
    $('software-version').title = revision === 'unversioned' ? 'Source revision unavailable for this build.'
      : 'Source commit ' + software.commit + (software.dirty ? ' (uncommitted changes)' : '');
  }
  renderControls();
}

function refresh() {
  if (statusRequest) return statusRequest;
  const revision = sessionRevision;
  const coolingGeneration = coolingSaveGeneration;
  const performanceGeneration = performanceSaveGeneration;
  statusRequest = (async () => {
    try {
      const status = await api('status');
      if (!status || typeof status.halted !== 'boolean') throw Error('Invalid controller status.');
      if (coolingGeneration !== coolingSaveGeneration && lastStatus?.cooling) status.cooling = lastStatus.cooling;
      if (performanceGeneration !== performanceSaveGeneration && lastStatus?.performance) status.performance = lastStatus.performance;
      lastStatus = status;
      online = true;
      hasConnected = true;
      if (revision === sessionRevision && typeof status.controls_unlocked === 'boolean') {
        setUnlocked(status.controls_unlocked);
      }
    } catch (error) {
      if (error.name === 'AbortError') return;
      online = false;
      hasConnected = true;
    } finally {
      statusRequest = null;
      renderStatus();
      document.dispatchEvent(new Event('minerstatus'));
    }
  })();
  return statusRequest;
}

function chartElement(name, attrs, value) {
  const node = document.createElementNS('http://www.w3.org/2000/svg', name);
  for (const [key, attr] of Object.entries(attrs)) node.setAttribute(key, attr);
  if (value !== undefined) node.textContent = value;
  return node;
}

let historyRequest = null;
let historySamples = [];
let historyMessage = 'No hashrate samples yet. History appears once the miner reports activity.';
let historyOverride = null;
let historyOverrideTimer;
const historyHours = [1, 3, 6, 12, 24];

function renderHistory() {
  const now = Date.now() / 1000;
  if (historyOverride && Date.now() >= historyOverride.until) historyOverride = null;
  const available = historySamples.length ? Math.max(0, now - historySamples[0].at) / 3600 : 0;
  const hours = historyOverride?.hours || historyHours.find(hours => available <= hours) || 24;
  const start = now - hours * 3600;
  const samples = historySamples.filter(sample => sample.at >= start && sample.at <= now);
  document.querySelectorAll('#chart-ranges button').forEach(button => {
    button.setAttribute('aria-pressed', String(historyOverride ? Number(button.dataset.hours) === hours : button.dataset.hours === 'auto'));
  });
  setText('chart-description', 'GH/s · Last ' + (hours === 1 ? 'hour' : hours + ' hours'));
  setText('chart-start', hours + (hours === 1 ? ' hour ago' : ' hours ago'));
  $('chart-empty').textContent = samples.length ? '' : historyMessage;
  $('chart-empty').hidden = Boolean(samples.length);
  $('chart').toggleAttribute('hidden', !samples.length);
  $('chart-axis').hidden = !samples.length;
  $('chart').setAttribute('aria-label', 'Hashrate history in gigahashes per second over the last ' + hours + ' hours');
  if (!samples.length) { $('chart').replaceChildren(); return; }
  const top = Math.max(1000, ...samples.map(sample => sample.rate));
  const grid = [0, .5, 1].flatMap(ratio => {
    const y = 160 - ratio * 143;
    return [
      chartElement('line', {x1: 48, x2: 610, y1: y, y2: y}),
      chartElement('text', {x: 0, y: y + 5}, formatNumber(top * ratio / 1000, 1)),
    ];
  });
  const segments = [];
  let segment = [];
  for (let i = 0; i < samples.length; i++) {
    if (i && samples[i].at - samples[i - 1].at > 90) {
      segments.push(segment);
      segment = [];
    }
    segment.push([48 + (samples[i].at - start) * 562 / (hours * 3600), 160 - samples[i].rate / top * 143]);
  }
  segments.push(segment);
  const marks = segments.map(points => points.length === 1
    ? chartElement('circle', {cx: points[0][0], cy: points[0][1], r: 3})
    : chartElement('polyline', {points: points.map(point => point.join(',')).join(' ')}));
  $('chart').replaceChildren(...grid, ...marks);
}
$('chart-ranges').onclick = event => {
  const button = event.target.closest('button[data-hours]');
  if (!button) return;
  clearTimeout(historyOverrideTimer);
  historyOverride = button.dataset.hours === 'auto' ? null : {hours: Number(button.dataset.hours), until: Date.now() + 5 * 60000};
  if (historyOverride) historyOverrideTimer = setTimeout(() => { historyOverride = null; renderHistory(); }, 5 * 60000);
  renderHistory();
};
function history() {
  if (historyRequest) return historyRequest;
  historyRequest = (async () => {
    try {
      const raw = await api('history');
      if (!Array.isArray(raw?.samples)) throw Error('Invalid history');
      const now = Date.now() / 1000;
      historySamples = raw.samples.slice(-1440).filter(sample => Number.isFinite(sample.at) && sample.at > now - 86400 && sample.at <= now && Number.isFinite(sample.rate) && sample.rate >= 0).sort((a, b) => a.at - b.at);
      historyMessage = 'No hashrate samples yet. History appears once the miner reports activity.';
    } catch (error) {
      if (error.name === 'AbortError') return;
      historyMessage = historySamples.length ? 'No samples in this time range.' : 'Hashrate history is unavailable.';
    } finally { historyRequest = null; }
    renderHistory();
  })();
  return historyRequest;
}

showPage();
window.addEventListener('hashchange', () => { showPage(); if (pageName() === 'overview') void history(); });
renderControls();
registerPolling(refresh, 3000);
registerPolling(() => { if (pageName() === 'overview') return history(); }, 15000);
