package dashboard

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GoHole — DNS Sinkhole</title>
<style>
  @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600;700&family=Space+Grotesk:wght@400;500;600&display=swap');

  :root {
    --bg: #0a0e1a;
    --bg2: #0f1524;
    --bg3: #141b2d;
    --border: #1e2d4a;
    --accent: #00d4ff;
    --accent2: #ff4757;
    --accent3: #2ed573;
    --accent4: #ffa502;
    --text: #e8eaf6;
    --text2: #6b7fa8;
    --mono: 'JetBrains Mono', monospace;
    --sans: 'Space Grotesk', sans-serif;
  }

  * { box-sizing: border-box; margin: 0; padding: 0; }

  body {
    background: var(--bg);
    color: var(--text);
    font-family: var(--sans);
    min-height: 100vh;
    overflow-x: hidden;
  }

  /* Animated background grid */
  body::before {
    content: '';
    position: fixed;
    inset: 0;
    background-image:
      linear-gradient(rgba(0,212,255,0.03) 1px, transparent 1px),
      linear-gradient(90deg, rgba(0,212,255,0.03) 1px, transparent 1px);
    background-size: 40px 40px;
    pointer-events: none;
    z-index: 0;
  }

  .layout {
    display: grid;
    grid-template-columns: 220px 1fr;
    min-height: 100vh;
    position: relative;
    z-index: 1;
  }

  /* Sidebar */
  .sidebar {
    background: var(--bg2);
    border-right: 1px solid var(--border);
    padding: 0;
    display: flex;
    flex-direction: column;
    position: sticky;
    top: 0;
    height: 100vh;
  }

  .logo {
    padding: 24px 20px 20px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .logo-icon {
    width: 32px;
    height: 32px;
    background: linear-gradient(135deg, var(--accent), #0080ff);
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    box-shadow: 0 0 20px rgba(0,212,255,0.3);
  }

  .logo-text {
    font-family: var(--mono);
    font-weight: 700;
    font-size: 18px;
    color: var(--accent);
    letter-spacing: -0.5px;
  }

  .logo-sub {
    font-size: 10px;
    color: var(--text2);
    font-family: var(--mono);
    letter-spacing: 0.5px;
  }

  nav { padding: 16px 0; flex: 1; }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 20px;
    cursor: pointer;
    color: var(--text2);
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s;
    border-left: 3px solid transparent;
  }

  .nav-item:hover { color: var(--text); background: rgba(0,212,255,0.05); }
  .nav-item.active { color: var(--accent); border-left-color: var(--accent); background: rgba(0,212,255,0.08); }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent3);
    box-shadow: 0 0 8px var(--accent3);
    animation: pulse 2s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .sidebar-footer {
    padding: 16px 20px;
    border-top: 1px solid var(--border);
    font-size: 11px;
    color: var(--text2);
    font-family: var(--mono);
  }

  /* Main content */
  .main {
    overflow-y: auto;
    padding: 28px;
  }

  .page { display: none; }
  .page.active { display: block; }

  .page-title {
    font-size: 22px;
    font-weight: 600;
    margin-bottom: 6px;
    letter-spacing: -0.3px;
  }

  .page-sub {
    font-size: 13px;
    color: var(--text2);
    margin-bottom: 24px;
  }

  /* Stats grid */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px;
    margin-bottom: 24px;
  }

  .stat-card {
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 20px;
    position: relative;
    overflow: hidden;
    transition: border-color 0.2s;
  }

  .stat-card::before {
    content: '';
    position: absolute;
    top: 0; left: 0; right: 0;
    height: 2px;
  }

  .stat-card.blue::before { background: var(--accent); }
  .stat-card.red::before { background: var(--accent2); }
  .stat-card.green::before { background: var(--accent3); }
  .stat-card.yellow::before { background: var(--accent4); }

  .stat-card:hover { border-color: rgba(0,212,255,0.3); }

  .stat-label {
    font-size: 11px;
    color: var(--text2);
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 8px;
    font-family: var(--mono);
  }

  .stat-value {
    font-size: 28px;
    font-weight: 700;
    font-family: var(--mono);
    line-height: 1;
    margin-bottom: 4px;
  }

  .stat-card.blue .stat-value { color: var(--accent); }
  .stat-card.red .stat-value { color: var(--accent2); }
  .stat-card.green .stat-value { color: var(--accent3); }
  .stat-card.yellow .stat-value { color: var(--accent4); }

  .stat-sub {
    font-size: 12px;
    color: var(--text2);
  }

  /* Cards */
  .card {
    background: var(--bg2);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
    margin-bottom: 20px;
  }

  .card-header {
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .card-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .card-body { padding: 20px; }

  .two-col { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
  .three-col { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 20px; }

  /* Query log table */
  .query-table { width: 100%; border-collapse: collapse; font-family: var(--mono); font-size: 12px; }
  .query-table th {
    text-align: left;
    padding: 8px 12px;
    color: var(--text2);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 1px;
    border-bottom: 1px solid var(--border);
    background: var(--bg3);
  }

  .query-table td {
    padding: 8px 12px;
    border-bottom: 1px solid rgba(30,45,74,0.5);
    color: var(--text2);
    transition: background 0.15s;
  }

  .query-table tr:hover td { background: rgba(0,212,255,0.04); }
  .query-table td.domain { color: var(--text); }
  .query-table td.blocked { color: var(--accent2); }
  .query-table td.allowed { color: var(--accent3); }
  .query-table td.cached { color: var(--accent4); }

  .badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .badge.blocked { background: rgba(255,71,87,0.15); color: var(--accent2); border: 1px solid rgba(255,71,87,0.2); }
  .badge.allowed { background: rgba(46,213,115,0.15); color: var(--accent3); border: 1px solid rgba(46,213,115,0.2); }
  .badge.cached  { background: rgba(255,165,2,0.15);  color: var(--accent4); border: 1px solid rgba(255,165,2,0.2); }
  .badge.error   { background: rgba(150,150,150,0.15); color: #aaa; border: 1px solid rgba(150,150,150,0.2); }

  /* Bar chart */
  .bar-item {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
  }

  .bar-label {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text2);
    width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .bar-track {
    flex: 1;
    height: 6px;
    background: var(--bg3);
    border-radius: 3px;
    overflow: hidden;
  }

  .bar-fill {
    height: 100%;
    border-radius: 3px;
    transition: width 0.6s ease;
  }

  .bar-count {
    font-family: var(--mono);
    font-size: 11px;
    color: var(--text2);
    width: 50px;
    text-align: right;
    flex-shrink: 0;
  }

  /* Timeline chart */
  #timeline-canvas { width: 100%; height: 120px; }

  /* Check domain */
  .check-form { display: flex; gap: 10px; margin-bottom: 20px; }

  .input {
    flex: 1;
    background: var(--bg3);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 10px 14px;
    color: var(--text);
    font-family: var(--mono);
    font-size: 13px;
    outline: none;
    transition: border-color 0.2s;
  }

  .input:focus { border-color: var(--accent); }

  .btn {
    padding: 10px 18px;
    border-radius: 8px;
    border: none;
    cursor: pointer;
    font-family: var(--sans);
    font-weight: 600;
    font-size: 13px;
    transition: all 0.2s;
  }

  .btn-primary { background: var(--accent); color: var(--bg); }
  .btn-primary:hover { background: #00bbdd; box-shadow: 0 0 20px rgba(0,212,255,0.3); }
  .btn-danger { background: var(--accent2); color: white; }
  .btn-danger:hover { background: #ff2f43; }

  .check-result {
    background: var(--bg3);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 16px;
    font-family: var(--mono);
    font-size: 13px;
    display: none;
  }

  .check-result.blocked { border-color: rgba(255,71,87,0.4); }
  .check-result.allowed { border-color: rgba(46,213,115,0.4); }

  /* Blocklist cards */
  .list-card {
    background: var(--bg3);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 14px 16px;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .list-name { font-size: 14px; font-weight: 500; }
  .list-meta { font-size: 12px; color: var(--text2); font-family: var(--mono); margin-top: 2px; }

  .toggle {
    width: 36px;
    height: 20px;
    background: var(--bg);
    border-radius: 10px;
    position: relative;
    cursor: pointer;
    border: 1px solid var(--border);
    transition: background 0.2s;
  }

  .toggle.on { background: var(--accent3); border-color: var(--accent3); }
  .toggle::after {
    content: '';
    position: absolute;
    top: 2px; left: 2px;
    width: 14px; height: 14px;
    border-radius: 50%;
    background: white;
    transition: transform 0.2s;
  }
  .toggle.on::after { transform: translateX(16px); }

  /* Loading */
  .loading { text-align: center; padding: 40px; color: var(--text2); font-family: var(--mono); font-size: 13px; }

  .spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-right: 8px;
    vertical-align: middle;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  /* Scrollbar */
  ::-webkit-scrollbar { width: 6px; height: 6px; }
  ::-webkit-scrollbar-track { background: transparent; }
  ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: var(--text2); }

  .refresh-btn {
    background: none;
    border: 1px solid var(--border);
    color: var(--text2);
    border-radius: 6px;
    padding: 4px 10px;
    font-size: 11px;
    cursor: pointer;
    font-family: var(--mono);
    transition: all 0.2s;
  }
  .refresh-btn:hover { border-color: var(--accent); color: var(--accent); }

  .time-select {
    background: var(--bg3);
    border: 1px solid var(--border);
    color: var(--text2);
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 11px;
    cursor: pointer;
    font-family: var(--mono);
    outline: none;
  }

  .percent-bar {
    height: 8px;
    background: var(--bg3);
    border-radius: 4px;
    overflow: hidden;
    margin-top: 6px;
  }
  .percent-fill {
    height: 100%;
    border-radius: 4px;
    background: linear-gradient(90deg, var(--accent2), #ff6b6b);
    transition: width 1s ease;
  }
</style>
</head>
<body>
<div class="layout">
  <aside class="sidebar">
    <div class="logo">
      <div class="logo-icon">⬡</div>
      <div>
        <div class="logo-text">GoHole</div>
        <div class="logo-sub">DNS SINKHOLE</div>
      </div>
    </div>
    <nav>
      <div class="nav-item active" data-page="dashboard">
        <span>◈</span> Dashboard
      </div>
      <div class="nav-item" data-page="queries">
        <span>◎</span> Query Log
      </div>
      <div class="nav-item" data-page="blocklists">
        <span>◉</span> Blocklists
      </div>
      <div class="nav-item" data-page="check">
        <span>◇</span> Domain Check
      </div>
      <div class="nav-item" data-page="cache">
        <span>◫</span> Cache
      </div>
    </nav>
    <div class="sidebar-footer">
      <div style="display:flex;align-items:center;gap:6px;margin-bottom:4px;">
        <div class="status-dot"></div>
        <span>ACTIVE</span>
      </div>
      <div id="uptime-display">Running</div>
    </div>
  </aside>

  <main class="main">
    <!-- Dashboard -->
    <div class="page active" id="page-dashboard">
      <div class="page-title">Dashboard</div>
      <div class="page-sub">Real-time DNS filtering statistics</div>

      <div style="display:flex;align-items:center;gap:10px;margin-bottom:20px;">
        <select class="time-select" id="time-range" onchange="loadDashboard()">
          <option value="1h">Last hour</option>
          <option value="6h">Last 6 hours</option>
          <option value="24h" selected>Last 24 hours</option>
          <option value="168h">Last 7 days</option>
        </select>
        <button class="refresh-btn" onclick="loadDashboard()">↻ Refresh</button>
        <span id="last-refresh" style="font-size:11px;color:var(--text2);font-family:var(--mono);"></span>
      </div>

      <div class="stats-grid">
        <div class="stat-card blue">
          <div class="stat-label">Total Queries</div>
          <div class="stat-value" id="stat-total">—</div>
          <div class="stat-sub">DNS requests</div>
        </div>
        <div class="stat-card red">
          <div class="stat-label">Blocked</div>
          <div class="stat-value" id="stat-blocked">—</div>
          <div class="stat-sub" id="stat-blocked-pct">of total queries</div>
          <div class="percent-bar"><div class="percent-fill" id="block-bar" style="width:0%"></div></div>
        </div>
        <div class="stat-card green">
          <div class="stat-label">Allowed</div>
          <div class="stat-value" id="stat-allowed">—</div>
          <div class="stat-sub">Forwarded upstream</div>
        </div>
        <div class="stat-card yellow">
          <div class="stat-label">Domains Blocked</div>
          <div class="stat-value" id="stat-domains">—</div>
          <div class="stat-sub">In blocklists</div>
        </div>
      </div>

      <div class="card" style="margin-bottom:20px;">
        <div class="card-header">
          <div class="card-title">⬡ Query Timeline</div>
        </div>
        <div class="card-body" style="padding:16px 20px;">
          <canvas id="timeline-canvas" height="100"></canvas>
        </div>
      </div>

      <div class="two-col">
        <div class="card">
          <div class="card-header">
            <div class="card-title">⬡ Top Blocked Domains</div>
          </div>
          <div class="card-body" id="top-blocked-list">
            <div class="loading"><span class="spinner"></span>Loading...</div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <div class="card-title">⬡ Top Queried Domains</div>
          </div>
          <div class="card-body" id="top-all-list">
            <div class="loading"><span class="spinner"></span>Loading...</div>
          </div>
        </div>
      </div>

      <div class="two-col">
        <div class="card">
          <div class="card-header">
            <div class="card-title">⬡ Top Clients</div>
          </div>
          <div class="card-body" id="top-clients-list">
            <div class="loading"><span class="spinner"></span>Loading...</div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <div class="card-title">⬡ Cache Stats</div>
          </div>
          <div class="card-body" id="cache-stats-mini">
            <div class="loading"><span class="spinner"></span>Loading...</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Query Log -->
    <div class="page" id="page-queries">
      <div class="page-title">Query Log</div>
      <div class="page-sub">Live DNS query stream</div>
      <div style="display:flex;gap:10px;margin-bottom:16px;">
        <button class="refresh-btn" onclick="loadQueryLog()">↻ Refresh</button>
        <button class="refresh-btn" id="auto-refresh-btn" onclick="toggleAutoRefresh()">▶ Auto-refresh</button>
      </div>
      <div class="card">
        <div class="card-body" style="padding:0;overflow-x:auto;">
          <table class="query-table">
            <thead>
              <tr>
                <th>TIME</th>
                <th>DOMAIN</th>
                <th>TYPE</th>
                <th>CLIENT</th>
                <th>STATUS</th>
                <th>LIST</th>
                <th>LATENCY</th>
              </tr>
            </thead>
            <tbody id="query-log-body">
              <tr><td colspan="7" class="loading"><span class="spinner"></span>Loading...</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Blocklists -->
    <div class="page" id="page-blocklists">
      <div class="page-title">Blocklists</div>
      <div class="page-sub">Community-maintained domain filter lists</div>
      <div id="blocklist-content">
        <div class="loading"><span class="spinner"></span>Loading...</div>
      </div>
    </div>

    <!-- Domain Check -->
    <div class="page" id="page-check">
      <div class="page-title">Domain Check</div>
      <div class="page-sub">Check if a domain would be blocked</div>
      <div class="card">
        <div class="card-body">
          <div class="check-form">
            <input class="input" id="check-input" placeholder="enter domain to check (e.g. ads.example.com)" onkeydown="if(event.key==='Enter')checkDomain()">
            <button class="btn btn-primary" onclick="checkDomain()">Check</button>
          </div>
          <div class="check-result" id="check-result"></div>
        </div>
      </div>
    </div>

    <!-- Cache -->
    <div class="page" id="page-cache">
      <div class="page-title">DNS Cache</div>
      <div class="page-sub">Response cache performance</div>
      <div class="card">
        <div class="card-header">
          <div class="card-title">Cache Statistics</div>
          <button class="btn btn-danger" onclick="flushCache()" style="font-size:12px;padding:6px 14px;">Flush Cache</button>
        </div>
        <div class="card-body" id="cache-detail">
          <div class="loading"><span class="spinner"></span>Loading...</div>
        </div>
      </div>
    </div>
  </main>
</div>

<script>
let autoRefreshInterval = null;

// Navigation
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', () => {
    document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    item.classList.add('active');
    const page = item.dataset.page;
    document.getElementById('page-' + page).classList.add('active');
    if (page === 'queries') loadQueryLog();
    if (page === 'blocklists') loadBlocklists();
    if (page === 'cache') loadCacheDetail();
  });
});

async function api(path) {
  const r = await fetch(path);
  return r.json();
}

function fmt(n) {
  if (n === undefined || n === null) return '—';
  if (n >= 1000000) return (n/1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n/1000).toFixed(1) + 'K';
  return n.toString();
}

function timeAgo(ts) {
  const d = new Date(ts);
  const diff = (Date.now() - d) / 1000;
  if (diff < 60) return Math.round(diff) + 's ago';
  if (diff < 3600) return Math.round(diff/60) + 'm ago';
  return d.toLocaleTimeString();
}

async function loadDashboard() {
  const since = document.getElementById('time-range').value;
  document.getElementById('last-refresh').textContent = 'Refreshing...';

  try {
    const [stats, topBlocked, topAll, clients, timeline] = await Promise.all([
      api('/api/stats?since=' + since),
      api('/api/top-domains?type=blocked&limit=8&since=' + since),
      api('/api/top-domains?limit=8&since=' + since),
      api('/api/top-clients?limit=6&since=' + since),
      api('/api/timeline?buckets=24&since=' + since),
    ]);

    const qs = stats.query_stats || {};
    document.getElementById('stat-total').textContent = fmt(qs.TotalQueries);
    document.getElementById('stat-blocked').textContent = fmt(qs.BlockedQueries);
    document.getElementById('stat-allowed').textContent = fmt(qs.AllowedQueries);

    const bl = stats.blocklist_stats || {};
    document.getElementById('stat-domains').textContent = fmt(bl.total_blocked);

    const pct = qs.PercentBlocked ? qs.PercentBlocked.toFixed(1) : '0.0';
    document.getElementById('stat-blocked-pct').textContent = pct + '% of total';
    document.getElementById('block-bar').style.width = pct + '%';

    // Top blocked
    renderBars('top-blocked-list', topBlocked || [], 'Domain', '#ff4757');
    // Top all
    renderBars('top-all-list', topAll || [], 'Domain', '#00d4ff');
    // Clients
    renderBarsClients('top-clients-list', clients || []);

    // Cache mini
    const cs = stats.cache_stats || {};
    document.getElementById('cache-stats-mini').innerHTML = 
      '<div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">' +
      statMini('Entries', cs.size + '/' + cs.max_size) +
      statMini('Hit Rate', (cs.hit_rate || 0).toFixed(1) + '%') +
      statMini('Hits', fmt(cs.hits)) +
      statMini('Misses', fmt(cs.misses)) +
      '</div>';

    // Timeline
    drawTimeline(timeline || []);

    document.getElementById('last-refresh').textContent = 'Updated ' + new Date().toLocaleTimeString();
  } catch(e) {
    console.error(e);
    document.getElementById('last-refresh').textContent = 'Error loading data';
  }
}

function statMini(label, value) {
  return '<div style="background:var(--bg3);border:1px solid var(--border);border-radius:8px;padding:12px;">' +
    '<div style="font-size:10px;color:var(--text2);font-family:var(--mono);text-transform:uppercase;letter-spacing:1px;margin-bottom:4px;">' + label + '</div>' +
    '<div style="font-size:20px;font-weight:700;font-family:var(--mono);color:var(--text);">' + value + '</div>' +
    '</div>';
}

function renderBars(containerId, items, labelKey, color) {
  if (!items || items.length === 0) {
    document.getElementById(containerId).innerHTML = '<div style="color:var(--text2);font-size:13px;text-align:center;padding:20px;">No data yet</div>';
    return;
  }
  const max = Math.max(...items.map(i => i.Count || 0));
  let html = '';
  items.forEach(item => {
    const pct = max > 0 ? (item.Count / max * 100) : 0;
    const domain = item.Domain || item.IP || '—';
    html += '<div class="bar-item">' +
      '<div class="bar-label" title="' + domain + '">' + domain + '</div>' +
      '<div class="bar-track"><div class="bar-fill" style="width:' + pct + '%;background:' + color + ';"></div></div>' +
      '<div class="bar-count">' + fmt(item.Count) + '</div>' +
      '</div>';
  });
  document.getElementById(containerId).innerHTML = html;
}

function renderBarsClients(containerId, items) {
  renderBars(containerId, items.map(c => ({Domain: c.IP, Count: c.Count})), 'IP', '#2ed573');
}

function drawTimeline(data) {
  const canvas = document.getElementById('timeline-canvas');
  const rect = canvas.getBoundingClientRect();
  canvas.width = rect.width * window.devicePixelRatio;
  canvas.height = 100 * window.devicePixelRatio;
  const ctx = canvas.getContext('2d');
  ctx.scale(window.devicePixelRatio, window.devicePixelRatio);

  const W = rect.width, H = 100;
  const pad = { t: 10, r: 10, b: 20, l: 40 };
  const chartW = W - pad.l - pad.r;
  const chartH = H - pad.t - pad.b;

  if (!data || data.length === 0) return;

  const maxVal = Math.max(...data.map(d => (d.blocked || 0) + (d.allowed || 0) + (d.cached || 0)), 1);

  ctx.clearRect(0, 0, W, H);

  // Grid lines
  ctx.strokeStyle = 'rgba(30,45,74,0.8)';
  ctx.lineWidth = 1;
  for (let i = 0; i <= 4; i++) {
    const y = pad.t + chartH - (i/4) * chartH;
    ctx.beginPath(); ctx.moveTo(pad.l, y); ctx.lineTo(W - pad.r, y); ctx.stroke();
  }

  const bw = chartW / data.length;

  // Draw bars (blocked stacked)
  data.forEach((d, i) => {
    const x = pad.l + i * bw;
    const allowed = (d.allowed || 0) + (d.cached || 0);
    const blocked = d.blocked || 0;

    const aH = (allowed / maxVal) * chartH;
    const bH = (blocked / maxVal) * chartH;

    if (aH > 0) {
      ctx.fillStyle = 'rgba(0,212,255,0.5)';
      ctx.fillRect(x + 1, pad.t + chartH - aH, bw - 2, aH);
    }
    if (bH > 0) {
      ctx.fillStyle = 'rgba(255,71,87,0.7)';
      ctx.fillRect(x + 1, pad.t + chartH - aH - bH, bw - 2, bH);
    }
  });

  // Labels
  ctx.fillStyle = 'rgba(107,127,168,0.8)';
  ctx.font = '9px JetBrains Mono, monospace';
  ctx.textAlign = 'right';
  for (let i = 0; i <= 4; i++) {
    const y = pad.t + chartH - (i/4) * chartH;
    ctx.fillText(fmt(Math.round(maxVal * i/4)), pad.l - 4, y + 3);
  }

  // Legend
  ctx.textAlign = 'left';
  ctx.fillStyle = 'rgba(0,212,255,0.8)';
  ctx.fillRect(pad.l, H - 12, 8, 8);
  ctx.fillStyle = 'rgba(107,127,168,0.8)';
  ctx.fillText('Allowed', pad.l + 12, H - 5);
  ctx.fillStyle = 'rgba(255,71,87,0.8)';
  ctx.fillRect(pad.l + 70, H - 12, 8, 8);
  ctx.fillStyle = 'rgba(107,127,168,0.8)';
  ctx.fillText('Blocked', pad.l + 82, H - 5);
}

async function loadQueryLog() {
  const tbody = document.getElementById('query-log-body');
  tbody.innerHTML = '<tr><td colspan="7" class="loading"><span class="spinner"></span>Loading...</td></tr>';
  try {
    const queries = await api('/api/queries?limit=200');
    if (!queries || queries.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;padding:30px;color:var(--text2);font-family:var(--mono);">No queries yet</td></tr>';
      return;
    }
    tbody.innerHTML = queries.map(q => {
      const rt = q.ResponseType || 'error';
      return '<tr>' +
        '<td style="color:var(--text2);white-space:nowrap;">' + timeAgo(q.Timestamp) + '</td>' +
        '<td class="domain">' + (q.Domain || '—') + '</td>' +
        '<td>' + (q.QueryType || '—') + '</td>' +
        '<td>' + (q.SourceIP || '—') + '</td>' +
        '<td><span class="badge ' + rt + '">' + rt + '</span></td>' +
        '<td style="color:var(--text2);font-size:11px;">' + (q.ListName || '') + '</td>' +
        '<td style="color:var(--text2);">' + (q.LatencyMs || 0) + 'ms</td>' +
        '</tr>';
    }).join('');
  } catch(e) {
    tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;padding:30px;color:var(--accent2);">Error loading queries</td></tr>';
  }
}

function toggleAutoRefresh() {
  const btn = document.getElementById('auto-refresh-btn');
  if (autoRefreshInterval) {
    clearInterval(autoRefreshInterval);
    autoRefreshInterval = null;
    btn.textContent = '▶ Auto-refresh';
    btn.style.borderColor = '';
    btn.style.color = '';
  } else {
    loadQueryLog();
    autoRefreshInterval = setInterval(loadQueryLog, 5000);
    btn.textContent = '⏹ Stop refresh';
    btn.style.borderColor = 'var(--accent3)';
    btn.style.color = 'var(--accent3)';
  }
}

async function loadBlocklists() {
  const container = document.getElementById('blocklist-content');
  try {
    const data = await api('/api/blocklists');
    const lists = data.lists || [];
    const total = data.total_blocked || 0;

    let html = '<div class="stats-grid" style="grid-template-columns:repeat(3,1fr);margin-bottom:20px;">' +
      '<div class="stat-card blue"><div class="stat-label">Total Domains</div><div class="stat-value">' + fmt(total) + '</div></div>' +
      '<div class="stat-card green"><div class="stat-label">Active Lists</div><div class="stat-value">' + lists.filter(l => l.enabled).length + '</div></div>' +
      '<div class="stat-card yellow"><div class="stat-label">Whitelist</div><div class="stat-value">' + fmt(data.total_whitelist) + '</div></div>' +
      '</div>';

    html += '<div class="card"><div class="card-header"><div class="card-title">Filter Lists</div></div><div class="card-body">';
    if (lists.length === 0) {
      html += '<div style="text-align:center;padding:20px;color:var(--text2);">No lists loaded</div>';
    } else {
      lists.forEach(l => {
        const updated = l.last_updated ? new Date(l.last_updated).toLocaleString() : 'Never';
        html += '<div class="list-card">' +
          '<div>' +
            '<div class="list-name">' + l.name + '</div>' +
            '<div class="list-meta">' + fmt(l.domain_count) + ' domains · Updated ' + updated + '</div>' +
          '</div>' +
          '<div class="' + (l.enabled ? 'toggle on' : 'toggle') + '"></div>' +
          '</div>';
      });
    }
    html += '</div></div>';
    container.innerHTML = html;
  } catch(e) {
    container.innerHTML = '<div style="color:var(--accent2);padding:20px;">Error loading blocklists</div>';
  }
}

async function checkDomain() {
  const domain = document.getElementById('check-input').value.trim();
  if (!domain) return;
  const result = document.getElementById('check-result');
  result.style.display = 'block';
  result.innerHTML = '<span class="spinner"></span> Checking...';
  result.className = 'check-result';

  try {
    const data = await api('/api/check?domain=' + encodeURIComponent(domain));
    if (data.blocked) {
      result.className = 'check-result blocked';
      result.innerHTML = '<span style="color:var(--accent2);font-weight:700;">⊗ BLOCKED</span><br>' +
        '<span style="color:var(--text2);">Domain:</span> ' + data.domain + '<br>' +
        '<span style="color:var(--text2);">List:</span> ' + (data.source || 'custom') + '<br>' +
        '<span style="color:var(--text2);">Whitelisted:</span> ' + data.whitelisted;
    } else {
      result.className = 'check-result allowed';
      result.innerHTML = '<span style="color:var(--accent3);font-weight:700;">✓ ALLOWED</span><br>' +
        '<span style="color:var(--text2);">Domain:</span> ' + data.domain + '<br>' +
        '<span style="color:var(--text2);">Whitelisted:</span> ' + data.whitelisted;
    }
  } catch(e) {
    result.innerHTML = '<span style="color:var(--accent2);">Error checking domain</span>';
  }
}

async function loadCacheDetail() {
  const container = document.getElementById('cache-detail');
  try {
    const cs = await api('/api/cache');
    container.innerHTML = '<div class="stats-grid" style="grid-template-columns:repeat(4,1fr);">' +
      '<div class="stat-card blue"><div class="stat-label">Entries</div><div class="stat-value">' + fmt(cs.size) + '</div><div class="stat-sub">of ' + fmt(cs.max_size) + ' max</div></div>' +
      '<div class="stat-card green"><div class="stat-label">Hit Rate</div><div class="stat-value">' + (cs.hit_rate||0).toFixed(1) + '%</div></div>' +
      '<div class="stat-card yellow"><div class="stat-label">Hits</div><div class="stat-value">' + fmt(cs.hits) + '</div></div>' +
      '<div class="stat-card red"><div class="stat-label">Misses</div><div class="stat-value">' + fmt(cs.misses) + '</div></div>' +
      '</div>';
  } catch(e) {
    container.innerHTML = '<div style="color:var(--accent2);">Error loading cache stats</div>';
  }
}

async function flushCache() {
  if (!confirm('Flush the DNS response cache?')) return;
  await fetch('/api/cache', { method: 'DELETE' });
  loadCacheDetail();
}

// Init
loadDashboard();
setInterval(loadDashboard, 30000);
</script>
</body>
</html>`
