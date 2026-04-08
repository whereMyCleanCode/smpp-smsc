package smsc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdufield"
	"golang.org/x/time/rate"
)

type Session struct {
	MessagesSent      int64
	MessagesReceived  int64
	lastActivityNanos int64
	enquireRetryCount int32

	Bound         bool
	Authenticated bool

	ID            string
	ApplicationID string
	PodID         string
	SystemID      string
	Password      string
	Address       string

	BindingType        BindingType
	RegisteredDelivery RegisteredDeliveryFlags

	Conn   net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer

	rateLimiter *rate.Limiter

	segmentsMgr *SegmentsManager

	errCh           chan error
	stopCh          chan struct{}
	pduQueue        chan pdu.Body
	PendingRequests sync.Map

	handler SMPPHandler

	ctx    context.Context
	cancel context.CancelFunc

	sequenceMutex sync.Mutex
	sessionMutex  sync.RWMutex
	writeMutex    sync.Mutex
	stopOnce      sync.Once

	sequenceNumber uint32

	logger Logger
	cfg    *Config

	registerMessageID   func(uint64)
	unregisterMessageID func(uint64)
}

func (s *Session) start() {
	if s.pduQueue == nil {
		s.pduQueue = make(chan pdu.Body, maxInt(64, s.cfg.WindowSize))
	}
	if s.Reader == nil && s.Conn != nil {
		s.Reader = bufio.NewReaderSize(s.Conn, s.cfg.DecoderBufferSize)
	}
	if s.Writer == nil && s.Conn != nil {
		s.Writer = bufio.NewWriterSize(s.Conn, s.cfg.DecoderBufferSize)
	}
	if s.handler == nil {
		s.handler = &defaultSMPPHandler{}
	}

	go s.outgoingHandler()
	go s.segmentsMgr.startCleanupRoutine(s.ctx.Done())

	for {
		select {
		case <-s.ctx.Done():
			s.Stop()
			return
		case <-s.stopCh:
			s.Stop()
			return
		default:
		}

		if s.Conn == nil || s.Reader == nil {
			s.Stop()
			return
		}

		if err := s.Conn.SetReadDeadline(time.Now().Add(s.cfg.Timeout)); err != nil {
			s.logger.Error().Err(err).Msg("failed to set read deadline")
			s.Stop()
			return
		}

		body, err := pdu.Decode(s.Reader)
		if err != nil {
			if isNetTimeout(err) {
				continue
			}
			if errors.Is(err, io.EOF) {
				s.Stop()
				return
			}
			s.logger.Warn().Err(err).Msg("session read failed")
			s.Stop()
			return
		}

		s.processPDU(body)
		s.updateLastActivity()
	}
}

func (s *Session) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}

		if s.Conn != nil {
			_ = s.Conn.Close()
		}

		if s.segmentsMgr != nil {
			s.segmentsMgr.Stop()
		}

		s.Bound = false
		s.Authenticated = false

		if s.unregisterMessageID != nil {
			s.PendingRequests.Range(func(key, _ interface{}) bool {
				if messageID, ok := key.(uint64); ok {
					s.unregisterMessageID(messageID)
				}
				s.PendingRequests.Delete(key)
				return true
			})
		}
	})
}

func (s *Session) updateLastActivity() {
	atomic.StoreInt64(&s.lastActivityNanos, time.Now().UnixNano())
}

func (s *Session) getLastActivity() time.Time {
	n := atomic.LoadInt64(&s.lastActivityNanos)
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func (s *Session) getEnquireRetryCount() int {
	return int(atomic.LoadInt32(&s.enquireRetryCount))
}

func (s *Session) incrementEnquireRetry() int {
	return int(atomic.AddInt32(&s.enquireRetryCount, 1))
}

func (s *Session) resetEnquireRetry() {
	atomic.StoreInt32(&s.enquireRetryCount, 0)
}

func (s *Session) SetApplicationID(applicationID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()
	s.ApplicationID = applicationID
}

func (s *Session) GetApplicationID() string {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	return s.ApplicationID
}

func (s *Session) getNextSequence() uint32 {
	s.sequenceMutex.Lock()
	defer s.sequenceMutex.Unlock()

	const maxSequence uint32 = 0x7FFFFFFF
	if s.sequenceNumber >= maxSequence {
		s.sequenceNumber = 1
		return s.sequenceNumber
	}
	s.sequenceNumber++
	return s.sequenceNumber
}

func (s *Session) enqueuePDU(body pdu.Body) error {
	select {
	case <-s.ctx.Done():
		return ErrSessionClosed
	default:
	}

	select {
	case s.pduQueue <- body:
		return nil
	case <-time.After(s.cfg.Timeout):
		return fmt.Errorf("session queue timeout")
	}
}

func (s *Session) outgoingHandler() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case body := <-s.pduQueue:
			if body == nil {
				continue
			}
			if err := s.sendPDU(body); err != nil {
				s.logger.Warn().Err(err).Msg("send PDU failed")
			}
		}
	}
}

