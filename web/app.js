// Net Tool Box 前端逻辑（原生 JS，无依赖）

async function api(path) {
  const resp = await fetch(path);
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({ error: '请求失败' }));
    throw new Error(err.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

function escapeHtml(s) {
  return String(s ?? '').replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[c]));
}

// ---- 本地 IP ----
async function loadLocal() {
  const el = document.getElementById('local-body');
  try {
    const d = await api('/api/localip');
    const upIfaces = (d.interfaces || []).filter(i => i.up && !i.loopback);
    let html = '';
    for (const iface of upIfaces) {
      const addrs = iface.addrs.filter(a => !a.isLoopback && !a.isLinkLocal);
      if (!addrs.length) continue;
      html += `<div style="margin:8px 0"><b>${escapeHtml(iface.name)}</b> <span class="muted">(${escapeHtml(iface.mac || '无 MAC')})</span></div>`;
      html += `<table><tr><th>类型</th><th>地址</th><th>属性</th></tr>`;
      for (const a of addrs) {
        const tag = a.isPrivate ? '<span class="badge info">私网</span>' : '<span class="badge warn">公网</span>';
        html += `<tr><td>${a.version}</td><td class="mono">${escapeHtml(a.ip)}</td><td>${tag} <span class="muted">${escapeHtml(a.cidr)}</span></td></tr>`;
      }
      html += `</table>`;
    }
    if (!html) html = '<div class="muted">未检测到有效网络接口</div>';
    html += `<div style="margin-top:10px" class="muted">默认网关：<span class="ip">${escapeHtml(d.gateway || '未检测到')}</span></div>`;
    if (d.dns && d.dns.length) {
      html += `<div class="muted">DNS：<span class="ip">${d.dns.map(escapeHtml).join('，')}</span></div>`;
    }
    el.innerHTML = html;
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">获取失败：${escapeHtml(e.message)}</div>`;
  }
}

// ---- 公网 IP ----
async function loadPublic() {
  const el = document.getElementById('public-body');
  try {
    const d = await api('/api/publicip');
    let html = '';
    if (d.ipv4) html += `<div>公网 IPv4：<span class="ip">${escapeHtml(d.ipv4)}</span></div>`;
    else html += `<div class="muted">公网 IPv4：未检测到</div>`;
    if (d.ipv6) html += `<div>公网 IPv6：<span class="ip">${escapeHtml(d.ipv6)}</span></div>`;
    else html += `<div class="muted">公网 IPv6：未检测到</div>`;

    html += `<table style="margin-top:10px"><tr><th>来源</th><th>结果</th><th>延迟</th></tr>`;
    for (const e of (d.entries || [])) {
      const status = e.ok
        ? `<span class="badge good">${e.version}</span> <span class="ip">${escapeHtml(e.ip)}</span>`
        : `<span class="badge bad">失败</span> <span class="muted">${escapeHtml(e.error)}</span>`;
      html += `<tr><td>${escapeHtml(e.source)}</td><td>${status}</td><td class="mono">${e.ok ? e.latencyMs + 'ms' : '-'}</td></tr>`;
    }
    html += `</table>`;
    el.innerHTML = html;
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">获取失败：${escapeHtml(e.message)}</div>`;
  }
}

// ---- NAT / CGNAT ----
async function runNAT() {
  const btn = document.getElementById('btn-nat');
  const el = document.getElementById('nat-body');
  btn.disabled = true;
  btn.textContent = '检测中…';
  el.innerHTML = '<div class="loading">正在检测（STUN / UPnP / 公网 IP），请稍候…</div>';
  try {
    const d = await api('/api/nat');
    let cls = 'info';
    if (d.hasPublicIP) cls = 'good';
    else if (d.isCGNAT || d.multiNAT) cls = 'bad';
    else if (d.ipv6Available) cls = 'warn';

    let html = `<div class="verdict ${cls}">${escapeHtml(d.summary)}</div>`;
    html += `<ul class="details">`;
    for (const line of (d.details || [])) html += `<li>${escapeHtml(line)}</li>`;
    html += `</ul>`;
    el.innerHTML = html;
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">检测失败：${escapeHtml(e.message)}</div>`;
  } finally {
    btn.disabled = false;
    btn.textContent = '开始检测';
  }
}

// ---- 本地端口 ----
async function checkLocalPort() {
  const port = document.getElementById('local-port').value.trim();
  const el = document.getElementById('local-port-result');
  if (!port) { el.innerHTML = '<div class="muted">请输入端口号</div>'; return; }
  try {
    const d = await api('/api/port/local?port=' + encodeURIComponent(port));
    el.innerHTML = `<div class="result-box ${d.inUse ? 'bad' : 'good'}">端口 ${d.port}：${escapeHtml(d.message)}</div>`;
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">${escapeHtml(e.message)}</div>`;
  }
}

