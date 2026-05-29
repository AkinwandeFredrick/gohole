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

// ── API handlers ─────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := d.cfg.QueryLogger.GetStats(24 * time.Hour)
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	cacheSize, _ := d.cfg.Cache.Size()
	hitRate, _ := d.cfg.Cache.HitRate()

	writeJSON(w, map[string]any{
		"total_queries":   stats.TotalQueries,
		"blocked_queries": stats.BlockedQueries,
		"blocked_percent": stats.PercentBlocked,
		"total_domains":   d.cfg.Blocklist.TotalBlocked(),
		"cache_size":      cacheSize,
		"cache_hit_rate":  hitRate,
	})
}

func (d *Dashboard) handleQueries(w http.ResponseWriter, r *http.Request) {
	queries, err := d.cfg.QueryLogger.GetRecentQueries(100)
	if err != nil {
		http.Error(w, "Failed to get queries", http.StatusInternalServerError)
		return
	}
	writeJSON(w, queries)
}

func (d *Dashboard) handleTopDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := d.cfg.QueryLogger.GetTopDomains("blocked", 10, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to get top domains", http.StatusInternalServerError)
		return
	}
	writeJSON(w, domains)
}

func (d *Dashboard) handleTopClients(w http.ResponseWriter, r *http.Request) {
	clients, err := d.cfg.QueryLogger.GetTopClients(10, 24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to get top clients", http.StatusInternalServerError)
		return
	}
	writeJSON(w, clients)
}

func (d *Dashboard) handleBlocklists(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, d.cfg.Blocklist.GetSources())
}

func (d *Dashboard) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	blocked, source := d.cfg.Blocklist.IsBlocked(domain)
	writeJSON(w, map[string]any{"domain": domain, "blocked": blocked, "source": source})
}

func (d *Dashboard) handleCache(w http.ResponseWriter, r *http.Request) {
	size, _ := d.cfg.Cache.Size()
	hitRate, _ := d.cfg.Cache.HitRate()
	writeJSON(w, map[string]any{
		"size":     size,
		"hit_rate": hitRate,
	})
}

func (d *Dashboard) handleTimeline(w http.ResponseWriter, r *http.Request) {
	minutes := 60 // Hardcoded default, no strconv needed
	data, err := d.cfg.QueryLogger.GetTimeline(minutes)
	if err != nil {
		data = []map[string]interface{}{}
	}
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

// ── Embedded HTML ─────────────────────────────────────────────────────────

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