func (s *Session) sendPDU(body pdu.Body) error {
	select {
	case <-s.ctx.Done():
		return ErrSessionClosed
	default:
	}
	if s.Conn == nil || s.Writer == nil {
		return fmt.Errorf("session connection is not initialized")
	}

	if err := s.Conn.SetWriteDeadline(time.Now().Add(s.cfg.Timeout)); err != nil {
		return err
	}

	var payload bytes.Buffer
	if err := body.SerializeTo(&payload); err != nil {
		return err
	}

	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	if _, err := io.Copy(s.Writer, &payload); err != nil {
		return err
	}
	if err := s.Writer.Flush(); err != nil {
		return err
	}
	atomic.AddInt64(&s.MessagesSent, 1)
	return nil
}

func (s *Session) processPDU(pkt pdu.Body) {
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(s.ctx); err != nil {
			return
		}
	}
	atomic.AddInt64(&s.MessagesReceived, 1)

	id := pkt.Header().ID
	seq := pkt.Header().Seq

	if uint32(id)&0x80000000 != 0 {
		switch id {
		case pdu.EnquireLinkRespID:
			s.PendingRequests.Delete(seq)
			s.resetEnquireRetry()
		case pdu.DeliverSMRespID:
			if s.handler != nil {
				_ = s.handler.HandleDeliverSMResp(s.ctx, seq, uint32(pkt.Header().Status), s)
			}
		default:
			s.PendingRequests.Delete(seq)
		}
		return
	}

	switch id {
	case pdu.BindTransmitterID:
		s.handleBindRequest(pkt, BindingTypeTransmitter, s.handler.HandleBindTransmitter, pdu.BindTransmitterRespID)
	case pdu.BindReceiverID:
		s.handleBindRequest(pkt, BindingTypeReceiver, s.handler.HandleBindReceiver, pdu.BindReceiverRespID)
	case pdu.BindTransceiverID:
		s.handleBindRequest(pkt, BindingTypeTransceiver, s.handler.HandleBindTransceiver, pdu.BindTransceiverRespID)
	case pdu.UnbindID:
		s.handleUnbindPDU(pkt)
	case pdu.SubmitSMID:
		s.handleSubmitSMPDU(pkt)
	case pdu.EnquireLinkID:
		s.handleEnquireLink(pkt)
	default:
		s.sendGenericNACK(seq, StatusInvCmdID)
	}
}

func (s *Session) sendEnquireLinkReq() error {
	if s.getEnquireRetryCount() >= s.cfg.MaxEnquireLinkRetryCount {
		return fmt.Errorf("max enquire_link retries exceeded")
	}

	req := pdu.NewEnquireLink()
	seq := s.getNextSequence()
	req.Header().Seq = seq

	s.PendingRequests.Store(seq, PendingRequest{CreatedAt: time.Now()})
	if err := s.enqueuePDU(req); err != nil {
		return err
	}
	s.incrementEnquireRetry()
	return nil
}

func (s *Session) sendGenericNACK(sequenceNumber uint32, status uint32) {
	resp := pdu.NewGenericNACK()
	resp.Header().Seq = sequenceNumber
	resp.Header().Status = pdu.Status(status)
	_ = s.enqueuePDU(resp)
}

func (s *Session) handleEnquireLink(pkt pdu.Body) {
	var status uint32 = StatusOK
	if s.handler != nil {
		handlerStatus, err := s.handler.HandleEnquireLink(s.ctx, s)
		if err != nil {
			status = handlerStatus
		}
	}

	resp := pdu.NewEnquireLinkRespSeq(pkt.Header().Seq)
	resp.Header().Status = pdu.Status(status)
	_ = s.enqueuePDU(resp)
}

