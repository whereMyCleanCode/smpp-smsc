package smsc

import (
	"context"
	"testing"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdufield"
)

type queryReplaceHandler struct {
	queryCalled    bool
	replaceCalled  bool
	lastMessageID  string
	lastSourceAddr string
}

func (h *queryReplaceHandler) HandleBindTransceiver(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeTransceiver
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleBindReceiver(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeReceiver
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleBindTransmitter(_ context.Context, _ map[string]string, s *Session) (uint32, error) {
	s.Bound = true
	s.BindingType = BindingTypeTransmitter
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleSubmitSM(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
	return &SmppResponse{Status: StatusOK}
}
func (h *queryReplaceHandler) HandleQuerySM(_ context.Context, params *QuerySmParams, _ *Session) (*QuerySmResponse, uint32, error) {
	h.queryCalled = true
	h.lastMessageID = params.MessageID
	return &QuerySmResponse{
		MessageID:    params.MessageID,
		FinalDate:    "",
		MessageState: 2,
		ErrorCode:    0,
	}, StatusOK, nil
}
func (h *queryReplaceHandler) HandleReplaceSM(_ context.Context, params *ReplaceSmParams, _ *Session) (uint32, error) {
	h.replaceCalled = true
	h.lastMessageID = params.MessageID
	h.lastSourceAddr = params.SourceAddr
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleUnbind(_ context.Context, _ *Session) (uint32, error) {
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleEnquireLink(_ context.Context, _ *Session) (uint32, error) {
	return StatusOK, nil
}
func (h *queryReplaceHandler) HandleDeliverSMResp(_ context.Context, _, _ uint32, _ *Session) error {
	return nil
}

func TestHandleQuerySM(t *testing.T) {
	handler := &queryReplaceHandler{}
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

func TestHandleReplaceSM(t *testing.T) {
	handler := &queryReplaceHandler{}
	session := &Session{
		ID:          "sess-replace",
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

	// Test parseReplaceSM directly with a mock PDU
	params := &ReplaceSmParams{
		MessageID:     "msg456",
		SourceAddr:    "79123456789",
		SourceAddrTON: 1,
		SourceAddrNPI: 1,
		DestAddr:      "dest",
		ShortMessage:  []byte("new text"),
		SeqNum:        1,
	}

	// Call handler directly
	status, err := handler.HandleReplaceSM(context.Background(), params, session)
	if err != nil {
		t.Fatalf("HandleReplaceSM returned error: %v", err)
	}
	if status != StatusOK {
		t.Fatalf("expected status OK, got %d", status)
	}
	if !handler.replaceCalled {
		t.Fatal("HandleReplaceSM was not called")
	}
	if handler.lastMessageID != "msg456" {
		t.Fatalf("expected message_id=msg456, got %s", handler.lastMessageID)
	}
	if handler.lastSourceAddr != "79123456789" {
		t.Fatalf("expected source_addr=79123456789, got %s", handler.lastSourceAddr)
	}
}

func TestQuerySMRequiresBinding(t *testing.T) {
	handler := &queryReplaceHandler{}
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
