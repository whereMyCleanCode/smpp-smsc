package smsc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/maypok86/otter/v2"
)

type SessController struct {
	cfg *Config

	ctx context.Context
	lgr Logger

	sessions       *SessionCache
	messageIDCache *otter.Cache[uint64, string]
	sessionIDs     sync.Map

	inactivityCheckStop chan struct{}
	inactivityCheckWg   sync.WaitGroup
}

func NewSessionsManager(ctx context.Context, logger Logger, cfg *Config) (*SessController, error) {
	sessionCacheCfg := cfg.SessionCache
	if sessionCacheCfg.InactiveTimeout == 0 {
		sessionCacheCfg.InactiveTimeout = cfg.InactivityTimeout
	}

	sessionCache, err := NewSessionCache(sessionCacheCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create session cache: %w", err)
	}

	messageIDCache, err := otter.New(&otter.Options[uint64, string]{
		MaximumSize:      100000,
		ExpiryCalculator: otter.ExpiryWriting[uint64, string](30 * time.Minute),
	})
	if err != nil {
		return nil, fmt.Errorf("create message-id cache: %w", err)
	}

	return &SessController{
		ctx:            ctx,
		lgr:            logger.WithStr("component", "sessions_manager"),
		cfg:            cfg,
		sessions:       sessionCache,
		messageIDCache: messageIDCache,
	}, nil
}

func (m *SessController) Start() {
	m.inactivityCheckStop = make(chan struct{})
	m.inactivityCheckWg.Add(1)
	go m.checkInactiveSessions()
}

func (m *SessController) Shutdown() {
	if m.inactivityCheckStop != nil {
		close(m.inactivityCheckStop)
		m.inactivityCheckWg.Wait()
	}

	if m.messageIDCache != nil {
		m.messageIDCache.InvalidateAll()
		m.messageIDCache.StopAllGoroutines()
	}

	m.sessions.Close()
	m.sessionIDs.Range(func(key, value interface{}) bool {
		m.sessionIDs.Delete(key)
		return true
	})
}

func (m *SessController) InitializeSession(session *Session) {
	m.sessions.Set(session.ID, session)
	m.sessionIDs.Store(session.ID, struct{}{})
	go session.start()
}

func (m *SessController) UpdateSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	m.sessions.Set(session.ID, session)
	return nil
}

func (m *SessController) DeleteSession(sessionID string) error {
	m.sessions.Delete(sessionID)
	m.sessionIDs.Delete(sessionID)
	return nil
}

func (m *SessController) GetSessionByID(sessionID string) (*Session, bool) {
	return m.sessions.Get(sessionID)
}

func (m *SessController) GetSessionsByPod(podID string) ([]*Session, error) {
	return m.filterSessions("pod_id", podID)
}

func (m *SessController) GetSessionsByApplicationID(applicationID string) ([]*Session, error) {
	return m.filterSessions("application_id", applicationID)
}

func (m *SessController) filterSessions(field string, value string) ([]*Session, error) {
	out := make([]*Session, 0)
	m.sessionIDs.Range(func(key, _ interface{}) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		session, exists := m.sessions.Get(sessionID)
		if !exists {
			return true
		}
		switch field {
		case "pod_id":
			if session.PodID == value {
				out = append(out, session)
			}
		case "application_id":
			if session.ApplicationID == value {
				out = append(out, session)
			}
		}
		return true
	})
	return out, nil
}

func (m *SessController) DeletePodSessions(podID string) error {
	toDelete := make([]string, 0)
	m.sessionIDs.Range(func(key, _ interface{}) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		session, exists := m.sessions.Get(sessionID)
		if exists && session.PodID == podID {
			toDelete = append(toDelete, sessionID)
		}
		return true
	})
	for _, id := range toDelete {
		_ = m.DeleteSession(id)
	}
	return nil
}

func (m *SessController) GetReceivers() []*Session {
	out := make([]*Session, 0)
	m.sessionIDs.Range(func(key, _ interface{}) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		session, exists := m.sessions.Get(sessionID)
		if exists && session.BindingType.IsReceiver() {
			out = append(out, session)
		}
		return true
	})
	return out
}

func (m *SessController) RegisterMessageID(messageID uint64, session *Session) {
	m.messageIDCache.Set(messageID, session.ID)
}

func (m *SessController) UnregisterMessageID(messageID uint64) {
	m.messageIDCache.Invalidate(messageID)
}

func (m *SessController) GetSessionByMessageID(messageID uint64) (*Session, bool) {
	sessionID, found := m.messageIDCache.GetIfPresent(messageID)
	if !found {
		return nil, false
	}
	session, exists := m.sessions.Get(sessionID)
	if !exists {
		m.messageIDCache.Invalidate(messageID)
		return nil, false
	}
	return session, true
}

func (m *SessController) checkInactiveSessions() {
	defer m.inactivityCheckWg.Done()
	interval := m.cfg.InactivityTimeout / 4
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.inactivityCheckStop:
			return
		case <-ticker.C:
			m.processInactiveSessions()
		}
	}
}

func (m *SessController) processInactiveSessions() {
	now := time.Now()
	toCheck := make([]*Session, 0)
	m.sessionIDs.Range(func(key, _ interface{}) bool {
		sessionID, ok := key.(string)
		if !ok {
			return true
		}
		session, exists := m.sessions.Get(sessionID)
		if exists && session.Bound {
			toCheck = append(toCheck, session)
		}
		return true
	})

	for _, session := range toCheck {
		m.checkSessionInactivity(session, now)
	}
}

func (m *SessController) checkSessionInactivity(session *Session, now time.Time) {
	inactiveDuration := now.Sub(session.getLastActivity())
	if inactiveDuration < m.cfg.InactivityTimeout {
		return
	}
	retryCount := session.getEnquireRetryCount()
	if retryCount >= m.cfg.MaxEnquireLinkRetry {
		_ = m.DeleteSession(session.ID)
		return
	}
	if err := session.sendEnquireLinkReq(); err != nil {
		_ = m.DeleteSession(session.ID)
	}
}
