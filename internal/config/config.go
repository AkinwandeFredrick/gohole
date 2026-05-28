package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DNS        DNSConfig       `yaml:"dns"`
	Dashboard  DashboardConfig `yaml:"dashboard"`
	Database   DatabaseConfig  `yaml:"database"`
	Cache      CacheConfig     `yaml:"cache"`
	Blocklists BlocklistConfig `yaml:"blocklists"`
	Whitelist  []string        `yaml:"whitelist"`
	Blacklist  []string        `yaml:"blacklist"`
}

type DNSConfig struct {
	ListenAddr string   `yaml:"listen_addr"`
	Upstream   []string `yaml:"upstream"`
	SinkholeIP string   `yaml:"sinkhole_ip"`
}

type DashboardConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type CacheConfig struct {
	MaxEntries int `yaml:"max_entries"`
}

type BlocklistConfig struct {
	AutoUpdate     bool              `yaml:"auto_update"`
	UpdateInterval time.Duration     `yaml:"update_interval"`
	Sources        []BlocklistSource `yaml:"sources"`
}

type BlocklistSource struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Format  string `yaml:"format"`
	Enabled bool   `yaml:"enabled"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		DNS: DNSConfig{
			ListenAddr: "0.0.0.0:53",
			Upstream:   []string{"1.1.1.1:53", "8.8.8.8:53"},
			SinkholeIP: "0.0.0.0",
		},
		Dashboard: DashboardConfig{
			ListenAddr: "0.0.0.0:8080",
			Username:   "admin",
			Password:   "gohole",
		},
		Database: DatabaseConfig{
			Path: "gohole.db",
		},
		Cache: CacheConfig{
			MaxEntries: 10000,
		},
		Blocklists: BlocklistConfig{
			AutoUpdate:     true,
			UpdateInterval: 24 * time.Hour,
			Sources: []BlocklistSource{
				{Name: "StevenBlack Unified", URL: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts", Format: "hosts", Enabled: true},
				{Name: "Malware Domains (URLhaus)", URL: "https://malware-filter.gitlab.io/malware-filter/urlhaus-filter-hosts.txt", Format: "hosts", Enabled: true},
				{Name: "AdGuard DNS Filter", URL: "https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt", Format: "abp", Enabled: true},
			},
		},
	}
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
