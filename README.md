<div align="center">

<img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go" />
<img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" />
<img src="https://img.shields.io/badge/platform-Linux%20%7C%20Docker%20%7C%20Raspberry%20Pi-lightgrey?style=for-the-badge" />

# GoHole

### A Pi-hole-style DNS sinkhole written in Go

Block **258,000+** ad, tracker, malware and C2 domains at the network level —  
before a single packet leaves your device.

</div>

---

## How It Works

Every time your device visits a website, it first asks a DNS server *"what's the IP for this domain?"* GoHole sits between your devices and the internet, intercepting those DNS queries. If the domain is on a blocklist, GoHole returns `0.0.0.0` — the request dies before it even leaves your network. For legitimate domains, it forwards the query to Cloudflare or Google DNS as normal.

```
Your Device
    │
    │ DNS query: "what's the IP for doubleclick.net?"
    ▼
┌─────────────────────────────┐
│         GoHole              │
│                             │
│  Blocklist check (O(1))     │
│       │                     │
│       ├── BLOCKED? → 0.0.0.0 (ad never loads)
│       │                     │
│       └── ALLOWED? → forward to 1.1.1.1 → real IP
│                             │
│  Every query logged to DB   │
│  Live dashboard on :8080    │
└─────────────────────────────┘
```

---

## Features

- **DNS server** — UDP + TCP on port 53, handles A, AAAA, CNAME, MX, TXT
- **258K+ domains blocked** out of the box (StevenBlack + AdGuard + URLhaus malware)
- **Wildcard blocking** — `*.tracker.com` blocks all subdomains
- **Whitelist / Blacklist** — per-domain overrides to fix false positives
- **Query logging** — every DNS request logged to SQLite with source IP, latency, block reason
- **Live web dashboard** — stats, query log, timeline chart, top blocked domains
- **DNS response cache** — TTL-aware, reduces upstream queries
- **Basic auth** — username/password protected dashboard
- **Auto-updates** — blocklists refresh every 24h automatically
- **Deployable** — Docker, systemd, Raspberry Pi, cloud (EC2/VPS)

---

## Who Can Use It — Deployment Models

GoHole can be used in three different ways depending on your setup:

### Option A: Personal device (localhost)
Run GoHole on your own machine and point only your device at it. No router changes needed.

```
Your laptop → GoHole (127.0.0.1:53) → internet
```

**Setup:** Change your system DNS to `127.0.0.1`
- **Windows:** Settings → Network → DNS → `127.0.0.1`
- **macOS:** System Settings → Network → DNS → `127.0.0.1`
- **Linux:** `/etc/resolv.conf` → `nameserver 127.0.0.1`

---

### Option B: Home network (router — recommended)
Run GoHole on any device on your home network (a Raspberry Pi is ideal). Set your router's DNS to point at it. Every device on your WiFi is automatically protected — phones, TVs, laptops, everything — with no changes on each device.

```
All home devices → Router (DHCP) → GoHole (192.168.x.x:53) → internet
```

**Setup:**
1. Run GoHole on a Raspberry Pi or always-on machine
2. Log into your router (usually `192.168.1.1`)
3. Find **DHCP / DNS settings**
4. Set **Primary DNS** to the local IP of your GoHole machine
5. All devices get the DNS automatically on reconnect

---

### Option C: Cloud / EC2 (shared or personal VPS)
Run GoHole on a cloud server. Users can manually point their device DNS to your server's public IP to use it from anywhere. This is how services like NextDNS work.

```
Any device anywhere → GoHole (your-server-ip:53) → internet
```

**Setup on EC2:**
```bash
# Free port 53 from systemd-resolved
sudo systemctl stop systemd-resolved
sudo systemctl disable systemd-resolved
echo "nameserver 1.1.1.1" | sudo tee /etc/resolv.conf

# Run GoHole (keep alive after SSH disconnect)
nohup sudo ./gohole -config config.yaml > gohole.log 2>&1 &

# Or use screen
screen -S gohole
sudo ./gohole -config config.yaml
# Ctrl+A, D to detach
```

