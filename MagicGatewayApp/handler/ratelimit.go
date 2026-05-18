package handler

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	maxLoginFailures = 10
	lockoutDuration  = 5 * time.Minute
)

type ipEntry struct {
	failures int
	lockedAt time.Time
}

var (
	loginMu      sync.Mutex
	loginRecords = make(map[string]*ipEntry)
)

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// Take first IP in the chain
		for i, c := range ip {
			if c == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func loginAllowed(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()

	e, ok := loginRecords[ip]
	if !ok {
		return true
	}
	if !e.lockedAt.IsZero() {
		if time.Since(e.lockedAt) >= lockoutDuration {
			delete(loginRecords, ip)
			return true
		}
		return false
	}
	return true
}

func recordFailure(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()

	e, ok := loginRecords[ip]
	if !ok {
		e = &ipEntry{}
		loginRecords[ip] = e
	}
	e.failures++
	if e.failures >= maxLoginFailures {
		e.lockedAt = time.Now()
	}
}

func recordSuccess(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginRecords, ip)
}
