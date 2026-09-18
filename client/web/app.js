// Tab switching
document.querySelectorAll('.tab').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
    if (btn.dataset.tab === 'history') loadMessages();
    if (btn.dataset.tab === 'settings') loadSettings();
  });
});

// Status polling
let isConnected = false;
async function pollStatus() {
  try {
    const r = await fetch('/api/status');
    const s = await r.json();
    const bar = document.getElementById('status-bar');
    const text = document.getElementById('status-text');
    const server = document.getElementById('status-server');
    if (s.connected) {
      bar.className = 'status-bar connected';
      text.textContent = '已连接';
    } else {
      bar.className = 'status-bar disconnected';
      text.textContent = '断开';
    }
    server.textContent = s.server;
    isConnected = s.connected;
  } catch (e) {
    // ignore
  }
}
setInterval(pollStatus, 3000);
pollStatus();

// Message list
let currentPage = 1;
let totalPages = 1;

async function loadMessages() {
  try {
    const r = await fetch('/api/messages?page=' + currentPage + '&limit=20');
    const data = await r.json();
    const list = document.getElementById('message-list');
    const totalEl = document.getElementById('total-count');
    totalEl.textContent = '共 ' + data.total + ' 条';
    totalPages = Math.ceil(data.total / data.limit) || 1;

    if (!data.messages || data.messages.length === 0) {
      list.innerHTML = '<div class="empty">📭 暂无验证码记录</div>';
    } else {
      list.innerHTML = data.messages.map(m => `
        <div class="message-card">
          <div class="code">${esc(m.sms_code)}</div>
          <div class="meta">
            <div class="msg">${m.title ? esc(m.title) + (m.msg ? '：' : '') + esc(m.msg) : esc(m.msg)}</div>
            <div class="time">${esc(m.received_at)}</div>
          </div>
          <button class="btn-copy" onclick="copyCode('${escAttr(m.sms_code)}', this)">📋 复制</button>
        </div>
      `).join('');
    }
    renderPagination(data.total, data.limit);
  } catch (e) {
    document.getElementById('message-list').innerHTML = '<div class="empty">加载失败</div>';
  }
}

function renderPagination(total, limit) {
  const pag = document.getElementById('pagination');
  if (total <= limit) { pag.innerHTML = ''; return; }
  pag.innerHTML = `
    <button ${currentPage <= 1 ? 'disabled' : ''} onclick="goPage(${currentPage-1})">上一页</button>
    <span class="page-info">${currentPage} / ${totalPages}</span>
    <button ${currentPage >= totalPages ? 'disabled' : ''} onclick="goPage(${currentPage+1})">下一页</button>
  `;
}

function goPage(p) {
  currentPage = p;
  loadMessages();
}

document.getElementById('btn-refresh').addEventListener('click', () => { currentPage = 1; loadMessages(); });

document.getElementById('btn-clear').addEventListener('click', async () => {
  if (!confirm('确定要清空所有历史记录吗？')) return;
  await fetch('/api/messages', { method: 'DELETE' });
  currentPage = 1;
  loadMessages();
});

async function copyCode(code, btn) {
  try {
    await navigator.clipboard.writeText(code);
    btn.textContent = '✓ 已复制';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = '📋 复制'; btn.classList.remove('copied'); }, 1500);
  } catch (e) {
    // Fallback
    const ta = document.createElement('textarea');
    ta.value = code;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    btn.textContent = '✓ 已复制';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = '📋 复制'; btn.classList.remove('copied'); }, 1500);
  }
}

// Settings
async function loadSettings() {
  try {
    const r = await fetch('/api/settings');
    const cfg = await r.json();
    document.getElementById('cfg-host').value = cfg.server_host || '127.0.0.1';
    document.getElementById('cfg-ws-port').value = cfg.ws_port || 9091;
    document.getElementById('cfg-token').value = cfg.auth_token || '';
  } catch (e) { /* ignore */ }
}

document.getElementById('settings-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const cfg = {
    server_host: document.getElementById('cfg-host').value.trim(),
    ws_port: parseInt(document.getElementById('cfg-ws-port').value) || 9091,
    auth_token: document.getElementById('cfg-token').value.trim(),
  };
  const r = await fetch('/api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg),
  });
  const result = await r.json();
  alert(result.ok || result.error || '已保存');
});

// Helpers
function esc(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function escAttr(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
}

// Auto-sync: the local server pushes an SSE event whenever a new message
// arrives, so the list stays fresh without manual refresh. The server sends
// an initial event on (re)connect, so the browser's automatic SSE reconnect
// also catches up on anything missed.
if (typeof EventSource !== 'undefined') {
  const es = new EventSource('/api/events');
  es.onmessage = () => loadMessages();
} else {
  // Fallback for very old browsers: poll while the history tab is visible.
  setInterval(() => {
    if (document.getElementById('tab-history').classList.contains('active')) loadMessages();
  }, 5000);
}

// Initial load
loadMessages();