Open these ports in your security group:
| Port | Protocol | Purpose |
|------|----------|---------|
| 53 | UDP | DNS queries |
| 53 | TCP | DNS queries (large responses) |
| 8080 | TCP | Web dashboard |

Users then set their device DNS to your server's public IP.

> **Note:** Running a public DNS resolver has abuse potential. Consider restricting port 53 to trusted IPs in your security group if you only want specific users.

---

### Option D: Self-host (anyone can run their own)
Anyone can clone and run their own GoHole instance. Change the credentials in `config.yaml` or use environment variables:

```bash
git clone https://github.com/AkinwandeFredrick/gohole.git
cd gohole
go mod tidy
make build
GOHOLE_USER=myuser GOHOLE_PASS=mypassword sudo ./gohole -config config.yaml
```

---

## Quick Start

### Prerequisites

```bash
# Go 1.22+
go version

# GCC (for SQLite)
sudo apt install gcc libsqlite3-dev   # Ubuntu/Debian
brew install gcc                       # macOS
```

### Install & Run

```bash
git clone https://github.com/AkinwandeFredrick/gohole.git
cd gohole
go mod tidy
make build
sudo ./gohole -config config.yaml
```

Dashboard opens at **http://localhost:8080**

Default login: `admin` / `changeme123` — **change this before exposing to a network**

---

## Configuration

```yaml
# config.yaml

dns:
  listen_addr: "0.0.0.0:53"
  upstream:
    - "1.1.1.1:53"     # Cloudflare (primary)
    - "8.8.8.8:53"     # Google (fallback)
  sinkhole_ip: "0.0.0.0"

dashboard:
  listen_addr: "0.0.0.0:8080"
  username: "admin"
  password: "changeme123"   # Change this!

blocklists:
  auto_update: true
  update_interval: 24h
  sources:
    - name: "StevenBlack Unified"
      url: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
      format: "hosts"
      enabled: true
    - name: "Malware Domains (URLhaus)"
      url: "https://malware-filter.gitlab.io/malware-filter/urlhaus-filter-hosts.txt"
      format: "hosts"
      enabled: true
    - name: "AdGuard DNS Filter"
      url: "https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt"
      format: "abp"
      enabled: true

# Never block these (fix false positives)
whitelist:
  # - "safe.example.com"

# Always block these (custom additions)
blacklist:
  # - "bad-domain.com"
```

### Environment variable overrides

```bash
GOHOLE_USER=admin GOHOLE_PASS=mysecretpassword sudo ./gohole
```

---

## Verifying It's Actually Working

### 1. Test with dig

```bash
# Replace 127.0.0.1 with your GoHole server IP if remote

# Should return 0.0.0.0 (BLOCKED)
dig @127.0.0.1 doubleclick.net A
dig @127.0.0.1 googleadservices.com A
dig @127.0.0.1 analytics.tiktok.com A

# Should return real IP (ALLOWED)
dig @127.0.0.1 google.com A
dig @127.0.0.1 github.com A
```

Expected output for a blocked domain:
```
;; ANSWER SECTION:
doubleclick.net.    300    IN    A    0.0.0.0
```

### 2. Domain Check page

Dashboard → **Domain Check** → type any domain → instantly see if it's blocked and which list caught it.

### 3. Watch the Query Log

Dashboard → **Query Log** → enable Auto-refresh → open a website in another tab → watch requests stream in, red for blocked, green for allowed.

### 4. Known blocked domains to test

```
doubleclick.net              ← Google Ads
googleadservices.com         ← Google Ads
pagead2.googlesyndication.com ← Google AdSense
ads.yahoo.com                ← Yahoo Ads
tracking.twitter.com         ← Twitter tracker
analytics.tiktok.com         ← TikTok analytics
static.ads-twitter.com       ← Twitter Ads
```

