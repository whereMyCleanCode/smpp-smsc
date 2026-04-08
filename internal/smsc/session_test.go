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
	if params.ServiceType != "" {
		t.Fatalf("unexpected service type: %q", params.ServiceType)
	}
	if params.SourceAddrTON != 1 || params.SourceAddrNPI != 1 {
		t.Fatalf("unexpected source ton/npi: ton=%d npi=%d", params.SourceAddrTON, params.SourceAddrNPI)
	}
	if params.DestAddrTON != 1 || params.DestAddrNPI != 1 {
		t.Fatalf("unexpected destination ton/npi: ton=%d npi=%d", params.DestAddrTON, params.DestAddrNPI)
	}
	if params.ESMClass != 0 || params.ProtocolID != 0 || params.PriorityFlag != 0 {
		t.Fatalf("unexpected protocol flags: esm=%d protocol=%d priority=%d", params.ESMClass, params.ProtocolID, params.PriorityFlag)
	}
	if params.ScheduleDeliveryTime != "" || params.ValidityPeriod != "" {
		t.Fatalf("unexpected schedule/validity values")
	}
	if params.ReplaceIfPresentFlag != 0 || params.SMDefaultMsgID != 0 {
		t.Fatalf("unexpected replace/sm_default_msg_id values")
	}
	if params.RegisteredDelivery != 1 {
		t.Fatalf("unexpected registered delivery: %d", params.RegisteredDelivery)
	}
	if params.DataCoding != DataCodingDefault {
		t.Fatalf("unexpected data coding: %d", params.DataCoding)
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
	if got, ok := params.TLVParams[uint16(pdutlv.TagMessagePayload)]; !ok || string(got) != "segment-payload" {
		t.Fatalf("raw message_payload tlv missing or invalid")
	}
	if got, ok := params.TLVParams[uint16(pdutlv.TagSarMsgRefNum)]; !ok || len(got) != 2 {
		t.Fatalf("raw sar_msg_ref_num tlv missing or invalid")
	}
}

