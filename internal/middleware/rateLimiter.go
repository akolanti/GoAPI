package middleware

import (
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"golang.org/x/time/rate"
)

var limiterInstance = NewIPRateLimiter(rate.Limit(config.RATE_LIMIT_PER_SECOND), config.BURST_RATE_LIMIT_PER_SECOND)

type IPRateLimiter struct {
	ips       map[string]*rate.Limiter
	mu        sync.RWMutex
	rateLimit rate.Limit
	burstRate int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{ips: make(map[string]*rate.Limiter), rateLimit: r, burstRate: b}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()
	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.rateLimit, i.burstRate)
		i.ips[ip] = limiter
	}
	return limiter
}

//TODO: when the users grow
// I must offload this key-value to redis
