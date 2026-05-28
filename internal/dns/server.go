package dns

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/AkinwandeFredrick/gohole/internal/blocklist"
	"github.com/AkinwandeFredrick/gohole/internal/cache"
	"github.com/AkinwandeFredrick/gohole/internal/logger"
)

type Config struct {
	ListenAddr  string
	Upstream    []string
	Blocklist   *blocklist.Engine
	Cache       *cache.Cache
	QueryLogger *logger.Logger
	SinkholIP   string
}

type Server struct {
	cfg    Config
	udpSrv *dns.Server
	tcpSrv *dns.Server
}

func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.udpSrv = &dns.Server{
		Addr:    s.cfg.ListenAddr,
		Net:     "udp",
		Handler: mux,
	}
	s.tcpSrv = &dns.Server{
		Addr:    s.cfg.ListenAddr,
		Net:     "tcp",
		Handler: mux,
	}

	errCh := make(chan error, 2)

	go func() {
		if err := s.udpSrv.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("UDP: %w", err)
		}
	}()

	go func() {
		if err := s.tcpSrv.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("TCP: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		s.udpSrv.Shutdown()
		s.tcpSrv.Shutdown()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()

	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	domain := strings.ToLower(strings.TrimSuffix(q.Name, "."))

	// Get source IP
	var srcIP string
	if addr := w.RemoteAddr(); addr != nil {
		srcIP, _, _ = net.SplitHostPort(addr.String())
	}

	blocked, listName := s.cfg.Blocklist.IsBlocked(domain)

	var responseType string
	var msg *dns.Msg

	if blocked {
		msg = s.sinkholeResponse(r, q)
		responseType = "blocked"
	} else {
		// Check cache first
		cacheKey := fmt.Sprintf("%s:%d", q.Name, q.Qtype)
		if cached := s.cfg.Cache.Get(cacheKey); cached != nil {
			msg = cached.(*dns.Msg).Copy()
			msg.Id = r.Id
			responseType = "cached"
		} else {
			// Forward to upstream
			var err error
			msg, err = s.forwardQuery(r)
			if err != nil {
				log.Printf("Upstream error for %s: %v", domain, err)
				dns.HandleFailed(w, r)
				s.logQuery(domain, q.Qtype, srcIP, "error", "", time.Since(start))
				return
			}
			// Cache the response
			if msg.Rcode == dns.RcodeSuccess && len(msg.Answer) > 0 {
				ttl := getMinTTL(msg)
				s.cfg.Cache.SetWithTTL(cacheKey, msg, time.Duration(ttl)*time.Second)
			}
			responseType = "allowed"
		}
	}

	elapsed := time.Since(start)
	w.WriteMsg(msg)

	s.logQuery(domain, q.Qtype, srcIP, responseType, listName, elapsed)
}

func (s *Server) sinkholeResponse(r *dns.Msg, q dns.Question) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.RecursionAvailable = true
	msg.Authoritative = true

	switch q.Qtype {
	case dns.TypeA:
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(s.cfg.SinkholIP).To4(),
		}
		msg.Answer = append(msg.Answer, rr)
	case dns.TypeAAAA:
		rr := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			AAAA: net.IPv6zero,
		}
		msg.Answer = append(msg.Answer, rr)
	default:
		msg.Rcode = dns.RcodeNameError
	}

	return msg
}

func (s *Server) forwardQuery(r *dns.Msg) (*dns.Msg, error) {
	client := &dns.Client{
		Timeout: 5 * time.Second,
	}

	// Pick a random upstream for load balancing
	upstream := s.cfg.Upstream[rand.Intn(len(s.cfg.Upstream))]

	msg, _, err := client.Exchange(r, upstream)
	if err != nil {
		// Try other upstreams on failure
		for _, up := range s.cfg.Upstream {
			if up == upstream {
				continue
			}
			msg, _, err = client.Exchange(r, up)
			if err == nil {
				return msg, nil
			}
		}
		return nil, err
	}

	return msg, nil
}

func (s *Server) logQuery(domain string, qtype uint16, srcIP, responseType, listName string, elapsed time.Duration) {
	s.cfg.QueryLogger.Log(logger.QueryRecord{
		Timestamp:    time.Now(),
		Domain:       domain,
		QueryType:    dns.TypeToString[qtype],
		SourceIP:     srcIP,
		ResponseType: responseType,
		ListName:     listName,
		LatencyMs:    elapsed.Milliseconds(),
	})
}

func getMinTTL(msg *dns.Msg) uint32 {
	var minTTL uint32 = 3600
	for _, rr := range msg.Answer {
		if ttl := rr.Header().Ttl; ttl < minTTL {
			minTTL = ttl
		}
	}
	if minTTL < 60 {
		minTTL = 60
	}
	return minTTL
}
