package smsc

import (
	"context"
	"testing"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
)

func TestSendDeliveryReport_FailureOnlySkipsSuccess(t *testing.T) {
	cfg := newTestConfig()
	ctx, cancel := newTestContext()
	defer cancel()

	manager, err := NewSessionsManager(ctx, newTestLogger(), cfg)
	if err != nil {
		t.Fatalf("NewSessionsManager: %v", err)
	}
	defer manager.Shutdown()

	session := &Session{
		ID:            "sess-dr-failonly",
		Bound:         true,
		BindingType:   BindingTypeTransceiver,
		cfg:           cfg,
		ctx:           ctx,
		cancel:        cancel,
		stopCh:        make(chan struct{}),
		pduQueue:      make(chan pdu.Body, 8),
		segmentsMgr:   NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:        newTestLogger(),
	}
	manager.sessions.Set(session.ID, session)
	manager.sessionIDs.Store(session.ID, struct{}{})

	msgID := uint64(1001)
	session.PendingRequests.Store(msgID, PendingRequest{
		SegmentsCount:      1,
		RegisteredDelivery: uint8(FailureOnlyReceipt),
		CreatedAt:          time.Now(),
	})

	srv := &Server{
		cfg:             cfg,
		sessionsManager: manager,
		ctx:             ctx,
		cancel:          cancel,
	}

	res, err := srv.SendDeliveryReport(context.Background(), session.ID, "7702", "7701", "mid-str", msgID, true, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res != DeliveryReportSkippedFailureOnly {
		t.Fatalf("got %v want DeliveryReportSkippedFailureOnly", res)
	}
	if _, ok := session.PendingRequests.Load(msgID); ok {
		t.Fatalf("pending should be removed after policy skip")
	}
}

func TestSendDeliveryReport_SuccessAndFailureSendsDeliverSM(t *testing.T) {
	cfg := newTestConfig()
	ctx, cancel := newTestContext()
	defer cancel()

	manager, err := NewSessionsManager(ctx, newTestLogger(), cfg)
	if err != nil {
		t.Fatalf("NewSessionsManager: %v", err)
	}
	defer manager.Shutdown()

	session := &Session{
		ID:            "sess-dr-send",
		Bound:         true,
		BindingType:   BindingTypeReceiver,
		cfg:           cfg,
		ctx:           ctx,
		cancel:        cancel,
		stopCh:        make(chan struct{}),
		pduQueue:      make(chan pdu.Body, 8),
		segmentsMgr:   NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:        newTestLogger(),
	}
	manager.sessions.Set(session.ID, session)
	manager.sessionIDs.Store(session.ID, struct{}{})

	msgID := uint64(2002)
	session.PendingRequests.Store(msgID, PendingRequest{
		SegmentsCount:      2,
		RegisteredDelivery: uint8(SuccessAndFailureReceipt),
		CreatedAt:          time.Now(),
	})

	srv := &Server{
		cfg:             cfg,
		sessionsManager: manager,
		ctx:             ctx,
		cancel:          cancel,
	}

	res, err := srv.SendDeliveryReport(context.Background(), session.ID, "7702", "7701", "ext-id", msgID, true, "")
	if err != nil {
		t.Fatalf("SendDeliveryReport: %v", err)
	}
	if res != DeliveryReportSent {
		t.Fatalf("got %v want DeliveryReportSent", res)
	}
	if _, ok := session.PendingRequests.Load(msgID); ok {
		t.Fatalf("pending should be removed after send")
	}

	select {
	case body := <-session.pduQueue:
		if body.Header().ID != pdu.DeliverSMID {
			t.Fatalf("expected deliver_sm, got %d", body.Header().ID)
		}
	default:
		t.Fatalf("expected one PDU in queue")
	}
}
