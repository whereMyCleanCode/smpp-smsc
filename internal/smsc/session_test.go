package smsc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdufield"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutlv"
)

func TestParseSubmitSMWithPayloadAndSAR(t *testing.T) {
	msg := pdu.NewSubmitSM(nil)
	fields := msg.Fields()

	_ = fields.Set(pdufield.ServiceType, "")
	_ = fields.Set(pdufield.SourceAddrTON, 1)
	_ = fields.Set(pdufield.SourceAddrNPI, 1)
	_ = fields.Set(pdufield.SourceAddr, "77010000000")
	_ = fields.Set(pdufield.DestAddrTON, 1)
	_ = fields.Set(pdufield.DestAddrNPI, 1)
	_ = fields.Set(pdufield.DestinationAddr, "77020000000")
	_ = fields.Set(pdufield.ESMClass, 0)
	_ = fields.Set(pdufield.ProtocolID, 0)
	_ = fields.Set(pdufield.PriorityFlag, 0)
	_ = fields.Set(pdufield.ScheduleDeliveryTime, "")
	_ = fields.Set(pdufield.ValidityPeriod, "")
	_ = fields.Set(pdufield.RegisteredDelivery, 1)
	_ = fields.Set(pdufield.ReplaceIfPresentFlag, 0)
	_ = fields.Set(pdufield.DataCoding, DataCodingDefault)
	_ = fields.Set(pdufield.SMDefaultMsgID, 0)
	_ = fields.Set(pdufield.ShortMessage, []byte("stub"))

	_ = msg.TLVFields().Set(pdutlv.TagMessagePayload, []byte("segment-payload"))
	_ = msg.TLVFields().Set(pdutlv.TagSarMsgRefNum, []byte{0x00, 0xAA})
	_ = msg.TLVFields().Set(pdutlv.TagSarTotalSegments, []byte{0x00, 0x02})
	_ = msg.TLVFields().Set(pdutlv.TagSarSegmentSeqnum, []byte{0x00, 0x01})

	params, status, err := parseSubmitSM(msg)
	if err != nil {
		t.Fatalf("parseSubmitSM error: %v", err)
	}
	if status != StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if !params.WithPayload {
		t.Fatalf("expected payload flag")
	}
	if string(params.ShortMessage) != "segment-payload" {
		t.Fatalf("unexpected payload: %q", string(params.ShortMessage))
	}
	if params.Segment == nil {
		t.Fatalf("expected segment from SAR TLVs")
	}
	if params.Segment.SegmentsCount != 2 || params.Segment.SegmentSeqNum != 1 {
		t.Fatalf("unexpected SAR data: total=%d seq=%d", params.Segment.SegmentsCount, params.Segment.SegmentSeqNum)
	}
}

func TestSessionHandleSubmitSMRegistersPendingRequest(t *testing.T) {
	cfg := newTestConfig()
	var registeredID uint64

	session := &Session{
		ID:          "sess-submit",
		BindingType: BindingTypeTransceiver,
		cfg:         cfg,
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 8),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:      newTestLogger(),
		registerMessageID: func(messageID uint64) {
			registeredID = messageID
		},
	}

	params := &SubmitSmParams{
		SourceAddr:         "77010000000",
		DestAddr:           "77020000000",
		ShortMessage:       []byte("hello"),
		Text:               "hello",
		DataCoding:         DataCodingDefault,
		RegisteredDelivery: 1,
		TLVParams:          map[uint16][]byte{},
	}

	messageID, response, err := session.handleSubmitSM(params, func(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
		return &SmppResponse{Status: StatusOK}
	})
	if err != nil {
		t.Fatalf("handleSubmitSM error: %v", err)
	}
	if response.Status != StatusOK {
		t.Fatalf("unexpected response status: %d", response.Status)
	}
	if messageID == 0 {
		t.Fatalf("message id should be generated")
	}
	if registeredID != messageID {
		t.Fatalf("message id registration callback mismatch: got=%d want=%d", registeredID, messageID)
	}
	if _, ok := session.PendingRequests.Load(messageID); !ok {
		t.Fatalf("pending request for message id not found")
	}
}

func TestSendEnquireLinkReq(t *testing.T) {
	cfg := newTestConfig()
	cfg.MaxEnquireLinkRetryCount = 1

	session := &Session{
		ID:          "sess-enquire",
		cfg:         cfg,
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 2),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:      newTestLogger(),
	}

	if err := session.sendEnquireLinkReq(); err != nil {
		t.Fatalf("sendEnquireLinkReq first call failed: %v", err)
	}
	if session.getEnquireRetryCount() != 1 {
		t.Fatalf("retry count mismatch: got=%d", session.getEnquireRetryCount())
	}

	select {
	case body := <-session.pduQueue:
		if body.Header().ID != pdu.EnquireLinkID {
			t.Fatalf("unexpected pdu id in queue: %v", body.Header().ID)
		}
	default:
		t.Fatalf("expected enquire_link in queue")
	}

	if err := session.sendEnquireLinkReq(); err == nil {
		t.Fatalf("expected error on retry limit")
	}
}

func TestDecodeMessageDefaultAndUCS2(t *testing.T) {
	text, err := DecodeMessage([]byte("hello"), DataCodingDefault)
	if err != nil {
		t.Fatalf("decode default: %v", err)
	}
	if text != "hello" {
		t.Fatalf("unexpected default decode: %q", text)
	}

	ucs := []byte{0x00, 0x68, 0x00, 0x69}
	text, err = DecodeMessage(ucs, DataCodingUCS2)
	if err != nil {
		t.Fatalf("decode ucs2: %v", err)
	}
	if text != "hi" {
		t.Fatalf("unexpected ucs2 decode: %q", text)
	}
}

func TestParseTLV(t *testing.T) {
	payload := []byte{
		0x04, 0x24, 0x00, 0x03, 'a', 'b', 'c',
		0x02, 0x0c, 0x00, 0x02, 0x00, 0x05,
	}
	result, err := ParseTLV(payload, 0)
	if err != nil {
		t.Fatalf("parse tlv: %v", err)
	}
	if !bytes.Equal(result[TagMessagePayload], []byte("abc")) {
		t.Fatalf("message_payload mismatch")
	}
	if len(result[TagSarMsgRefNum]) != 2 {
		t.Fatalf("sar_msg_ref_num not parsed")
	}
}
