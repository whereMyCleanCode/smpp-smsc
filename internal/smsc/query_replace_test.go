package smsc

import (
	"context"
	"testing"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdufield"
)

type queryTestHandler struct {
	queryCalled   bool
	lastMessageID string
}

func (h *queryTestHandler) HandleBindTransceiver(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeTransceiver
	return StatusOK, nil
}
func (h *queryTestHandler) HandleBindReceiver(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeReceiver
	return StatusOK, nil
}
func (h *queryTestHandler) HandleBindTransmitter(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeTransmitter
	return StatusOK, nil
}
func (h *queryTestHandler) HandleSubmitSM(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
	return &SmppResponse{Status: StatusOK}
}
func (h *queryTestHandler) HandleQuerySM(_ context.Context, params *QuerySmParams, _ *Session) (*QuerySmResponse, uint32, error) {
	h.queryCalled = true
	h.lastMessageID = params.MessageID
	return &QuerySmResponse{
		MessageID:    params.MessageID,
		FinalDate:    "",
		MessageState: 2,
		ErrorCode:    0,
	}, StatusOK, nil
}
func (h *queryTestHandler) HandleUnbind(_ context.Context, _ *Session) (uint32, error) { return StatusOK, nil }
func (h *queryTestHandler) HandleEnquireLink(_ context.Context, _ *Session) (uint32, error) { return StatusOK, nil }
func (h *queryTestHandler) HandleDeliverSMResp(_ context.Context, _, _ uint32, _ *Session) error { return nil }

func TestHandleQuerySM(t *testing.T) {
	handler := &queryTestHandler{}
	session := &Session{
		ID:          "sess-query",
		Bound:       true,
		BindingType: BindingTypeTransmitter,
		cfg:         newTestConfig(),
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 8),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}, 10),
		logger:      newTestLogger(),
		handler:     handler,
	}

	msg := pdu.NewQuerySM()
	fields := msg.Fields()
	_ = fields.Set(pdufield.MessageID, "msg123")
	_ = fields.Set(pdufield.SourceAddr, "79123456789")
	_ = fields.Set(pdufield.SourceAddrTON, 1)
	_ = fields.Set(pdufield.SourceAddrNPI, 1)

	session.processPDU(msg)

	if !handler.queryCalled {
		t.Fatal("HandleQuerySM was not called")
	}
	if handler.lastMessageID != "msg123" {
		t.Fatalf("expected message_id=msg123, got %s", handler.lastMessageID)
	}
}

func TestQuerySMRequiresBinding(t *testing.T) {
	handler := &queryTestHandler{}
	session := &Session{
		ID:          "sess-query-bound",
		Bound:       true,
		BindingType: BindingTypeTransceiver,
		cfg:         newTestConfig(),
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 8),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}, 10),
		logger:      newTestLogger(),
		handler:     handler,
	}

	msg := pdu.NewQuerySM()
	fields := msg.Fields()
	_ = fields.Set(pdufield.MessageID, "msg789")
	_ = fields.Set(pdufield.SourceAddr, "source")

	session.processPDU(msg)

	if !handler.queryCalled {
		t.Fatal("HandleQuerySM should be called for bound session")
	}
}
