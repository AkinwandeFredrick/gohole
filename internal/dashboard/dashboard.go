package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AkinwandeFredrick/gohole/internal/blocklist"
	"github.com/AkinwandeFredrick/gohole/internal/cache"
	"github.com/AkinwandeFredrick/gohole/internal/logger"
)

// Config holds dashboard configuration.
type Config struct {
	ListenAddr  string
	QueryLogger *logger.Logger
	Blocklist   *blocklist.Engine
	Cache       *cache.Cache
	Username    string
	Password    string
}

// session represents an authenticated session.
type session struct {
	token     string
	expiresAt time.Time
}

// Dashboard serves the web UI.
type Dashboard struct {
	cfg      Config
	srv      *http.Server
	sessions map[string]session
	mu       sync.RWMutex
}

// New creates a new Dashboard.
func New(cfg Config) *Dashboard {
	return &Dashboard{
		cfg:      cfg,
		sessions: make(map[string]session),
	}
}

// generateToken creates a random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createSession creates and stores a new session, returns the token.
func (d *Dashboard) createSession() (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	d.sessions[token] = session{token: token, expiresAt: time.Now().Add(24 * time.Hour)}
	d.mu.Unlock()
	return token, nil
}

// isAuthenticated checks if the request has a valid session cookie.
func (d *Dashboard) isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("gohole_session")
	if err != nil {
		return false
	}
	d.mu.RLock()
	sess, ok := d.sessions[cookie.Value]
	d.mu.RUnlock()
	return ok && time.Now().Before(sess.expiresAt)
}

// authMiddleware wraps handlers to require authentication.
func (d *Dashboard) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.isAuthenticated(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// handleLogin serves and processes the login form.
func (d *Dashboard) handleLogin(w http.ResponseWriter, r *http.Request) {
	if d.isAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == d.cfg.Username && password == d.cfg.Password {
			token, err := d.createSession()
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "gohole_session",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Expires:  time.Now().Add(24 * time.Hour),
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Bad credentials — re-render with error
		renderLogin(w, "Invalid username or password.")
		return
	}

	renderLogin(w, "")
}

// handleLogout clears the session.
func (d *Dashboard) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("gohole_session")
	if err == nil {
		d.mu.Lock()
		delete(d.sessions, cookie.Value)
		d.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "gohole_session",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Start binds and serves the dashboard.
func (d *Dashboard) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/login", d.handleLogin)
	mux.HandleFunc("/logout", d.handleLogout)
	mux.HandleFunc("/api/stats", d.authMiddleware(d.handleStats))
	mux.HandleFunc("/api/queries", d.authMiddleware(d.handleQueries))
	mux.HandleFunc("/api/top-domains", d.authMiddleware(d.handleTopDomains))
	mux.HandleFunc("/api/top-clients", d.authMiddleware(d.handleTopClients))
	mux.HandleFunc("/api/blocklists", d.authMiddleware(d.handleBlocklists))
	mux.HandleFunc("/api/check", d.authMiddleware(d.handleCheck))
	mux.HandleFunc("/api/cache", d.authMiddleware(d.handleCache))
	mux.HandleFunc("/api/timeline", d.authMiddleware(d.handleTimeline))
	mux.HandleFunc("/", d.authMiddleware(d.handleDashboard))

	d.srv = &http.Server{
		Addr:    d.cfg.ListenAddr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", d.cfg.ListenAddr)
	if err != nil {
		return err
	}
	log.Printf("Dashboard listening on http://%s", d.cfg.ListenAddr)

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.srv.Shutdown(shutCtx)
	}()

	if err := d.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ── API handlers ──────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := d.cfg.QueryLogger.GetStats()
	writeJSON(w, map[string]any{
		"total_queries":   stats.TotalQueries,
		"blocked_queries": stats.BlockedQueries,
		"blocked_percent": stats.BlockedPercent,
		"total_domains":   d.cfg.Blocklist.TotalBlocked(),
		"cache_size":      d.cfg.Cache.Size(),
		"cache_hit_rate":  d.cfg.Cache.HitRate(),
	})
}

func (d *Dashboard) handleQueries(w http.ResponseWriter, r *http.Request) {
	queries := d.cfg.QueryLogger.GetRecentQueries(100)
	writeJSON(w, queries)
}

func (d *Dashboard) handleTopDomains(w http.ResponseWriter, r *http.Request) {
	domains := d.cfg.QueryLogger.GetTopDomains(10)
	writeJSON(w, domains)
}

func (d *Dashboard) handleTopClients(w http.ResponseWriter, r *http.Request) {
	clients := d.cfg.QueryLogger.GetTopClients(10)
	writeJSON(w, clients)
}

func (d *Dashboard) handleBlocklists(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, d.cfg.Blocklist.GetSources())
}

