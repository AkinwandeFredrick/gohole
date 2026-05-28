package blocklist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AkinwandeFredrick/gohole/internal/config"
)

type Engine struct {
	mu              sync.RWMutex
	blocked         map[string]string
	wildcards       map[string]string
	whitelist       map[string]bool
	customBlacklist map[string]bool
	regexRules      []*regexRule
	stats           map[string]int
	listMeta        map[string]ListMeta
}

type regexRule struct {
	pattern *regexp.Regexp
	source  string
}

type ListMeta struct {
	Name        string
	URL         string
	LastUpdated time.Time
	DomainCount int
	Enabled     bool
}

func New() *Engine {
	return &Engine{
		blocked:         make(map[string]string),
		wildcards:       make(map[string]string),
		whitelist:       make(map[string]bool),
		customBlacklist: make(map[string]bool),
		stats:           make(map[string]int),
		listMeta:        make(map[string]ListMeta),
	}
}

func (e *Engine) IsBlocked(domain string) (bool, string) {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.whitelist[domain] {
		return false, ""
	}

	if e.customBlacklist[domain] {
		return true, "custom"
	}

	if source, ok := e.blocked[domain]; ok {
		return true, source
	}

	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if source, ok := e.wildcards[parent]; ok {
			return true, source
		}
	}

	for _, rule := range e.regexRules {
		if rule.pattern.MatchString(domain) {
			return true, rule.source
		}
	}

	return false, ""
}

func (e *Engine) AddWhitelist(domain string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.whitelist[strings.ToLower(domain)] = true
}

func (e *Engine) AddCustomBlacklist(domain string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.customBlacklist[strings.ToLower(domain)] = true
}

func (e *Engine) AddRegexRule(pattern, source string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.regexRules = append(e.regexRules, &regexRule{pattern: re, source: source})
	return nil
}

func (e *Engine) LoadAll(cfg config.BlocklistConfig) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for _, src := range cfg.Sources {
		if !src.Enabled {
			continue
		}
		wg.Add(1)
		go func(src config.BlocklistSource) {
			defer wg.Done()
			if err := e.LoadSource(src); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", src.Name, err))
				mu.Unlock()
			}
		}(src)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors loading blocklists: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (e *Engine) LoadSource(src config.BlocklistSource) error {
	log.Printf("Loading blocklist: %s from %s", src.Name, src.URL)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(src.URL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	count, err := e.parseAndLoad(resp.Body, src.Format, src.Name)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.listMeta[src.Name] = ListMeta{
		Name:        src.Name,
		URL:         src.URL,
		LastUpdated: time.Now(),
		DomainCount: count,
		Enabled:     src.Enabled,
	}
	e.mu.Unlock()

	log.Printf("Loaded %d domains from %s", count, src.Name)
	return nil
}

func (e *Engine) parseAndLoad(r io.Reader, format, source string) (int, error) {
	domains := make(map[string]bool)
	wildcards := make(map[string]bool)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		var domain string
		switch format {
		case "hosts":
			domain = parseHostsLine(line)
		case "domains":
			domain = parseDomainLine(line)
		case "abp":
			domain = parseABPLine(line)
		default:
			domain = parseDomainLine(line)
		}

		if domain == "" {
			continue
		}

		domain = strings.ToLower(domain)

		if strings.HasPrefix(domain, "*.") {
			wildcards[domain[2:]] = true
		} else if isValidDomain(domain) {
			domains[domain] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Fix: correct map existence check
	for d := range domains {
		if _, exists := e.blocked[d]; !exists {
			e.blocked[d] = source
		}
	}
	for w := range wildcards {
		if _, exists := e.wildcards[w]; !exists {
			e.wildcards[w] = source
		}
	}

	return len(domains) + len(wildcards), nil
}

func parseHostsLine(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}

	ip := fields[0]
	if ip != "0.0.0.0" && ip != "127.0.0.1" && ip != "::" {
		return ""
	}

	domain := fields[1]
	if domain == "localhost" || domain == "localhost.localdomain" || domain == "broadcasthost" {
		return ""
	}

	return domain
}

func parseDomainLine(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	domain := strings.TrimSpace(line)
	domain = strings.TrimPrefix(domain, "||")
	domain = strings.TrimSuffix(domain, "^")
	return domain
}

func parseABPLine(line string) string {
	if strings.HasPrefix(line, "||") && strings.HasSuffix(line, "^") {
		domain := line[2 : len(line)-1]
		if strings.ContainsAny(domain, "/$@*") {
			return ""
		}
		return domain
	}
	return ""
}

func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	if strings.ContainsAny(domain, " \t/\\") {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}

func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lists := make([]map[string]interface{}, 0)
	for _, meta := range e.listMeta {
		lists = append(lists, map[string]interface{}{
			"name":         meta.Name,
			"domain_count": meta.DomainCount,
			"last_updated": meta.LastUpdated,
			"enabled":      meta.Enabled,
		})
	}

	return map[string]interface{}{
		"total_blocked":   len(e.blocked) + len(e.wildcards),
		"total_whitelist": len(e.whitelist),
		"total_blacklist": len(e.customBlacklist),
		"lists":           lists,
	}
}

func (e *Engine) CheckDomain(domain string) map[string]interface{} {
	blocked, source := e.IsBlocked(domain)
	return map[string]interface{}{
		"domain":      domain,
		"blocked":     blocked,
		"source":      source,
		"whitelisted": e.whitelist[strings.ToLower(domain)],
	}
}

func (e *Engine) StartAutoUpdate(ctx context.Context, cfg config.BlocklistConfig) {
	ticker := time.NewTicker(cfg.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Auto-updating blocklists...")
			if err := e.LoadAll(cfg); err != nil {
				log.Printf("Auto-update error: %v", err)
			}
		}
	}
}

func (e *Engine) TotalBlocked() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.blocked) + len(e.wildcards)
}
