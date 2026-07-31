package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

type Limiter struct {
	mu sync.Mutex 
	limiters map[string]*rate.Limiter 
	rps float64
	burst int 
}

func New(rps float64, burst int) *Limiter {
	return &Limiter{
		limiters: make(map[string]*rate.Limiter),
		rps: 	  rps, 
		burst: 	  burst, 
	}
}

func (l *Limiter) Allow(clientID string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[clientID]
	if !ok {
		lim = rate.NewLimiter(rate.Limit(l.rps), l.burst)
		l.limiters[clientID] = lim 
	}
	l.mu.Unlock()
	return lim.Allow()
}

func (l *Limiter) Remove(clientID string) {
	l.mu.Lock()
	delete(l.limiters, clientID)
	l.mu.Unlock()
}

