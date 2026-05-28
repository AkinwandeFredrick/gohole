package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/AkinwandeFredrick/gohole/internal/blocklist"
	"github.com/AkinwandeFredrick/gohole/internal/cache"
	"github.com/AkinwandeFredrick/gohole/internal/logger"
)

type Config struct {
	ListenAddr  string
	QueryLogger *logger.Logger
	Blocklist   *blocklist.Engine
	Cache       *cache.Cache
	Username    string
	Password    string
}

type Dashboard struct {
	cfg Config
	srv *http.Server
}

func New(cfg Config) *Dashboard {
	// Default credentials if not set
	if cfg.Username == "" {
		cfg.Username = getEnv("GOHOLE_USER", "admin")
	}
	if cfg.Password == "" {
		cfg.Password = getEnv("GOHOLE_PASS", "gohole")
	}
	return &Dashboard{cfg: cfg}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (d *Dashboard) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != d.cfg.Username || pass != d.cfg.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="GoHole Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (d *Dashboard) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// All routes protected by basic auth
	mux.HandleFunc("/api/stats", d.basicAuth(d.handleStats))
	mux.HandleFunc("/api/queries", d.basicAuth(d.handleQueries))
	mux.HandleFunc("/api/top-domains", d.basicAuth(d.handleTopDomains))
	mux.HandleFunc("/api/top-clients", d.basicAuth(d.handleTopClients))
	mux.HandleFunc("/api/blocklists", d.basicAuth(d.handleBlocklists))
	mux.HandleFunc("/api/check", d.basicAuth(d.handleCheck))
	mux.HandleFunc("/api/cache", d.basicAuth(d.handleCache))
	mux.HandleFunc("/api/timeline", d.basicAuth(d.handleTimeline))
	mux.HandleFunc("/", d.basicAuth(d.handleDashboard))

	d.srv = &http.Server{
		Addr:    d.cfg.ListenAddr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", d.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("dashboard failed to bind %s: %w", d.cfg.ListenAddr, err)
	}

	log.Printf("Dashboard listening on http://%s (auth enabled)", d.cfg.ListenAddr)
	log.Printf("Login: %s / %s", d.cfg.Username, d.cfg.Password)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.srv.Shutdown(shutdownCtx)
	}()

	if err := d.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("dashboard serve error: %w", err)
	}
	return nil
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	since := parseDuration(r.URL.Query().Get("since"), 24*time.Hour)
	stats, err := d.cfg.QueryLogger.GetStats(since)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]interface{}{
		"query_stats":     stats,
		"blocklist_stats": d.cfg.Blocklist.Stats(),
		"cache_stats":     d.cfg.Cache.Stats(),
	})
}

func (d *Dashboard) handleQueries(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	queries, err := d.cfg.QueryLogger.GetRecentQueries(limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, queries)
}

func (d *Dashboard) handleTopDomains(w http.ResponseWriter, r *http.Request) {
	rt := r.URL.Query().Get("type")
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	since := parseDuration(r.URL.Query().Get("since"), 24*time.Hour)
	domains, err := d.cfg.QueryLogger.GetTopDomains(rt, limit, since)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, domains)
}

func (d *Dashboard) handleTopClients(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	since := parseDuration(r.URL.Query().Get("since"), 24*time.Hour)
	clients, err := d.cfg.QueryLogger.GetTopClients(limit, since)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, clients)
}

func (d *Dashboard) handleBlocklists(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, d.cfg.Blocklist.Stats())
}

func (d *Dashboard) handleCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "domain parameter required", 400)
		return
	}
	jsonResponse(w, d.cfg.Blocklist.CheckDomain(domain))
}

func (d *Dashboard) handleCache(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		d.cfg.Cache.Flush()
		jsonResponse(w, map[string]string{"status": "flushed"})
		return
	}
	jsonResponse(w, d.cfg.Cache.Stats())
}

func (d *Dashboard) handleTimeline(w http.ResponseWriter, r *http.Request) {
	buckets := parseIntDefault(r.URL.Query().Get("buckets"), 24)
	since := parseDuration(r.URL.Query().Get("since"), 24*time.Hour)
	data, err := d.cfg.QueryLogger.GetQueriesOverTime(buckets, since)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, data)
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
