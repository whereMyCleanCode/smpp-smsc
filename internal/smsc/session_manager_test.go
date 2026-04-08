package smsc

import (
	"bufio"
	"context"
	"testing"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
)

func TestSessionManagerMessageIDRouting(t *testing.T) {
	cfg := newTestConfig()
	ctx, cancel := newTestContext()
	defer cancel()

	manager, err := NewSessionsManager(ctx, newTestLogger(), cfg)
	if err != nil {
		t.Fatalf("new sessions manager: %v", err)
	}
	defer manager.Shutdown()

	session := &Session{
		ID:            "sess-1",
		ApplicationID: "app-1",
		PodID:         "pod-1",
		Bound:         true,
		BindingType:   BindingTypeTransceiver,
		ctx:           context.Background(),
		cancel:        func() {},
		stopCh:        make(chan struct{}),
		pduQueue:      make(chan pdu.Body, 4),
		segmentsMgr:   NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}, 10),
		logger:        newTestLogger(),
		cfg:           cfg,
	}
	manager.sessions.Set(session.ID, session)
	manager.sessionIDs.Store(session.ID, struct{}{})

	manager.RegisterMessageID(42, session)
	got, ok := manager.GetSessionByMessageID(42)
	if !ok {
		t.Fatalf("message id should resolve to session")
	}
	if got.ID != "sess-1" {
		t.Fatalf("unexpected session id: %s", got.ID)
	}

	manager.UnregisterMessageID(42)
	if _, ok = manager.GetSessionByMessageID(42); ok {
		t.Fatalf("message id should be removed")
	}
}

func TestSessionManagerDeletesInactiveSessionAfterRetries(t *testing.T) {
	cfg := newTestConfig()
	cfg.InactivityTimeout = 10 * time.Millisecond
	cfg.MaxEnquireLinkRetry = 0

	ctx, cancel := newTestContext()
	defer cancel()
	manager, err := NewSessionsManager(ctx, newTestLogger(), cfg)
	if err != nil {
		t.Fatalf("new sessions manager: %v", err)
	}
	defer manager.Shutdown()

	conn := newMockConn()
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	session := &Session{
		ID:                "sess-timeout",
		Conn:              conn,
		Reader:            bufio.NewReader(conn),
		Writer:            bufio.NewWriter(conn),
		Bound:             true,
		BindingType:       BindingTypeTransceiver,
		cfg:               cfg,
		ctx:               sessionCtx,
		cancel:            sessionCancel,
		stopCh:            make(chan struct{}),
		pduQueue:          make(chan pdu.Body, 4),
		segmentsMgr:       NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}, 10),
		logger:            newTestLogger(),
		lastActivityNanos: time.Now().Add(-time.Second).UnixNano(),
	}
	manager.sessions.Set(session.ID, session)
	manager.sessionIDs.Store(session.ID, struct{}{})

	manager.checkSessionInactivity(session, time.Now())

	if _, ok := manager.GetSessionByID("sess-timeout"); ok {
		t.Fatalf("session should be deleted after max retry threshold")
	}
}