func (s *Session) handleBindRequest(
	pkt pdu.Body,
	bindType BindingType,
	handler func(context.Context, map[string]string, *Session) (uint32, error),
	respID pdu.ID,
) {
	params, err := parseBindParams(pkt)
	if err != nil {
		s.sendGenericNACK(pkt.Header().Seq, StatusInvMsgLen)
		return
	}

	if s.Bound {
		s.sendBindResponse(respID, pkt.Header().Seq, StatusAlyBnd, s.cfg.SystemID)
		return
	}

	status, handlerErr := handler(s.ctx, params, s)
	if handlerErr != nil {
		if status == 0 {
			status = StatusBindFail
		}
		s.sendBindResponse(respID, pkt.Header().Seq, status, s.cfg.SystemID)
		return
	}

	s.SystemID = params["system_id"]
	s.Password = params["password"]
	s.BindingType = bindType
	if status == StatusOK {
		s.Bound = true
		s.Authenticated = true
	}

	systemID := s.cfg.SystemID
	if systemID == "" {
		systemID = params["system_id"]
	}
	s.sendBindResponse(respID, pkt.Header().Seq, status, systemID)
}

func parseBindParams(pkt pdu.Body) (map[string]string, error) {
	fields := pkt.Fields()
	systemIDField := fields[pdufield.SystemID]
	passwordField := fields[pdufield.Password]
	if systemIDField == nil || passwordField == nil {
		return nil, fmt.Errorf("bind missing system_id/password")
	}
	params := map[string]string{
		"system_id": systemIDField.String(),
		"password":  passwordField.String(),
	}
	if f := fields[pdufield.SystemType]; f != nil {
		params["system_type"] = f.String()
	}
	return params, nil
}

func (s *Session) sendBindResponse(commandID pdu.ID, sequenceNumber uint32, status uint32, systemID string) {
	var resp pdu.Body
	switch commandID {
	case pdu.BindTransmitterRespID:
		resp = pdu.NewBindTransmitterResp()
	case pdu.BindReceiverRespID:
		resp = pdu.NewBindReceiverResp()
	case pdu.BindTransceiverRespID:
		resp = pdu.NewBindTransceiverResp()
	default:
		resp = pdu.NewGenericNACK()
	}

	resp.Header().Seq = sequenceNumber
	resp.Header().Status = pdu.Status(status)
	if commandID != pdu.GenericNACKID {
		_ = resp.Fields().Set(pdufield.SystemID, systemID)
	}
	_ = s.enqueuePDU(resp)
}

func (s *Session) handleUnbindPDU(pkt pdu.Body) {
	status, err := s.handler.HandleUnbind(s.ctx, s)
	if err != nil {
		s.sendGenericNACK(pkt.Header().Seq, status)
		return
	}
	s.sendUnbindResponse(pkt.Header().Seq, status)
	s.Stop()
}

func (s *Session) sendUnbindResponse(sequenceNumber uint32, status uint32) {
	resp := pdu.NewUnbindResp()
	resp.Header().Seq = sequenceNumber
	resp.Header().Status = pdu.Status(status)
	_ = s.enqueuePDU(resp)
}

func (s *Session) handleSubmitSMPDU(pkt pdu.Body) {
	params, status, err := parseSubmitSM(pkt)
	if err != nil {
		s.sendSubmitSMResponse(pkt.Header().Seq, &SmppResponse{Status: status}, "")
		return
	}

	s.RegisteredDelivery = RegisteredDeliveryFlags(params.RegisteredDelivery)

	messageID, response, handleErr := s.handleSubmitSM(params, s.handler.HandleSubmitSM)
	if handleErr != nil {
		s.sendSubmitSMResponse(pkt.Header().Seq, response, "")
		return
	}
	s.sendSubmitSMResponse(pkt.Header().Seq, response, strconv.FormatUint(messageID, 10))
}