func TestHandleSubmitSMCallbackReceivesExpandedSubmitParams(t *testing.T) {
	msg := pdu.NewSubmitSM(nil)
	fields := msg.Fields()

	_ = fields.Set(pdufield.ServiceType, "CMT")
	_ = fields.Set(pdufield.SourceAddrTON, 5)
	_ = fields.Set(pdufield.SourceAddrNPI, 0)
	_ = fields.Set(pdufield.SourceAddr, "MyBrand")
	_ = fields.Set(pdufield.DestAddrTON, 1)
	_ = fields.Set(pdufield.DestAddrNPI, 1)
	_ = fields.Set(pdufield.DestinationAddr, "77020000000")
	_ = fields.Set(pdufield.ESMClass, 0x40)
	_ = fields.Set(pdufield.ProtocolID, 3)
	_ = fields.Set(pdufield.PriorityFlag, 1)
	_ = fields.Set(pdufield.ScheduleDeliveryTime, "000000000100000R")
	_ = fields.Set(pdufield.ValidityPeriod, "000000000200000R")
	_ = fields.Set(pdufield.RegisteredDelivery, 1)
	_ = fields.Set(pdufield.ReplaceIfPresentFlag, 1)
	_ = fields.Set(pdufield.DataCoding, DataCodingDefault)
	_ = fields.Set(pdufield.SMDefaultMsgID, 7)
	_ = fields.Set(pdufield.ShortMessage, []byte("hello"))
	_ = msg.TLVFields().Set(pdutlv.TagMessagePayload, []byte("hello from tlv"))

	params, status, err := parseSubmitSM(msg)
	if err != nil {
		t.Fatalf("parseSubmitSM error: %v", err)
	}
	if status != StatusOK {
		t.Fatalf("unexpected parse status: %d", status)
	}

	session := &Session{
		ID:          "sess-submit-expanded",
		BindingType: BindingTypeTransceiver,
		cfg:         newTestConfig(),
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 8),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:      newTestLogger(),
	}

	_, response, handleErr := session.handleSubmitSM(params, func(_ context.Context, got *SubmitSmParams, _ *Session) *SmppResponse {
		if got.ServiceType != "CMT" ||
			got.SourceAddrTON != 5 ||
			got.SourceAddrNPI != 0 ||
			got.SourceAddr != "MyBrand" ||
			got.DestAddrTON != 1 ||
			got.DestAddrNPI != 1 ||
			got.DestAddr != "77020000000" ||
			got.ESMClass != 0x40 ||
			got.ProtocolID != 3 ||
			got.PriorityFlag != 1 ||
			got.ScheduleDeliveryTime != "000000000100000R" ||
			got.ValidityPeriod != "000000000200000R" ||
			got.RegisteredDelivery != 1 ||
			got.ReplaceIfPresentFlag != 1 ||
			got.DataCoding != DataCodingDefault ||
			got.SMDefaultMsgID != 7 {
			t.Fatalf("callback got unexpected parsed submit_sm fields")
		}
		if raw, ok := got.TLVParams[uint16(pdutlv.TagMessagePayload)]; !ok || string(raw) != "hello from tlv" {
			t.Fatalf("callback did not receive raw tlv payload")
		}
		return &SmppResponse{Status: StatusOK}
	})
	if handleErr != nil {
		t.Fatalf("handleSubmitSM error: %v", handleErr)
	}
	if response.Status != StatusOK {
		t.Fatalf("unexpected response status: %d", response.Status)
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

func TestSessionHandleSubmitSMReplaceIfPresentSupersedesPending(t *testing.T) {
	cfg := newTestConfig()
	var lastRegistered uint64

	session := &Session{
		ID:          "sess-replace",
		BindingType: BindingTypeTransceiver,
		cfg:         cfg,
		ctx:         context.Background(),
		cancel:      func() {},
		stopCh:      make(chan struct{}),
		pduQueue:    make(chan pdu.Body, 8),
		segmentsMgr: NewSegmentsManager(newTestLogger(), time.Minute, &stubIDGenerator{}),
		logger:      newTestLogger(),
		registerMessageID: func(messageID uint64) {
			lastRegistered = messageID
		},
	}

	base := SubmitSmParams{
		ServiceType:          "",
		SourceAddr:           "77010000000",
		DestAddr:             "77020000000",
		ShortMessage:         []byte("one"),
		Text:                 "one",
		DataCoding:           DataCodingDefault,
		RegisteredDelivery:   uint8(SuccessAndFailureReceipt),
		ReplaceIfPresentFlag: 0,
		SMDefaultMsgID:       0,
		TLVParams:            map[uint16][]byte{},
	}

	id1, resp1, err1 := session.handleSubmitSM(&base, func(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
		return &SmppResponse{Status: StatusOK}
	})
	if err1 != nil || resp1.Status != StatusOK {
		t.Fatalf("first submit: err=%v status=%d", err1, resp1.Status)
	}
	if _, ok := session.PendingRequests.Load(id1); !ok {
		t.Fatalf("first message should have pending DLR tracking")
	}

	second := base
	second.ShortMessage = []byte("two")
	second.Text = "two"
	second.ReplaceIfPresentFlag = 1

	id2, resp2, err2 := session.handleSubmitSM(&second, func(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
		return &SmppResponse{Status: StatusOK}
	})
	if err2 != nil || resp2.Status != StatusOK {
		t.Fatalf("second submit: err=%v status=%d", err2, resp2.Status)
	}
	if id2 == id1 {
		t.Fatalf("expected new message id on replace")
	}
	if _, ok := session.PendingRequests.Load(id1); ok {
		t.Fatalf("previous message pending should be removed on replace")
	}
	if _, ok := session.PendingRequests.Load(id2); !ok {
		t.Fatalf("new message should have pending DLR tracking")
	}
	if lastRegistered != id2 {
		t.Fatalf("last registered message id mismatch: got=%d want=%d", lastRegistered, id2)
	}
}

func TestSessionHandleSubmitSMRegistersPendingRequestForFailureOnlyReceipt(t *testing.T) {
	cfg := newTestConfig()
	var registeredID uint64

	session := &Session{
		ID:          "sess-submit-failure-only",
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
		RegisteredDelivery: uint8(FailureOnlyReceipt),
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

func TestSegmentedSubmitSMAnySegmentRequestedDLRRegistersPending(t *testing.T) {
	cfg := newTestConfig()
	var registeredID uint64

	session := &Session{
		ID:          "sess-submit-segmented-any-dlr",
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

	baseSegment := &MessageSegment{
		SegmentGroupID: "group-seg-dlr",
		MessageRefNum:  7,
		SegmentsCount:  2,
		Encoding:       DataCodingDefault,
		RegisteredAt:   time.Now(),
	}

	seg1 := *baseSegment
	seg1.SegmentSeqNum = 1
	seg1.Text = []byte("Hello ")

	seg2 := *baseSegment
	seg2.SegmentSeqNum = 2
	seg2.Text = []byte("World")

	first := &SubmitSmParams{
		SourceAddr:         "77010000000",
		DestAddr:           "77020000000",
		DataCoding:         DataCodingDefault,
		RegisteredDelivery: 0,
		Segment:            &seg1,
		TLVParams:          map[uint16][]byte{},
	}
	messageID1, response1, err1 := session.handleSubmitSM(first, func(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
		return &SmppResponse{Status: StatusOK}
	})
	if err1 != nil {
		t.Fatalf("first segment handleSubmitSM error: %v", err1)
	}
	if response1.Status != StatusOK || messageID1 == 0 {
		t.Fatalf("unexpected first segment result")
	}
	if _, ok := session.PendingRequests.Load(messageID1); ok {
		t.Fatalf("pending request should not be created before full message")
	}

	second := &SubmitSmParams{
		SourceAddr:         "77010000000",
		DestAddr:           "77020000000",
		DataCoding:         DataCodingDefault,
		RegisteredDelivery: uint8(FailureOnlyReceipt),
		Segment:            &seg2,
		TLVParams:          map[uint16][]byte{},
	}
	messageID2, response2, err2 := session.handleSubmitSM(second, func(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
		return &SmppResponse{Status: StatusOK}
	})
	if err2 != nil {
		t.Fatalf("second segment handleSubmitSM error: %v", err2)
	}
	if response2.Status != StatusOK || messageID2 == 0 {
		t.Fatalf("unexpected second segment result")
	}
	if messageID2 != messageID1 {
		t.Fatalf("segment message ids mismatch: first=%d second=%d", messageID1, messageID2)
	}
	if registeredID != messageID2 {
		t.Fatalf("message id registration callback mismatch: got=%d want=%d", registeredID, messageID2)
	}
	if _, ok := session.PendingRequests.Load(messageID2); !ok {
		t.Fatalf("pending request should be created when any segment requested DLR")
	}
}

func TestSessionMeta(t *testing.T) {
	s := &Session{}
	if _, ok := s.GetSessionMeta("k"); ok {
		t.Fatalf("unexpected meta hit on empty session")
	}
	if len(s.SessionMeta()) != 0 {
		t.Fatalf("expected empty snapshot")
	}

	s.SetSessionMeta("tenant", "acme")
	s.SetSessionMeta("route", "primary")

	if v, ok := s.GetSessionMeta("tenant"); !ok || v != "acme" {
		t.Fatalf("GetSessionMeta tenant: ok=%v v=%q", ok, v)
	}

	all := s.SessionMeta()
	if len(all) != 2 || all["route"] != "primary" {
		t.Fatalf("SessionMeta snapshot: %v", all)
	}
	all["route"] = "tampered"
	if v, _ := s.GetSessionMeta("route"); v != "primary" {
		t.Fatalf("SessionMeta should return a copy")
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
