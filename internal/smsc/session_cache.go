package smsc

import (
	"fmt"
	"time"

	"github.com/maypok86/otter/v2"
)

type SessionCache struct {
	cache  *otter.Cache[string, *Session]
	logger Logger
}

func NewSessionCache(config SessionCacheConfig, logger Logger) (*SessionCache, error) {
	capacity := config.Cap
	if capacity <= 0 {
		capacity = 10000
	}
	ttl := config.InactiveTimeout
	if ttl <= 0 {
		ttl = time.Hour
	}

	cache, err := otter.New(&otter.Options[string, *Session]{
		MaximumSize:      capacity,
		ExpiryCalculator: otter.ExpiryAccessing[string, *Session](ttl * 5),
		OnDeletion: func(e otter.DeletionEvent[string, *Session]) {
			if e.Value != nil {
				e.Value.Stop()
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create session cache: %w", err)
	}
	return &SessionCache{
		cache:  cache,
		logger: logger,
	}, nil
}

func (sc *SessionCache) Set(sessionID string, session *Session) {
	sc.cache.Set(sessionID, session)
}

func (sc *SessionCache) Get(sessionID string) (*Session, bool) {
	return sc.cache.GetIfPresent(sessionID)
}

func (sc *SessionCache) Delete(sessionID string) {
	sc.cache.Invalidate(sessionID)
}

func (sc *SessionCache) Close() {
	if sc.cache == nil {
		return
	}
	sc.cache.InvalidateAll()
	sc.cache.StopAllGoroutines()
	sc.cache = nil
}