// ---- 远端端口 ----
async function checkRemotePort() {
  const host = document.getElementById('remote-host').value.trim();
  const port = document.getElementById('remote-port').value.trim();
  const el = document.getElementById('remote-port-result');
  if (!host || !port) { el.innerHTML = '<div class="muted">请输入主机和端口</div>'; return; }
  try {
    const d = await api('/api/port/remote?host=' + encodeURIComponent(host) + '&port=' + encodeURIComponent(port));
    if (d.open) {
      el.innerHTML = `<div class="result-box good">${escapeHtml(host)}:${d.port} 已开放（${d.latencyMs} ms）</div>`;
    } else {
      el.innerHTML = `<div class="result-box bad">${escapeHtml(host)}:${d.port} 不可达：${escapeHtml(d.error)}</div>`;
    }
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">${escapeHtml(e.message)}</div>`;
  }
}

// ---- 列出监听端口 ----
async function listPorts() {
  const el = document.getElementById('list-ports-result');
  el.innerHTML = '<div class="loading">正在获取…</div>';
  try {
    const d = await api('/api/port/list');
    const ports = d.ports || [];
    if (!ports.length) { el.innerHTML = '<div class="muted">未检测到监听端口</div>'; return; }
    let html = `<table><tr><th>协议</th><th>端口</th><th>地址</th><th>PID</th></tr>`;
    for (const p of ports) {
      html += `<tr><td>${escapeHtml(p.proto)}</td><td class="mono">${p.port}</td><td class="mono">${escapeHtml(p.addr)}</td><td class="mono">${escapeHtml(p.pid || '-')}</td></tr>`;
    }
    html += `</table>`;
    el.innerHTML = html;
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">获取失败：${escapeHtml(e.message)}</div>`;
  }
}

// ---- 本地网络链路 ----
async function runTrace() {
  const btn = document.getElementById('btn-trace');
  const el = document.getElementById('trace-body');
  btn.disabled = true;
  btn.textContent = '追踪中…（约需 10~30 秒）';
  el.innerHTML = '<div class="loading">正在追踪路由链路，请稍候…</div>';
  try {
    const d = await api('/api/trace');
    renderTrace(d, el);
  } catch (e) {
    el.innerHTML = `<div class="result-box bad">追踪失败：${escapeHtml(e.message)}</div>`;
  } finally {
    btn.disabled = false;
    btn.textContent = '开始追踪';
  }
}

function renderTrace(d, el) {
  let cls = 'info';
  if (d.cgnatDetected) cls = 'bad';
  else if (d.natLayers >= 2) cls = 'warn';
  else if (d.natLayers === 1) cls = 'good';

  let html = `<div class="verdict ${cls}">${escapeHtml(d.summary)}</div>`;
  html += `<div class="muted" style="margin:8px 0 2px">目标 <span class="ip">${escapeHtml(d.target)}</span> · 本机 <span class="ip">${escapeHtml(d.sourceIP)}</span> · 网关 <span class="ip">${escapeHtml(d.gateway || '未检测到')}</span></div>`;

  html += `<div class="trace-chain">`;
  const nodes = (d.hops || []);
  for (let i = 0; i < nodes.length; i++) {
    const h = nodes[i];
    let icon, label;
    switch (h.type) {
      case 'local':   icon = '🖥️'; label = '本机'; break;
      case 'private': icon = '🔀'; label = '内网跳（路由器/光猫）'; break;
      case 'cgnat':   icon = '⚠️'; label = '运营商 NAT（CGNAT）'; break;
      case 'public':  icon = '🌐'; label = '公网出口'; break;
      default:        icon = '❔'; label = '无响应（被隐藏）'; break;
    }
    const gw = h.isGateway ? ' <span class="badge info">网关</span>' : '';
    if (h.type === 'timeout') {
      html += `<div class="trace-node timeout"><div class="trace-hop">#${h.hop}</div><div class="trace-label">${icon} ${escapeHtml(label)}</div></div>`;
    } else {
      html += `<div class="trace-node ${h.type}"><div class="trace-hop">#${h.hop}</div><div class="trace-label">${icon} ${escapeHtml(label)}${gw}</div><div class="trace-ip">${escapeHtml(h.ip)}</div></div>`;
    }
    if (i < nodes.length - 1) {
      html += `<div class="trace-arrow">↓</div>`;
    }
  }
  html += `</div>`;
  el.innerHTML = html;
}

// 页面加载时自动获取本地 IP 与公网 IP
document.addEventListener('DOMContentLoaded', () => {
  loadLocal();
  loadPublic();
});
