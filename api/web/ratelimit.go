package web

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	rateLimitStaleAfter = 1 * time.Hour
	rateLimitSweepEvery = 10 * time.Minute
)

// One caller's bucket, plus when we last touched it.
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Rate-limits spam that may occur from an
// evil source.
type Limiter struct {
	byIP  map[string]*client
	mut   sync.Mutex
	rate  rate.Limit
	burst int
}

// Makes a limiter that refills at r with a burst of b
// and starts its background sweeper.
func NewLimiter(r rate.Limit, b int) *Limiter {
	l := &Limiter{
		byIP:  make(map[string]*client),
		rate:  r,
		burst: b,
	}
	go l.sweepLoop()
	return l
}

// Returns the bucket for an IP or makes a new one if they're fresh.
func (l *Limiter) getOrCreate(ip string) *rate.Limiter {
	l.mut.Lock()
	defer l.mut.Unlock()

	c, ok := l.byIP[ip]
	if !ok {
		freshLimiter := rate.NewLimiter(l.rate, l.burst)
		l.byIP[ip] = &client{limiter: freshLimiter, lastSeen: time.Now()}
		return freshLimiter
	}

	c.lastSeen = time.Now()
	return c.limiter
}

// Drops callers we haven't heard from in a while.
// (Called in [limiter.sweepLoop])
func (l *Limiter) sweep() {
	l.mut.Lock()
	defer l.mut.Unlock()

	for ip, c := range l.byIP {
		if time.Since(c.lastSeen) > rateLimitStaleAfter {
			delete(l.byIP, ip)
		}
	}
}

// Sweeps on a ticker for the life of the process.
// (meant to be started using the go statement in [newLimiter])
func (l *Limiter) sweepLoop() {
	ticker := time.NewTicker(rateLimitSweepEvery)
	defer ticker.Stop()

	for range ticker.C {
		l.sweep()
	}
}

// Middleware that caps how often a single IP can hit an endpoint.
// Answers with [ErrTooManyRequests] once their bucket runs dry.
func (l *Limiter) Check(next handleError) handleError {
	return func(w http.ResponseWriter, r *http.Request) error {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = "unknown"
		}

		rateLimiter := l.getOrCreate(host)
		if !rateLimiter.Allow() {
			return ErrTooManyRequests
		}

		return next(w, r)
	}
}