### 5. Real-world test

Set your device DNS to your GoHole IP, visit a news website (e.g. CNN, DailyMail), and check the dashboard Query Log. You should see dozens of ad/tracker domains being blocked in real time.

---

## Docker

```bash
docker compose -f deployments/docker/docker-compose.yml up -d
```

To set credentials via Docker:
```yaml
# docker-compose.yml
environment:
  - GOHOLE_USER=admin
  - GOHOLE_PASS=mysecretpassword
```

---

## Raspberry Pi (recommended home setup)

```bash
# Install Go on Pi
wget https://go.dev/dl/go1.22.0.linux-arm64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Clone and build
git clone https://github.com/AkinwandeFredrick/gohole.git
cd gohole
sudo apt install gcc libsqlite3-dev
go mod tidy
make build

# Install as a service (runs on boot)
make install
sudo systemctl start gohole
```

---

## Systemd (Linux service)

```bash
make install
sudo systemctl start gohole
sudo systemctl status gohole
sudo journalctl -u gohole -f   # live logs
```

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `address already in use` on port 53 | `sudo systemctl stop systemd-resolved && sudo systemctl disable systemd-resolved` |
| Website broken after setting DNS | Use Domain Check to find the false positive, add to `whitelist` in config |
| Dashboard not loading | Check GoHole is running: `ps aux \| grep gohole`. Check port 8080 is open in firewall |
| `go.sum` missing on build | Run `go mod tidy` first |
| DNS not resolving at all | Confirm upstream DNS is reachable: `dig @1.1.1.1 google.com` |

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    GoHole Process                    │
│                                                      │
│  ┌─────────────┐   ┌──────────────────────────────┐ │
│  │  DNS Server  │   │      Blocklist Engine        │ │
│  │  UDP + TCP   │──▶│  hash map  +  wildcards      │ │
│  │  port 53     │   │  regex rules                 │ │
│  └──────┬───────┘   └──────────────────────────────┘ │
│         │           ┌──────────────────────────────┐ │
│         ├──────────▶│   DNS Response Cache         │ │
│         │           │   TTL-aware, 10K entries     │ │
│         │           └──────────────────────────────┘ │
│         │           ┌──────────────────────────────┐ │
│         └──────────▶│   Query Logger               │ │
│                     │   async batched SQLite writes│ │
│  ┌─────────────┐    └──────────────────────────────┘ │
│  │  Dashboard   │                                    │
│  │  HTTP :8080  │◀── REST API + embedded HTML UI     │
│  │  Basic Auth  │                                    │
│  └─────────────┘                                     │
└─────────────────────────────────────────────────────┘
              │ (non-blocked domains)
              ▼
       1.1.1.1 / 8.8.8.8
```

---

## API Reference

All endpoints require Basic Auth (`Authorization: Basic <base64(user:pass)>`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/stats?since=24h` | Query, blocklist, and cache statistics |
| GET | `/api/queries?limit=100` | Recent query log |
| GET | `/api/top-domains?type=blocked&limit=10` | Top domains by query count |
| GET | `/api/top-clients?limit=10` | Most active client IPs |
| GET | `/api/blocklists` | Blocklist metadata and domain counts |
| GET | `/api/check?domain=ads.example.com` | Check if a domain is blocked |
| GET | `/api/cache` | Cache hit/miss statistics |
| DELETE | `/api/cache` | Flush the DNS response cache |
| GET | `/api/timeline?buckets=24&since=24h` | Query volume over time (for chart) |

---

## Contributing

PRs welcome. Open issues or suggestions:

- DNSSEC validation
- DNS-over-HTTPS (DoH) upstream support
- Per-client blocklist profiles
- Prometheus metrics endpoint
- TLS / HTTPS for dashboard

---

## License

MIT — free to use, modify, and deploy.

---

<div align="center">
Built by <a href="https://github.com/AkinwandeFredrick">AkinwandeFredrick</a>
</div>
