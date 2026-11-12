"use strict";
const token =
  location.hash.slice(1) ||
  sessionStorage.getItem("kdbox-installer-token") ||
  "";
if (token) sessionStorage.setItem("kdbox-installer-token", token);
history.replaceState(null, "", "/");
const content = document.querySelector("#content"),
  status = document.querySelector("#status"),
  error = document.querySelector("#error");
let state = {},
  rendered = "",
  selected = "",
  localError = "";
const esc = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
async function api(path, data) {
  const r = await fetch("/api/" + path, {
    method: data ? "POST" : "GET",
    headers: {
      "X-Installer-Token": token,
      ...(data ? { "Content-Type": "application/json" } : {}),
    },
    body: data ? JSON.stringify(data) : undefined,
  });
  const result = await r.json();
  if (!r.ok) throw Error(result.error || "Request failed");
  return result;
}
function showError(message) {
  error.textContent = message;
  error.hidden = !message;
}
async function action(name, data) {
  try {
    localError = "";
    showError("");
    await api(name, data);
    await poll();
    return true;
  } catch (e) {
    localError = e.message;
    showError(localError);
    return false;
  }
}
function discovery() {
  return `${state.payload_ready ? "" : '<p role="alert" class="prerequisite-note"><strong>This folder is missing a complete, verified controller payload.</strong> Use a prepared product ZIP, or build this source checkout as described in the setup guide. Existing installations can still be connected for recovery.</p>'}<h2>Find your miner</h2><p>Connect this computer and your original KD BOX to the same private network. We'll check compatibility before any installation.</p><div class="prerequisite-note"><p><strong>Start with stock Goldshell 2.2.1.</strong> <a href="${esc(state.firmware_url)}" target="_blank" rel="noopener noreferrer">Get the firmware from Goldshell ↗</a></p><p>Prepare the stock base using the setup guide. This installer does not download or flash firmware.</p></div><label><input id="lan" type="checkbox">I own or administer this local network.</label><div class="actions"><button id="scan" type="button">Find KD BOX miners</button></div><small>Read-only search of ${esc(state.networks.join(", ") || "no detected local subnet")}. Searching does not change miner settings.</small><details><summary>Miner not found? Look up an IP address</summary><form id="lookup-form"><label for="manual">Miner's private IPv4 address</label><input id="manual" placeholder="192.168.1.100" inputmode="decimal" autocomplete="off"><div class="actions"><button id="lookup" type="submit" class="secondary">Look up miner</button></div><p>Guest Wi-Fi, VPNs or network isolation can block discovery. Use the IP from your router if the miner is outside this computer's local subnet.</p></form></details>`;
}
function bindDiscovery() {
  document
    .querySelector("#scan")
    ?.addEventListener("click", () =>
      action("scan", { authorized: document.querySelector("#lan").checked }),
    );
  document.querySelector("#lookup-form")?.addEventListener("submit", (event) => {
    event.preventDefault();
    action("scan", {authorized: document.querySelector("#lan").checked, host: document.querySelector("#manual").value});
  });
}
function recovery() {
  if (!state.backups.length) return "";
  return `<details><summary>Restore a recovery backup</summary><p>Restores this miner's saved files and restarts it. The backup identity and every file are checked first. Backups are private and are not full firmware images.</p><label for="backup">Backup on this computer</label><select id="backup">${state.backups.map((b) => `<option value="${esc(b.id)}">${esc(b.label)}</option>`).join("")}</select><label class="acknowledgement"><input id="restoreAck" type="checkbox">I want to restore this backup to the connected miner and restart it.</label><button id="restore" class="secondary">Restore backup & restart</button></details>`;
}
function render() {
  const stage = state.stage;
  const activeStep = ({welcome: 0, select: 0, searching: 0, connecting: 1, identity: 1, review: 2, repair: 2, installing: 2, done: 3, existing: 3}[stage] ?? (state.backup ? 2 : 0));
  document.querySelectorAll(".wizard-progress li").forEach((li, index) => {
    li.classList.toggle("complete", index < activeStep);
    if (index === activeStep) li.setAttribute("aria-current", "step");
    else li.removeAttribute("aria-current");
  });
  document
    .querySelectorAll(".wizard-progress li")
    .forEach((li, i) =>
      li.classList.toggle(
        "active",
        i ===
          activeStep,
      ),
    );
  if (stage === "welcome" || stage === "error") {
    content.innerHTML =
      stage === "error" && state.backup
        ? `<h2>Installation needs attention</h2><p>Keep the miner powered. Do not reboot or repeat installation. Inspect the dashboard and use your recovery backup if needed.</p><a class="button" href="http://${esc(state.host)}/" target="_blank" rel="noopener noreferrer">Open miner dashboard</a><p>Private recovery backup:</p><code>${esc(state.backup)}</code><details><summary>Reconnect for recovery</summary>${discovery()}</details>`
        : discovery();
    bindDiscovery();
  } else if (stage === "searching") {
    content.innerHTML =
      "<h2>Searching your local network…</h2><p>This usually takes a few seconds. Leave this window open.</p>";
  } else if (stage === "select") {
    content.innerHTML = `<h2>Select your KD BOX</h2>${state.devices.map((d, i) => `<label class="device"><input type="radio" name="miner" value="${esc(d.host)}" ${d.compatible ? "" : "disabled"} ${d.compatible && state.devices.filter((x) => x.compatible).length === 1 ? "checked" : ""}><span class="device-details"><strong>${esc(d.host)}</strong><span class="device-model">${esc(d.model)}</span><small>Firmware ${esc(d.firmware)} · ${esc(d.board)}${d.compatible ? "" : " · Prepare stock 2.2.1 before connecting."}</small></span></label>`).join("")}<form id="credentials" class="credential-panel" ${state.devices.some((d) => d.compatible) ? "" : "hidden"}><h3>Connect to your miner</h3><p>Use your current passwords. Neither will be changed.</p><label><input id="selectedAck" type="checkbox">I confirm the selected IP belongs to my miner. The installer may start SSH through its dashboard maintenance endpoint.</label><label for="web">Current dashboard password</label><input id="web" type="password" placeholder="123456789" autocomplete="off"><small>Goldshell's default is 123456789. Enter your current password.</small><details><summary>Changed the SSH root password?</summary><label for="ssh">Current SSH root password</label><input id="ssh" type="password" placeholder="Leave blank to use the stock 2.2.1 default" autocomplete="off"><small>Usually you can leave this blank. The SSH password differs from the dashboard password. Neither will be changed.</small></details><div class="actions"><button id="connect" type="submit">Connect & check compatibility</button></div></form><details><summary>Search again or look up another IP</summary>${discovery()}</details>`;
    bindDiscovery();
    document.querySelector("#credentials")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      selected =
        document.querySelector("input[name=miner]:checked")?.value || "";
      const web = document.querySelector("#web"),
        ssh = document.querySelector("#ssh");
      if (
        await action("connect", {
          host: selected,
          authorized: document.querySelector("#selectedAck").checked,
          web_password: web.value,
          ssh_password: ssh.value,
        })
      ) {
        web.value = "";
        ssh.value = "";
      }
    });
  } else if (stage === "identity") {
    content.innerHTML = `<h2>Confirm the miner's SSH identity</h2><p>You selected <strong>${esc(state.identity.host)}</strong>. This is its SSH fingerprint:</p><pre class="fingerprint">${esc(state.identity.fingerprint)}</pre><p>Compare it with a trusted prior connection if available. A first connection establishes trust; discovery alone cannot authenticate a miner.</p><button id="identityYes">Trust this miner & continue</button><button id="identityNo" class="secondary">Cancel connection</button>`;
    document.querySelector("#identityYes").onclick = () =>
      action("identity", { accept: true });
    document.querySelector("#identityNo").onclick = () =>
      action("identity", { accept: false });
  } else if (stage === "connecting") {
    content.innerHTML =
      "<h2>Checking your miner…</h2><p>Verifying the running firmware, board, CPU and stock engine. Keep this window open.</p>";
  } else if (stage === "review" || stage === "repair") {
    const repair = stage === "repair";
    content.innerHTML = `<h2>${repair ? "Repair previous installation on" : "Ready to install on"} ${esc(state.host)}</h2>${repair ? "<p>The previous mod is incomplete or is not responding. A stock firmware reflash can leave its settings behind while removing the dashboard and startup hook. Repair restores the application and keeps saved passwords, pools, cooling, performance and algorithm settings. An existing safety stop must be inspected before proceeding.</p>" : ""}<dl><dt>Model</dt><dd>${esc(state.verified.model)}</dd><dt>Stock firmware</dt><dd>${esc(state.verified.firmware)}</dd><dt>Board</dt><dd>${esc(state.verified.mcbversion)}</dd><dt>Passwords & pools</dt><dd>Existing passwords and pool settings are preserved.</dd></dl><p>${repair ? "A saved algorithm is retained during repair." : "New installations select Bitcoin (XBT) automatically. Existing pools are kept; verify they support XBT."}</p><p>A private recovery backup will be saved automatically before application files are installed. This does not flash firmware or replace the bootloader.</p><label class="acknowledgement"><input id="installAck" type="checkbox">I own or am authorized to modify this miner, have reliable power, cooling and recovery access, and accept the risk of interruption, damage or a bricked miner. Recovery is not guaranteed.</label><button id="install">${repair ? "Back up & repair KD BOX XBT" : "Back up & install KD BOX XBT"}</button>${recovery()}`;
    if (!state.payload_ready) {
      content.innerHTML = `<h2>Previous installation needs repair</h2><p>Download and extract a complete installer package to repair the dashboard and startup files. Saved settings have not been changed.</p>${recovery()}`;
    }
    const install = document.querySelector("#install");
    if (install) install.onclick = () =>
      action("install", {
        authorized: document.querySelector("#installAck").checked,
      });
  } else if (stage === "installing") {
    content.innerHTML = `<h2>Working on your miner…</h2><p>Leave the installer running and keep the miner powered. Do not restart or repeat installation.</p>${state.backup ? `<p>Private recovery backup:</p><code>${esc(state.backup)}</code>` : ""}`;
  } else if (stage === "existing" || stage === "done") {
    content.innerHTML = `<h2>${stage === "existing" ? "Already installed" : "All done"}</h2><a class="button" href="http://${esc(state.host)}/" target="_blank" rel="noopener noreferrer">Open miner dashboard</a>${stage === "existing" || state.operation === "repair" || state.operation === "restore" ? "<p>Your saved settings are kept. Open the dashboard with your unchanged password, check the selected algorithm and pool, and supervise startup. Verify all chip temperatures and accepted shares. Inspect any safety stop before resuming.</p>" : "<p>Unlock Mining with your unchanged dashboard password. Bitcoin (XBT) is selected by default. Check your XBT pool and payout/worker, then verify accepted shares and all chip temperatures while supervising the miner.</p>"}${state.backup ? `<p>Keep this recovery backup private:</p><code>${esc(state.backup)}</code>` : ""}${stage === "existing" ? recovery() : ""}`;
  }
  document
    .querySelector("#restore")
    ?.addEventListener("click", () =>
      action("restore", {
        authorized: document.querySelector("#restoreAck").checked,
        backup: document.querySelector("#backup").value,
      }),
    );
}
async function poll() {
  try {
    state = await api("state");
    if (status.textContent !== state.message) status.textContent = state.message;
    status.dataset.busy = String(state.busy && state.stage !== "identity");
    content.setAttribute("aria-busy", String(state.busy && state.stage !== "identity"));
    document.querySelector("#installer-version").textContent = "Version " + state.version;
    showError(state.error || localError);
    document.querySelector("#close").disabled = state.busy;
    const key =
      state.stage +
      JSON.stringify(state.devices) +
      JSON.stringify(state.identity) +
      String(state.backup);
    if (key !== rendered) {
      rendered = key;
      render();
    }
  } catch (e) {
    showError(e.message);
  }
}
document.querySelector("#close").onclick = async () => {
  try {
    await api("close", {});
    sessionStorage.removeItem("kdbox-installer-token");
    content.innerHTML =
      "<h2>Installer closed</h2><p>You can close this browser tab.</p>";
    clearInterval(timer);
  } catch (e) {
    showError(e.message);
  }
};
window.addEventListener("beforeunload", (event) => {
  if (state.busy) {
    event.preventDefault();
    event.returnValue = "";
  }
});
let theme = "dark";
try { theme = localStorage.getItem("kdbox-installer-theme") === "light" ? "light" : "dark"; } catch {}
function applyTheme() {
  document.documentElement.dataset.theme = theme;
  document.querySelector("#theme").textContent = theme === "dark" ? "Light mode" : "Dark mode";
}
document.querySelector("#theme").onclick = () => {
  theme = theme === "dark" ? "light" : "dark";
  try { localStorage.setItem("kdbox-installer-theme", theme); } catch {}
  applyTheme();
};
applyTheme();
poll();
const timer = setInterval(poll, 700);