func parseSubmitSM(pkt pdu.Body) (*SubmitSmParams, uint32, error) {
	fields := pkt.Fields()
	params := &SubmitSmParams{
		ServiceType:          fields[pdufield.ServiceType].String(),
		SourceAddr:           fields[pdufield.SourceAddr].String(),
		DestAddr:             fields[pdufield.DestinationAddr].String(),
		ScheduleDeliveryTime: fields[pdufield.ScheduleDeliveryTime].String(),
		ValidityPeriod:       fields[pdufield.ValidityPeriod].String(),
		TLVParams:            make(map[uint16][]byte),
		SeqNum:               pkt.Header().Seq,
	}

	if params.SourceAddr == "" {
		return nil, StatusInvSrcAdr, fmt.Errorf("source_addr is empty")
	}
	if params.DestAddr == "" {
		return nil, StatusInvDstAdr, fmt.Errorf("destination_addr is empty")
	}

	if v, ok := fields[pdufield.SourceAddrTON].(*pdufield.Fixed); ok {
		params.SourceAddrTON = v.Data
	}
	if v, ok := fields[pdufield.SourceAddrNPI].(*pdufield.Fixed); ok {
		params.SourceAddrNPI = v.Data
	}
	if v, ok := fields[pdufield.DestAddrTON].(*pdufield.Fixed); ok {
		params.DestAddrTON = v.Data
	}
	if v, ok := fields[pdufield.DestAddrNPI].(*pdufield.Fixed); ok {
		params.DestAddrNPI = v.Data
	}
	if v, ok := fields[pdufield.ESMClass].(*pdufield.Fixed); ok {
		params.ESMClass = v.Data
	}
	if v, ok := fields[pdufield.ProtocolID].(*pdufield.Fixed); ok {
		params.ProtocolID = v.Data
	}
	if v, ok := fields[pdufield.PriorityFlag].(*pdufield.Fixed); ok {
		params.PriorityFlag = v.Data
	}
	if v, ok := fields[pdufield.ReplaceIfPresentFlag].(*pdufield.Fixed); ok {
		params.ReplaceIfPresentFlag = v.Data
	}
	if v, ok := fields[pdufield.SMDefaultMsgID].(*pdufield.Fixed); ok {
		params.SMDefaultMsgID = v.Data
	}
	if v, ok := fields[pdufield.DataCoding].(*pdufield.Fixed); ok {
		params.DataCoding = v.Data
	}
	if params.DataCoding == 0 {
		params.DataCoding = DataCodingDefault
	}

	if params.DataCoding != DataCodingDefault &&
		params.DataCoding != DataCodingUCS2 &&
		params.DataCoding != DataCodingLatin1 &&
		params.DataCoding != DataCodingCyrillic {
		return nil, StatusInvDataCoding, fmt.Errorf("unsupported data_coding=%d", params.DataCoding)
	}

	if v, ok := fields[pdufield.RegisteredDelivery].(*pdufield.Fixed); ok {
		params.RegisteredDelivery = v.Data
	}

	if sm, ok := fields[pdufield.ShortMessage].(*pdufield.SM); ok {
		params.ShortMessage = append([]byte(nil), sm.Data...)
	}

	for tag, field := range pkt.TLVFields() {
		params.TLVParams[uint16(tag)] = append([]byte(nil), field.Bytes()...)
	}

	if payload, ok := params.TLVParams[TagMessagePayload]; ok && len(payload) > 0 {
		params.ShortMessage = append([]byte(nil), payload...)
		params.WithPayload = true
	}

	if udhList, ok := fields[pdufield.GSMUserData].(*pdufield.UDHList); ok && len(udhList.Data) > 0 {
		segment := &MessageSegment{
			RegisteredAt: time.Now(),
			Text:         append([]byte(nil), params.ShortMessage...),
		}
		for _, udh := range udhList.Data {
			iei := udh.IEI.Data
			data := udh.IEData.Data
			switch iei {
			case 0x00:
				if len(data) == 3 {
					segment.MessageRefNum = data[0]
					segment.SegmentsCount = data[1]
					segment.SegmentSeqNum = data[2]
				}
			case 0x08:
				if len(data) == 4 {
					segment.MessageRefNum = data[1]
					segment.SegmentsCount = data[2]
					segment.SegmentSeqNum = data[3]
				}
			}
		}
		if segment.SegmentsCount > 1 && segment.SegmentSeqNum > 0 {
			params.Segment = segment
		}
	}

	if refNum, ok := params.GetTLVUint16(TagSarMsgRefNum); ok {
		if params.Segment == nil {
			params.Segment = &MessageSegment{
				RegisteredAt: time.Now(),
			}
		}
		params.Segment.MessageRefNum = uint8(refNum)
		if total, ok := params.GetTLVUint16(TagSarTotalSegments); ok {
			params.Segment.SegmentsCount = uint8(total)
		}
		if seqNum, ok := params.GetTLVUint16(TagSarSegmentSeqnum); ok {
			params.Segment.SegmentSeqNum = uint8(seqNum)
		}
		params.Segment.Text = append([]byte(nil), params.ShortMessage...)
	}

	if params.Segment == nil {
		text, err := DecodeMessage(params.ShortMessage, params.DataCoding)
		if err != nil {
			return nil, StatusInvDataCoding, err
		}
		params.Text = text
	} else {
		fillSegmentGroupID(params)
	}

	return params, StatusOK, nil
}

func fillSegmentGroupID(params *SubmitSmParams) {
	prefix := params.SourceAddr + "|" + params.DestAddr
	if params.Segment != nil && params.Segment.SegmentsCount > 1 {
		prefix += "|" + strconv.Itoa(int(params.Segment.MessageRefNum))
	}
	sum := md5.Sum([]byte(prefix))
	params.Segment.SegmentGroupID = hex.EncodeToString(sum[:])
}

