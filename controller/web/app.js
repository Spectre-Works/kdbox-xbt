const $=id=>document.getElementById(id);let initialized=false,busy=false;

let savedTheme;try{savedTheme=localStorage.getItem('theme')}catch(e){}
document.documentElement.dataset.theme=['light','dark'].includes(savedTheme)?savedTheme:(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light');

$('theme').onclick=()=>{let t=document.documentElement.dataset.theme==='dark'?'light':'dark';document.documentElement.dataset.theme=t;try{localStorage.setItem('theme',t)}catch(e){}};



$('unlock').onclick=()=>$('login').showModal();$('cancel').onclick=()=>$('login').close();$('fan').oninput=()=>$('fan-value').textContent=$('fan').value+'%';

async function api(path,data){let r=await fetch(`http://${location.hostname}:8088/api/`+path,{credentials:'include',method:data===undefined?'GET':'POST',headers:{'Content-Type':'application/json'},body:data===undefined?undefined:JSON.stringify(data)});let v=await r.json();if(!r.ok){if(r.status===401)$('unlock').textContent='Unlock controls';const error=Error(v.error||'Request failed');error.status=r.status;throw error}if(!['status','history','logout','pool-status'].includes(path))$('unlock').textContent='Controls unlocked';return v}

async function action(fn){if(busy)return;busy=true;try{await fn();await refresh()}catch(e){$('notice').textContent=e.message}finally{busy=false}}

$('login-form').onsubmit=e=>{e.preventDefault();action(async()=>{await api('login',{password:$('password').value});$('password').value='';$('login').close();$('unlock').textContent='Controls unlocked';if(typeof loadPage==='function')await loadPage()})};

$('tuning').onsubmit=e=>{e.preventDefault();action(()=>api('trial',{clock:Number($('clock').value),millivolts:Number($('voltage').value),fan:Number($('fan').value),mode:$('mode').value}))};

$('confirm').onclick=()=>action(()=>api('confirm',{}));$('revert').onclick=()=>action(()=>api('revert',{}));$('switch').onclick=()=>action(()=>api('algorithm',{id:Number($('algo').value)}));

$('resume').onclick=()=>action(()=>api('resume',{}));

async function refresh(){try{const s=await api('status'),d=s.device||{};if(s.software?.version)$('software-version').textContent='· '+s.software.version;if(!busy)$('notice').textContent=(s.stale?'Telemetry is updating. ':'')+s.message;

 $('resume').hidden=!s.halted;$('live-tuning').textContent=d.clock?`Running: ${d.clock} MHz · ${d.voltage} V`:'Waiting for live ASIC settings';

 $('hash').textContent=d.hashrate==null?'—':`${(d.hashrate/1000).toFixed(1)} / ${(d.av_hashrate/1000).toFixed(1)}`;$('temp').textContent=(d.temp||'—')+(d.peak_temp?` · peak ${d.peak_temp}°C`:'');$('shares').textContent=`${d.accepted??'—'} / ${d.rejected??'—'}`;$('errors').textContent=d.nonces>0?`${(100*d.hwerrors/d.nonces).toFixed(3)}%`:'—';$('error-count').textContent=d.nonces>0?`${d.hwerrors} invalid / ${d.nonces.toLocaleString()} nonces`:'Waiting for nonce counters';$('fans').textContent='Fans: '+(d.fanspeed||'waiting for telemetry');

 $('trial').hidden=!s.pending;$('countdown').textContent=`Trial: ${s.seconds_left}s remaining`;if(!initialized&&s.approved){$('clock').value=s.approved.clock;$('voltage').value=s.approved.millivolts;$('fan').value=s.approved.fan;$('mode').value=s.approved.mode;$('fan').oninput();if(s.algorithm!=null)$('algo').value=s.algorithm;initialized=true}

 }catch(e){$('notice').textContent='Miner disconnected. Recovery is designed to run on the device, but a frozen or unpowered miner cannot complete it. Inspect the miner before retrying.'}}

async function history(){try{const raw=await api('history');const a=Array.isArray(raw)?raw:[];if(!a.length){$('chart').innerHTML='<text x="20" y="90">History is warming up…</text>';return}const values=a.map(x=>Number(x)||0),top=Math.max(...values,1);const points=values.map((v,i)=>`${20+i*960/Math.max(values.length-1,1)},${160-v/top*135}`).join(' ');$('chart').innerHTML=`<text x="20" y="15">${(top/1000).toFixed(0)} GH/s</text><polyline points="${points}"/>`}catch(e){$('chart').innerHTML='<text x="20" y="90">History temporarily unavailable</text>'}}

refresh();history();setInterval(refresh,3000);setInterval(history,15000);

