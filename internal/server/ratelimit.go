package server

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/backendArchitect/go-boilerplate/internal/api"
)

// rateLimit applies RATE_LIMIT_RPS per client IP to every request, plus the
// stricter AUTH_RATE_LIMIT_RPM to /api/v1/auth/*, where passwords are checked.
func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP(r)
		l := s.globalLimit
		ok := l.allow(ip)
		if ok && strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			l = s.authLimit
			ok = l.allow(ip)
		}
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(1/float64(l.limit)))))
			s.writeError(w, r, errorf(api.ErrorCodeRateLimited, "too many requests, retry later"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP is the caller's IP: from CLIENT_IP_HEADER when behind a trusted
// proxy, otherwise the TCP peer. It takes the last value of the header, the
// one the proxy added, because clients can forge everything before it.
func (s *Server) clientIP(r *http.Request) string {
	if h := s.cfg.ClientIPHeader; h != "" {
		if vals := r.Header.Values(h); len(vals) > 0 {
			v := vals[len(vals)-1]
			return strings.TrimSpace(v[strings.LastIndexByte(v, ',')+1:])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipLimiter is a token bucket per client IP. A zero limit disables it.
// ponytail: buckets live in this process, so N replicas allow N times the
// limit; move limiting to the load balancer or Redis when that matters.
type ipLimiter struct {
	limit   rate.Limit
	burst   int
	mu      sync.Mutex
	clients map[string]*ipClient
	swept   time.Time
}

type ipClient struct {
	bucket *rate.Limiter
	seen   time.Time
}

func newIPLimiter(limit rate.Limit, burst int) *ipLimiter {
	return &ipLimiter{limit: limit, burst: burst, clients: map[string]*ipClient{}}
}

func (l *ipLimiter) allow(ip string) bool {
	if l.limit <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Forget clients idle for a minute; their buckets are full again by then,
	// so this bounds memory without changing behavior.
	if now.Sub(l.swept) > time.Minute {
		for ip, c := range l.clients {
			if now.Sub(c.seen) > time.Minute {
				delete(l.clients, ip)
			}
		}
		l.swept = now
	}
	c := l.clients[ip]
	if c == nil {
		c = &ipClient{bucket: rate.NewLimiter(l.limit, l.burst)}
		l.clients[ip] = c
	}
	c.seen = now
	return c.bucket.AllowN(now, 1)
}