func (s *Session) handleSubmitSM(
	params *SubmitSmParams,
	handle func(context.Context, *SubmitSmParams, *Session) *SmppResponse,
) (uint64, *SmppResponse, error) {
	if handle == nil {
		return 0, &SmppResponse{Status: StatusSysErr}, fmt.Errorf("submit handler is nil")
	}

	if params.Segment != nil && params.Segment.SegmentsCount > 1 {
		params.Segment.DeliveryReceiptRequested = RegisteredDeliveryFlags(params.RegisteredDelivery).RequiresDeliveryReceipt()
		messageID, status, fullText, isComplete, deliveryReceiptRequested, err := s.segmentsMgr.AddSegment(params.Segment)
		params.MessageID = messageID
		if err != nil {
			return messageID, &SmppResponse{Status: status}, err
		}
		if !isComplete {
			return messageID, &SmppResponse{Status: StatusOK}, nil
		}
		params.Text = fullText
		response := handle(s.ctx, params, s)
		if response == nil {
			response = &SmppResponse{Status: StatusSysErr}
		}
		if deliveryReceiptRequested {
			segmentsCount := calcSegments(params.DataCoding, params.Text)
			s.PendingRequests.Store(messageID, PendingRequest{SegmentsCount: segmentsCount, CreatedAt: time.Now()})
			if s.registerMessageID != nil {
				s.registerMessageID(messageID)
			}
		}
		if response.Status != StatusOK {
			return messageID, response, fmt.Errorf("submit_sm failed with status=%d", response.Status)
		}
		return messageID, response, nil
	}

	messageID, err := s.segmentsMgr.generateMessageID()
	if err != nil {
		return 0, &SmppResponse{Status: StatusSysErr}, err
	}
	params.MessageID = messageID
	if params.Text == "" {
		text, derr := DecodeMessage(params.ShortMessage, params.DataCoding)
		if derr != nil {
			return messageID, &SmppResponse{Status: StatusInvDataCoding}, derr
		}
		params.Text = text
	}

	response := handle(s.ctx, params, s)
	if response == nil {
		response = &SmppResponse{Status: StatusSysErr}
	}
	if RegisteredDeliveryFlags(params.RegisteredDelivery).RequiresDeliveryReceipt() {
		segmentsCount := calcSegments(params.DataCoding, params.Text)
		s.PendingRequests.Store(messageID, PendingRequest{SegmentsCount: segmentsCount, CreatedAt: time.Now()})
		if s.registerMessageID != nil {
			s.registerMessageID(messageID)
		}
	}
	if response.Status != StatusOK {
		return messageID, response, fmt.Errorf("submit_sm failed with status=%d", response.Status)
	}
	return messageID, response, nil
}

func (s *Session) sendSubmitSMResponse(sequenceNumber uint32, status *SmppResponse, messageID string) {
	resp := pdu.NewSubmitSMResp()
	resp.Header().Seq = sequenceNumber
	resp.Header().Status = pdu.Status(status.Status)
	_ = resp.Fields().Set(pdufield.MessageID, messageID)
	_ = s.enqueuePDU(resp)
}

func (s *Session) RemovePendingRequestByMessageID(messageID uint64) {
	if _, ok := s.PendingRequests.Load(messageID); ok {
		s.PendingRequests.Delete(messageID)
		if s.unregisterMessageID != nil {
			s.unregisterMessageID(messageID)
		}
	}
}

func calcSegments(dataCoding byte, text string) uint8 {
	var maxSingle, maxConcat, length int
	switch dataCoding {
	case 0x00:
		if septets := gsm7SeptetCount(text); septets >= 0 {
			maxSingle, maxConcat = 160, 153
			length = septets
			break
		}
		fallthrough
	case 0x08:
		maxSingle, maxConcat = 70, 67
		length = len(utf16.Encode([]rune(text)))
	default:
		maxSingle, maxConcat = 140, 134
		length = len(text)
	}

	if length <= maxSingle {
		return 1
	}

	n := length / maxConcat
	if length%maxConcat != 0 {
		n++
	}
	return uint8(n)
}

func gsm7SeptetCount(text string) int {
	count := 0
	for _, r := range text {
		switch {
		case GSM7BitCharset[r]:
			count++
		case GSM7BitCharsetExt[r]:
			count += 2
		default:
			return -1
		}
	}
	return count
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