func (d *Dashboard) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	blocked := d.cfg.Blocklist.IsBlocked(domain)
	writeJSON(w, map[string]any{"domain": domain, "blocked": blocked})
}

func (d *Dashboard) handleCache(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"size":     d.cfg.Cache.Size(),
		"hit_rate": d.cfg.Cache.HitRate(),
	})
}

func (d *Dashboard) handleTimeline(w http.ResponseWriter, r *http.Request) {
	data := d.cfg.QueryLogger.GetTimeline(60)
	writeJSON(w, data)
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// ── Login page renderer ───────────────────────────────────────────────────────

func renderLogin(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl := template.Must(template.New("login").Parse(loginHTML))
	tmpl.Execute(w, map[string]string{"Error": errMsg})
}

// ── Embedded HTML ─────────────────────────────────────────────────────────────

const loginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GoHole — Sign In</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Space+Mono:wght@400;700&family=Inter:wght@300;400;500&display=swap" rel="stylesheet">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --bg: #0a0c10;
    --surface: #111318;
    --border: #1e2130;
    --accent: #00d4aa;
    --accent-dim: rgba(0, 212, 170, 0.12);
    --text: #e2e8f0;
    --muted: #64748b;
    --error: #f87171;
  }
  html, body { height: 100%; }
  body {
    background: var(--bg);
    color: var(--text);
    font-family: 'Inter', sans-serif;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    overflow: hidden;
  }
  /* animated grid background */
  body::before {
    content: '';
    position: fixed; inset: 0;
    background-image:
      linear-gradient(var(--border) 1px, transparent 1px),
      linear-gradient(90deg, var(--border) 1px, transparent 1px);
    background-size: 40px 40px;
    opacity: 0.4;
    pointer-events: none;
  }
  body::after {
    content: '';
    position: fixed; inset: 0;
    background: radial-gradient(ellipse 80% 60% at 50% 40%, rgba(0,212,170,0.06) 0%, transparent 70%);
    pointer-events: none;
  }
  .card {
    position: relative;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 48px 40px;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 0 0 1px rgba(0,212,170,0.05), 0 32px 64px rgba(0,0,0,0.6);
    animation: slideUp 0.4s cubic-bezier(.16,1,.3,1);
  }
  @keyframes slideUp {
    from { opacity: 0; transform: translateY(24px); }
    to   { opacity: 1; transform: translateY(0); }
  }
  .logo {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 32px;
  }
  .logo-icon {
    width: 40px; height: 40px;
    background: var(--accent-dim);
    border: 1px solid rgba(0,212,170,0.3);
    border-radius: 10px;
    display: flex; align-items: center; justify-content: center;
    font-size: 18px;
  }
  .logo-text {
    font-family: 'Space Mono', monospace;
    font-size: 22px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: -0.5px;
  }
  .logo-sub {
    font-size: 11px;
    color: var(--muted);
    font-family: 'Space Mono', monospace;
    letter-spacing: 1px;
    text-transform: uppercase;
    margin-top: 1px;
  }
  h1 {
    font-size: 20px;
    font-weight: 500;
    margin-bottom: 6px;
  }
  .subtitle {
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 28px;
  }
  .field { margin-bottom: 16px; }
  label {
    display: block;
    font-size: 12px;
    font-weight: 500;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.8px;
    margin-bottom: 8px;
    font-family: 'Space Mono', monospace;
  }
  input {
    width: 100%;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 14px;
    color: var(--text);
    font-size: 14px;
    font-family: 'Inter', sans-serif;
    outline: none;
    transition: border-color 0.2s, box-shadow 0.2s;
  }
  input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px rgba(0,212,170,0.1);
  }
  .error-msg {
    background: rgba(248,113,113,0.1);
    border: 1px solid rgba(248,113,113,0.3);
    border-radius: 8px;
    padding: 10px 14px;
    font-size: 13px;
    color: var(--error);
    margin-bottom: 20px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  button[type=submit] {
    width: 100%;
    padding: 13px;
    background: var(--accent);
    color: #0a0c10;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    font-family: 'Space Mono', monospace;
    cursor: pointer;
    margin-top: 8px;
    transition: opacity 0.2s, transform 0.1s;
    letter-spacing: 0.3px;
  }
  button[type=submit]:hover { opacity: 0.9; }
  button[type=submit]:active { transform: scale(0.99); }
  .footer {
    text-align: center;
    margin-top: 24px;
    font-size: 11px;
    color: var(--muted);
    font-family: 'Space Mono', monospace;
  }
</style>
</head>
<body>
<div class="card">
  <div class="logo">
    <div class="logo-icon">🕳️</div>
    <div>
      <div class="logo-text">GoHole</div>
      <div class="logo-sub">DNS Sinkhole</div>
    </div>
  </div>
  <h1>Welcome back</h1>
  <p class="subtitle">Sign in to access your dashboard</p>
  {{if .Error}}
  <div class="error-msg">⚠ {{.Error}}</div>
  {{end}}
  <form method="POST" action="/login">
    <div class="field">
      <label>Username</label>
      <input type="text" name="username" autocomplete="username" autofocus required>
    </div>
    <div class="field">
      <label>Password</label>
      <input type="password" name="password" autocomplete="current-password" required>
    </div>
    <button type="submit">Sign In →</button>
  </form>
  <div class="footer">GoHole v1.0 · DNS Sinkhole</div>
</div>
</body>
</html>`

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GoHole Dashboard</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Space+Mono:wght@400;700&family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --bg: #0a0c10;
    --surface: #111318;
    --surface2: #161a24;
    --border: #1e2130;
    --accent: #00d4aa;
    --accent-dim: rgba(0,212,170,0.12);
    --red: #f87171;
    --yellow: #fbbf24;
    --text: #e2e8f0;
    --muted: #64748b;
  }
  html, body { height: 100%; background: var(--bg); color: var(--text); font-family: 'Inter', sans-serif; }
  /* Layout */
  .layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
  /* Sidebar */
  .sidebar {
    background: var(--surface);
    border-right: 1px solid var(--border);
    padding: 24px 16px;
    display: flex; flex-direction: column;
    position: sticky; top: 0; height: 100vh;
  }
  .brand { display: flex; align-items: center; gap: 10px; padding: 0 8px 28px; border-bottom: 1px solid var(--border); margin-bottom: 20px; }
  .brand-icon { font-size: 22px; }
  .brand-name { font-family: 'Space Mono', monospace; font-size: 16px; color: var(--accent); font-weight: 700; }
  nav a {
    display: flex; align-items: center; gap: 10px;
    padding: 9px 12px; border-radius: 8px;
    font-size: 13px; color: var(--muted);
    text-decoration: none; margin-bottom: 2px;
    transition: background 0.15s, color 0.15s;
  }
  nav a:hover, nav a.active { background: var(--accent-dim); color: var(--accent); }
  .sidebar-spacer { flex: 1; }
  .logout-btn {
    display: flex; align-items: center; gap: 10px;
    padding: 9px 12px; border-radius: 8px;
    font-size: 13px; color: var(--muted);
    background: none; border: none; cursor: pointer; width: 100%;
    transition: background 0.15s, color 0.15s; font-family: 'Inter', sans-serif;
  }
  .logout-btn:hover { background: rgba(248,113,113,0.1); color: var(--red); }
  /* Main */
  .main { padding: 32px; overflow-y: auto; }
  .page-title { font-size: 22px; font-weight: 600; margin-bottom: 24px; }
  .page { display: none; }
  .page.active { display: block; }
  /* Stat cards */
  .stats-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; margin-bottom: 28px; }
  .stat-card {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 20px;
  }
  .stat-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.8px; font-family: 'Space Mono', monospace; margin-bottom: 8px; }
  .stat-value { font-size: 28px; font-weight: 600; font-family: 'Space Mono', monospace; }
  .stat-value.green { color: var(--accent); }
  .stat-value.red { color: var(--red); }
  .stat-sub { font-size: 12px; color: var(--muted); margin-top: 4px; }
  /* Tables */
  .card { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; overflow: hidden; margin-bottom: 20px; }
  .card-header { padding: 16px 20px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; }
  .card-title { font-size: 13px; font-weight: 600; font-family: 'Space Mono', monospace; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { padding: 10px 16px; text-align: left; font-size: 11px; color: var(--muted); font-weight: 500; text-transform: uppercase; letter-spacing: 0.6px; border-bottom: 1px solid var(--border); }
  td { padding: 11px 16px; border-bottom: 1px solid rgba(30,33,48,0.6); font-family: 'Space Mono', monospace; font-size: 12px; }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: var(--surface2); }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; font-family: 'Space Mono', monospace; }
  .badge-blocked { background: rgba(248,113,113,0.12); color: var(--red); }
  .badge-allowed { background: rgba(0,212,170,0.1); color: var(--accent); }
  /* Check domain */
  .check-form { display: flex; gap: 10px; padding: 20px; }
  .check-input { flex: 1; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 10px 14px; color: var(--text); font-size: 13px; font-family: 'Space Mono', monospace; outline: none; }
  .check-input:focus { border-color: var(--accent); }
  .check-btn { padding: 10px 18px; background: var(--accent); color: #0a0c10; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; font-family: 'Space Mono', monospace; cursor: pointer; }
  .check-result { padding: 0 20px 20px; font-family: 'Space Mono', monospace; font-size: 13px; }
  /* Chart */
  .chart-wrap { padding: 16px 20px 20px; }
  canvas { width: 100% !important; }
  /* Responsive */
  @media (max-width: 768px) {
    .layout { grid-template-columns: 1fr; }
    .sidebar { position: relative; height: auto; }
  }
</style>
</head>
<body>
<div class="layout">
  <aside class="sidebar">
    <div class="brand">
      <span class="brand-icon">🕳️</span>
      <span class="brand-name">GoHole</span>
    </div>
    <nav>
      <a href="#" class="active" data-page="overview">📊 Overview</a>
      <a href="#" data-page="queries">📋 Query Log</a>
      <a href="#" data-page="top">🏆 Top Domains</a>
      <a href="#" data-page="blocklists">🛡️ Blocklists</a>
      <a href="#" data-page="check">🔍 Check Domain</a>
    </nav>
    <div class="sidebar-spacer"></div>
    <button class="logout-btn" onclick="logout()">⎋ Sign Out</button>
  </aside>

  <main class="main">
    <!-- Overview -->
    <div class="page active" id="page-overview">
      <div class="page-title">Overview</div>
      <div class="stats-grid">
        <div class="stat-card"><div class="stat-label">Total Queries</div><div class="stat-value" id="stat-total">—</div></div>
        <div class="stat-card"><div class="stat-label">Blocked</div><div class="stat-value red" id="stat-blocked">—</div></div>
        <div class="stat-card"><div class="stat-label">Block Rate</div><div class="stat-value green" id="stat-rate">—</div></div>
        <div class="stat-card"><div class="stat-label">Domains Listed</div><div class="stat-value" id="stat-domains">—</div></div>
        <div class="stat-card"><div class="stat-label">Cache Size</div><div class="stat-value" id="stat-cache">—</div></div>
        <div class="stat-card"><div class="stat-label">Cache Hit Rate</div><div class="stat-value green" id="stat-hit">—</div></div>
      </div>
      <div class="card">
        <div class="card-header"><span class="card-title">QUERY TIMELINE (last 60 min)</span></div>
        <div class="chart-wrap"><canvas id="timeline-chart" height="80"></canvas></div>
      </div>
    </div>

    <!-- Query Log -->
    <div class="page" id="page-queries">
      <div class="page-title">Query Log</div>
      <div class="card">
        <table>
          <thead><tr><th>Time</th><th>Domain</th><th>Client</th><th>Type</th><th>Status</th></tr></thead>
          <tbody id="queries-body"><tr><td colspan="5" style="text-align:center;color:var(--muted)">Loading…</td></tr></tbody>
        </table>
      </div>
    </div>

    <!-- Top Domains -->
    <div class="page" id="page-top">
      <div class="page-title">Top Domains</div>
      <div class="card">
        <table>
          <thead><tr><th>Domain</th><th>Hits</th></tr></thead>
          <tbody id="top-body"><tr><td colspan="2" style="text-align:center;color:var(--muted)">Loading…</td></tr></tbody>
        </table>
      </div>
    </div>

    <!-- Blocklists -->
    <div class="page" id="page-blocklists">
      <div class="page-title">Blocklists</div>
      <div class="card">
        <table>
          <thead><tr><th>Name</th><th>Domains</th><th>Status</th></tr></thead>
          <tbody id="blocklists-body"><tr><td colspan="3" style="text-align:center;color:var(--muted)">Loading…</td></tr></tbody>
        </table>
      </div>
    </div>

    <!-- Check Domain -->
    <div class="page" id="page-check">
      <div class="page-title">Check Domain</div>
      <div class="card">
        <div class="check-form">
          <input class="check-input" id="check-input" placeholder="doubleclick.net" type="text" onkeydown="if(event.key==='Enter')checkDomain()">
          <button class="check-btn" onclick="checkDomain()">Check →</button>
        </div>
        <div class="check-result" id="check-result"></div>
      </div>
    </div>
  </main>
</div>

<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.0/chart.umd.min.js"></script>
<script>
let chart = null;

// Navigation
document.querySelectorAll('nav a').forEach(a => {
  a.addEventListener('click', e => {
    e.preventDefault();
    document.querySelectorAll('nav a').forEach(x => x.classList.remove('active'));
    a.classList.add('active');
    const page = a.dataset.page;
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    document.getElementById('page-' + page).classList.add('active');
    loadPage(page);
  });
});

async function api(path) {
  const r = await fetch(path);
  if (r.status === 401) { window.location = '/login'; return null; }
  return r.json();
}

async function loadStats() {
  const d = await api('/api/stats');
  if (!d) return;
  document.getElementById('stat-total').textContent = d.total_queries.toLocaleString();
  document.getElementById('stat-blocked').textContent = d.blocked_queries.toLocaleString();
  document.getElementById('stat-rate').textContent = d.blocked_percent.toFixed(1) + '%';
  document.getElementById('stat-domains').textContent = d.total_domains.toLocaleString();
  document.getElementById('stat-cache').textContent = d.cache_size.toLocaleString();
  document.getElementById('stat-hit').textContent = d.cache_hit_rate.toFixed(1) + '%';
}

async function loadTimeline() {
  const d = await api('/api/timeline');
  if (!d) return;
  const labels = d.map(x => x.time);
  const total  = d.map(x => x.total);
  const blocked = d.map(x => x.blocked);
  if (chart) { chart.destroy(); }
  const ctx = document.getElementById('timeline-chart').getContext('2d');
  chart = new Chart(ctx, {
    type: 'line',
    data: {
      labels,
      datasets: [
        { label: 'Total',   data: total,   borderColor: '#00d4aa', backgroundColor: 'rgba(0,212,170,0.08)', tension: 0.4, fill: true, pointRadius: 0 },
        { label: 'Blocked', data: blocked, borderColor: '#f87171', backgroundColor: 'rgba(248,113,113,0.06)', tension: 0.4, fill: true, pointRadius: 0 },
      ]
    },
    options: {
      plugins: { legend: { labels: { color: '#64748b', font: { family: 'Space Mono', size: 11 } } } },
      scales: {
        x: { ticks: { color: '#64748b', font: { family: 'Space Mono', size: 10 }, maxTicksLimit: 10 }, grid: { color: '#1e2130' } },
        y: { ticks: { color: '#64748b', font: { family: 'Space Mono', size: 10 } }, grid: { color: '#1e2130' } }
      }
    }
  });
}

async function loadQueries() {
  const d = await api('/api/queries');
  if (!d) return;
  const tb = document.getElementById('queries-body');
  if (!d.length) { tb.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--muted)">No queries yet</td></tr>'; return; }
  tb.innerHTML = d.map(q => {
    const badge = q.blocked
      ? '<span class="badge badge-blocked">BLOCKED</span>'
      : '<span class="badge badge-allowed">ALLOWED</span>';
    const t = new Date(q.timestamp).toLocaleTimeString();
    return '<tr><td>' + t + '</td><td>' + q.domain + '</td><td>' + (q.client||'—') + '</td><td>' + (q.type||'A') + '</td><td>' + badge + '</td></tr>';
  }).join('');
}

async function loadTop() {
  const d = await api('/api/top-domains');
  if (!d) return;
  const tb = document.getElementById('top-body');
  if (!d.length) { tb.innerHTML = '<tr><td colspan="2" style="text-align:center;color:var(--muted)">No data yet</td></tr>'; return; }
  tb.innerHTML = d.map(x => '<tr><td>' + x.domain + '</td><td>' + x.count + '</td></tr>').join('');
}

async function loadBlocklists() {
  const d = await api('/api/blocklists');
  if (!d) return;
  const tb = document.getElementById('blocklists-body');
  if (!d.length) { tb.innerHTML = '<tr><td colspan="3" style="text-align:center;color:var(--muted)">None configured</td></tr>'; return; }
  tb.innerHTML = d.map(b => '<tr><td>' + b.name + '</td><td>' + (b.count||0).toLocaleString() + '</td><td><span class="badge badge-allowed">ACTIVE</span></td></tr>').join('');
}

async function checkDomain() {
  const dom = document.getElementById('check-input').value.trim();
  if (!dom) return;
  const d = await api('/api/check?domain=' + encodeURIComponent(dom));
  if (!d) return;
  const r = document.getElementById('check-result');
  r.innerHTML = d.blocked
    ? '<span style="color:var(--red)">✗ ' + dom + ' is BLOCKED</span>'
    : '<span style="color:var(--accent)">✓ ' + dom + ' is ALLOWED</span>';
}

async function loadPage(page) {
  if (page === 'overview') { loadStats(); loadTimeline(); }
  else if (page === 'queries') loadQueries();
  else if (page === 'top') loadTop();
  else if (page === 'blocklists') loadBlocklists();
}

function logout() {
  window.location = '/logout';
}

// Initial load
loadStats();
loadTimeline();
setInterval(() => { loadStats(); loadTimeline(); }, 10000);
</script>
</body>
</html>`
