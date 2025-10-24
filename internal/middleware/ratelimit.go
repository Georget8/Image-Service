package middleware

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type visitor struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

type RateLimiter struct {
    visitors map[string]*visitor
    mu       sync.RWMutex
    rate     int
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
    rl := &RateLimiter{
        visitors: make(map[string]*visitor),
        rate:     requestsPerMinute,
    }

    go rl.cleanupVisitors()

    return rl
}

func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    v, exists := rl.visitors[ip]
    if !exists {
        limiter := rate.NewLimiter(rate.Limit(rl.rate)/60, rl.rate)
        rl.visitors[ip] = &visitor{limiter, time.Now()}
        return limiter
    }

    v.lastSeen = time.Now()
    return v.limiter
}

func (rl *RateLimiter) cleanupVisitors() {
    for {
        time.Sleep(3 * time.Minute)

        rl.mu.Lock()
        for ip, v := range rl.visitors {
            if time.Since(v.lastSeen) > 3*time.Minute {
                delete(rl.visitors, ip)
            }
        }
        rl.mu.Unlock()
    }
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := r.RemoteAddr
        limiter := rl.getVisitor(ip)

        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}