package smsc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdufield"
	"github.com/whereMyCleanCode/go-smpp/v2/smpp/pdu/pdutlv"
	"golang.org/x/time/rate"
)

type Server struct {
	mu sync.Mutex

	idGenerator IDGenerator
	cfg         *Config
	lgr         Logger

	listener net.Listener

	sessionsManager *SessController

	ctx          context.Context
	cancel       context.CancelFunc
	shutdownWait sync.WaitGroup
	handler      SMPPHandler
}

func NewServer(cfg *Config, lgr Logger, idGenerator IDGenerator) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if lgr == nil {
		lgr = DefaultLogger()
	}
	if idGenerator == nil {
		g, err := NewSnowflakeGenerator(1)
		if err != nil {
			return nil, err
		}
		idGenerator = g
	}

	ctx, cancel := context.WithCancel(context.Background())
	sessionsManager, err := NewSessionsManager(ctx, lgr, cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create sessions manager: %w", err)
	}

	srv := &Server{
		idGenerator:     idGenerator,
		cfg:             cfg,
		lgr:             lgr.WithStr("component", "smpp_server"),
		ctx:             ctx,
		cancel:          cancel,
		sessionsManager: sessionsManager,
		handler:         &defaultSMPPHandler{},
	}
	return srv, nil
}

func (s *Server) SetHandler(handler SMPPHandler) {
	if handler == nil {
		return
	}
	s.handler = handler
}

func (s *Server) Start() chan error {
	errCh := make(chan error, 1)

	s.lgr.Info().
		Str("address", s.cfg.Address).
		Str("pod_id", s.cfg.PodID).
		Str("system_id", s.cfg.SystemID).
		Str("log_level", s.cfg.LogLevel).
		Msg("starting SMPP SMSC server")

	if s.cfg.StartupVerbose {
		s.lgr.Info().
			Dur("timeout", s.cfg.Timeout).
			Dur("inactivity_timeout", s.cfg.InactivityTimeout).
			Dur("segments_ttl", s.cfg.SegsBucketTtl).
			Int("window_size", s.cfg.WindowSize).
			Int("decoder_buffer_size", s.cfg.DecoderBufferSize).
			Int("max_rps_limit", s.cfg.DefaultMaxRPSLimit).
			Int("burst_rps_limit", s.cfg.DefaultBurstRPSLimit).
			Msg("runtime limits")

		s.lgr.Info().
			Bool("tcp_nodelay", s.cfg.TCPNoDelay).
			Bool("tcp_keepalive", s.cfg.TCPKeepAlive).
			Dur("tcp_keepalive_period", s.cfg.TCPKeepAlivePeriod).
			Int("tcp_read_buffer", s.cfg.TCPReadBufferSize).
			Int("tcp_write_buffer", s.cfg.TCPWriteBufferSize).
			Int("tcp_linger", s.cfg.TCPLinger).
			Msg("tcp settings")
	}

	listener, err := net.Listen("tcp", s.cfg.Address)
	if err != nil {
		s.lgr.Error().Err(err).Str("address", s.cfg.Address).Msg("failed to start listener")
		errCh <- fmt.Errorf("listen: %w", err)
		return errCh
	}

	s.listener = listener
	s.lgr.Info().Str("listen_addr", listener.Addr().String()).Msg("listener started")

	s.sessionsManager.Start()
	s.lgr.Info().Msg("sessions manager started")

	s.shutdownWait.Add(1)
	go func() {
		defer s.shutdownWait.Done()
		s.acceptLoop(errCh)
	}()

	return errCh
}

func (s *Server) Shutdown() {
	s.lgr.Info().Msg("shutdown requested")
	s.cancel()
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.lgr.Warn().Err(err).Msg("listener close failed")
		} else {
			s.lgr.Info().Msg("listener closed")
		}
	}
	s.lgr.Info().Msg("stopping sessions manager")
	s.sessionsManager.Shutdown()
	s.lgr.Info().Msg("waiting for background loops")
	s.shutdownWait.Wait()
	s.lgr.Info().Msg("server shutdown completed")
}

func (s *Server) acceptLoop(errCh chan error) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				s.lgr.Info().Msg("accept loop stopped by context cancellation")
				return
			}
			s.lgr.Warn().Err(err).Msg("accept failed")
			select {
			case errCh <- err:
			default:
			}
			continue
		}

		s.applyTCPSettings(conn)
		s.lgr.Info().
			Str("client_addr", conn.RemoteAddr().String()).
			Msg("incoming connection accepted")
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	if err := conn.SetReadDeadline(time.Now().Add(s.cfg.Timeout)); err != nil {
		s.lgr.Warn().
			Err(err).
			Str("client_addr", conn.RemoteAddr().String()).
			Msg("failed to set initial read deadline")
		_ = conn.Close()
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(s.cfg.Timeout)); err != nil {
		s.lgr.Warn().
			Err(err).
			Str("client_addr", conn.RemoteAddr().String()).
			Msg("failed to set initial write deadline")
		_ = conn.Close()
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	now := time.Now()
	sessionID := s.generateSessionID()

	session := &Session{
		ID:                sessionID,
		PodID:             s.cfg.PodID,
		Address:           conn.RemoteAddr().String(),
		Conn:              conn,
		Reader:            bufio.NewReaderSize(conn, s.cfg.DecoderBufferSize),
		Writer:            bufio.NewWriterSize(conn, s.cfg.DecoderBufferSize),
		Bound:             false,
		BindingType:       BindingTypeNone,
		handler:           s.handler,
		cfg:               s.cfg,
		logger:            s.lgr.With().Str("session_id", sessionID).Str("client_addr", conn.RemoteAddr().String()).Logger(),
		pduQueue:          make(chan pdu.Body, maxInt(64, s.cfg.WindowSize)),
		errCh:             make(chan error, 1),
		stopCh:            make(chan struct{}),
		ctx:               ctx,
		cancel:            cancel,
		lastActivityNanos: now.UnixNano(),
		segmentsMgr:       NewSegmentsManager(s.lgr, s.cfg.SegsBucketTtl, s.idGenerator, s.cfg.MaxSubmitSMSegments),
		rateLimiter:       rate.NewLimiter(rate.Limit(maxInt(1, s.cfg.DefaultMaxRPSLimit)), maxInt(1, s.cfg.DefaultBurstRPSLimit)),
	}

	session.registerMessageID = func(messageID uint64) {
		s.sessionsManager.RegisterMessageID(messageID, session)
	}
	session.unregisterMessageID = func(messageID uint64) {
		s.sessionsManager.UnregisterMessageID(messageID)
	}

	s.sessionsManager.InitializeSession(session)
	s.lgr.Info().
		Str("session_id", sessionID).
		Str("client_addr", conn.RemoteAddr().String()).
		Str("pod_id", s.cfg.PodID).
		Msg("session initialized")
}

func (s *Server) generateSessionID() string {
	id, err := s.idGenerator.GenerateID()
	if err != nil {
		return fmt.Sprintf("sess_%s_%d", s.cfg.PodID, time.Now().UnixNano())
	}
	return fmt.Sprintf("sess_%s_%d", s.cfg.PodID, id)
}

func (s *Server) GetSession(sessionID string) (*Session, bool) {
	return s.sessionsManager.GetSessionByID(sessionID)
}

func (s *Server) GetSessionByMessageID(applicationID string, messageID uint64) (*Session, error) {
	session, ok := s.sessionsManager.GetSessionByMessageID(messageID)
	if !ok {
		return nil, fmt.Errorf("session not found for message_id=%d", messageID)
	}
	if applicationID != "" && session.ApplicationID != applicationID {
		return nil, fmt.Errorf("session application_id mismatch")
	}
	return session, nil
}

func (s *Server) GetSessionsByApplicationID(applicationID string) ([]*Session, error) {
	return s.sessionsManager.GetSessionsByApplicationID(applicationID)
}

type DeliverSMParams struct {
	SourceAddr         string
	DestAddr           string
	Message            []byte
	DataCoding         uint8
	ESMClass           uint8
	ServiceType        string
	SourceAddrTON      uint8
	SourceAddrNPI      uint8
	DestAddrTON        uint8
	DestAddrNPI        uint8
	ProtocolID         uint8
	PriorityFlag       uint8
	RegisteredDelivery uint8
	ReplaceIfPresent   uint8
	ValidityPeriod     string
	ScheduleTime       string
	SMDefaultMsgID     uint8
	TLVOptions         map[uint16][]byte
}

func NewDeliverSMParams(sourceAddr, destAddr string, message []byte) *DeliverSMParams {
	return &DeliverSMParams{
		SourceAddr:         sourceAddr,
		DestAddr:           destAddr,
		Message:            message,
		DataCoding:         DataCodingDefault,
		ESMClass:           0,
		ServiceType:        "",
		SourceAddrTON:      1,
		SourceAddrNPI:      1,
		DestAddrTON:        1,
		DestAddrNPI:        1,
		ProtocolID:         0,
		PriorityFlag:       0,
		RegisteredDelivery: 0,
		ReplaceIfPresent:   0,
		ValidityPeriod:     "",
		ScheduleTime:       "",
		SMDefaultMsgID:     0,
		TLVOptions:         map[uint16][]byte{},
	}
}

func (p *DeliverSMParams) SetUDHIndicator() {
	modeAndType := p.ESMClass & 0x3F
	p.ESMClass = modeAndType | 0x40
}

func (p *DeliverSMParams) SetSourceAddrAlphaName() {
	p.SourceAddrTON = 5
	p.SourceAddrNPI = 0
}

func (p *DeliverSMParams) AddTLV(tag uint16, value []byte) {
	if p.TLVOptions == nil {
		p.TLVOptions = map[uint16][]byte{}
	}
	p.TLVOptions[tag] = append([]byte(nil), value...)
}

func (s *Server) SendDeliverSM(ctx context.Context, sessionID, sourceAddr, destAddr string, message []byte, dataCoding uint8, esmClass uint8) (uint32, error) {
	params := NewDeliverSMParams(sourceAddr, destAddr, message)
	params.DataCoding = dataCoding
	params.ESMClass = esmClass
	return s.SendDeliverSMWithParams(ctx, sessionID, params)
}

func (s *Server) SendDeliverSMWithParams(ctx context.Context, sessionID string, params *DeliverSMParams) (uint32, error) {
	session, ok := s.sessionsManager.GetSessionByID(sessionID)
	if !ok {
		return 0, fmt.Errorf("session not found: %s", sessionID)
	}
	if !session.Bound {
		return 0, fmt.Errorf("session not bound: %s", sessionID)
	}
	if !session.BindingType.IsReceiver() {
		return 0, fmt.Errorf("invalid binding type: %s", session.BindingType)
	}

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	msg := s.createDeliverSMPDUWithParams(params)
	seq := session.getNextSequence()
	msg.Header().Seq = seq
	msg.Header().Status = pdu.Status(StatusOK)

	session.PendingRequests.Store(seq, PendingRequest{CreatedAt: time.Now()})

	if err := session.enqueuePDU(msg); err != nil {
		return 0, err
	}
	return seq, nil
}

// SendDeliveryReport sends a GSM short message delivery receipt as deliver_sm (ESM class delivery receipt).
// sourceAddr is typically the MT destination (recipient); destAddr the original submit source (sender).
// messageIDStr must match the message_id from submit_sm_resp (id: field in the receipt body).
// internalMessageID is the SMSC internal id stored in PendingRequests at submit time; it is removed after a send or policy skip.
func (s *Server) SendDeliveryReport(
	ctx context.Context,
	sessionID, sourceAddr, destAddr, messageIDStr string,
	internalMessageID uint64,
	success bool,
	receiptText string,
) (DeliveryReportResult, error) {
	session, ok := s.sessionsManager.GetSessionByID(sessionID)
	if !ok {
		return DeliveryReportSkippedSessionClosed, fmt.Errorf("session not found: %s", sessionID)
	}
	if !session.Bound {
		return DeliveryReportSkippedSessionClosed, fmt.Errorf("session not bound: %s", sessionID)
	}
	if !session.BindingType.IsReceiver() {
		return DeliveryReportSkippedSessionClosed, fmt.Errorf("invalid binding type for deliver: %s", session.BindingType)
	}

	v, ok := session.PendingRequests.Load(internalMessageID)
	if !ok {
		return DeliveryReportSkippedNoReceipt, fmt.Errorf("pending request not found for message_id=%d", internalMessageID)
	}
	pr := v.(PendingRequest)
	flags := RegisteredDeliveryFlags(pr.RegisteredDelivery)
	if !flags.RequiresDeliveryReceipt() {
		session.RemovePendingRequestByMessageID(internalMessageID)
		return DeliveryReportSkippedNoReceipt, nil
	}
	if !flags.ShouldSendDeliveryReceipt(success) {
		session.RemovePendingRequestByMessageID(internalMessageID)
		switch flags.GetReceiptType() {
		case SuccessOnlyReceipt:
			return DeliveryReportSkippedSuccessOnly, nil
		case FailureOnlyReceipt:
			return DeliveryReportSkippedFailureOnly, nil
		default:
			return DeliveryReportSkippedNoReceipt, nil
		}
	}

	now := time.Now()
	var fields DeliveryReceiptFields
	if receiptText != "" {
		fields = BuildDeliveryReceiptFromPendingWithText(messageIDStr, pr, success, now, receiptText)
	} else {
		fields = BuildDeliveryReceiptFromPending(messageIDStr, pr, success, now)
	}
	body := []byte(FormatDeliveryReceiptString(fields))
	params := NewDeliverSMParams(sourceAddr, destAddr, body)
	params.DataCoding = DataCodingDefault
	params.ESMClass = ESMClassDeliveryReceipt

	_, err := s.SendDeliverSMWithParams(ctx, sessionID, params)
	if err != nil {
		if errors.Is(err, ErrSessionClosed) {
			return DeliveryReportSkippedSessionClosed, err
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, err
		}
		if strings.Contains(err.Error(), "timeout") {
			return DeliveryReportSkippedQueueFull, err
		}
		return 0, err
	}

	session.RemovePendingRequestByMessageID(internalMessageID)
	return DeliveryReportSent, nil
}

func (s *Server) createDeliverSMPDUWithParams(params *DeliverSMParams) pdu.Body {
	body := pdu.NewDeliverSM()
	fields := body.Fields()
	_ = fields.Set(pdufield.ServiceType, params.ServiceType)
	_ = fields.Set(pdufield.SourceAddrTON, params.SourceAddrTON)
	_ = fields.Set(pdufield.SourceAddrNPI, params.SourceAddrNPI)
	_ = fields.Set(pdufield.SourceAddr, params.SourceAddr)
	_ = fields.Set(pdufield.DestAddrTON, params.DestAddrTON)
	_ = fields.Set(pdufield.DestAddrNPI, params.DestAddrNPI)
	_ = fields.Set(pdufield.DestinationAddr, params.DestAddr)
	_ = fields.Set(pdufield.ESMClass, params.ESMClass)
	_ = fields.Set(pdufield.ProtocolID, params.ProtocolID)
	_ = fields.Set(pdufield.PriorityFlag, params.PriorityFlag)
	_ = fields.Set(pdufield.ScheduleDeliveryTime, params.ScheduleTime)
	_ = fields.Set(pdufield.ValidityPeriod, params.ValidityPeriod)
	_ = fields.Set(pdufield.RegisteredDelivery, params.RegisteredDelivery)
	_ = fields.Set(pdufield.ReplaceIfPresentFlag, params.ReplaceIfPresent)
	_ = fields.Set(pdufield.DataCoding, params.DataCoding)
	_ = fields.Set(pdufield.SMDefaultMsgID, params.SMDefaultMsgID)
	_ = fields.Set(pdufield.ShortMessage, params.Message)

	for tag, value := range params.TLVOptions {
		_ = body.TLVFields().Set(pdutlv.Tag(tag), value)
	}
	return body
}

func (s *Server) SendSegmentedDeliverSM(
	ctx context.Context,
	sessionID, sourceAddr, destAddr, text string,
	determineEncoding func(string) uint8,
	encodeText func(string, uint8) ([]byte, error),
) (int, error) {
	if text == "" {
		return 0, fmt.Errorf("empty text")
	}
	if sessionID == "" {
		return 0, fmt.Errorf("session ID is required")
	}
	if sourceAddr == "" || destAddr == "" {
		return 0, fmt.Errorf("source and destination are required")
	}

	dataCoding := determineEncoding(text)
	if len(text) > s.cfg.DefaultMaxSegsCount {
		runes := []rune(text)
		if len(runes) > s.cfg.DefaultMaxSegsCount {
			text = string(runes[:s.cfg.DefaultMaxSegsCount])
		}
	}

	segments := splitTextIntoSegments(text, dataCoding)
	if len(segments) == 1 {
		payload, err := encodeText(text, dataCoding)
		if err != nil {
			return 0, err
		}
		params := NewDeliverSMParams(sourceAddr, destAddr, payload)
		params.DataCoding = dataCoding
		_, err = s.SendDeliverSMWithParams(ctx, sessionID, params)
		if err != nil {
			return 0, err
		}
		return 1, nil
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	msgRef := uint16(r.Intn(65536))
	msgRefBytes := []byte{byte(msgRef >> 8), byte(msgRef)}

	sent := 0
	for i, segmentText := range segments {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		default:
		}

		udh := []byte{
			0x06, 0x08, 0x04,
			msgRefBytes[0], msgRefBytes[1],
			byte(len(segments)),
			byte(i + 1),
		}
		segmentBytes, err := encodeText(segmentText, dataCoding)
		if err != nil {
			return sent, err
		}
		if len(segmentBytes)+len(udh) > 255 {
			segmentBytes = segmentBytes[:255-len(udh)]
		}
		msg := append(append([]byte{}, udh...), segmentBytes...)

		params := NewDeliverSMParams(sourceAddr, destAddr, msg)
		params.DataCoding = dataCoding
		params.SetUDHIndicator()

		if _, err := s.SendDeliverSMWithParams(ctx, sessionID, params); err != nil {
			return sent, err
		}
		sent++
		time.Sleep(10 * time.Millisecond)
	}
	return sent, nil
}

func (s *Server) SendDeliverSMText(ctx context.Context, sessionID, sourceAddr, destAddr, text string) (int, error) {
	determineEncoding := func(text string) uint8 {
		if IsGSM7Bit(text) {
			return DataCodingDefault
		}
		if isLatin1Encodable(text) {
			return DataCodingLatin1
		}
		return DataCodingUCS2
	}
	encode := func(text string, dc uint8) ([]byte, error) {
		return EncodeMessage(text, dc)
	}

	return s.SendSegmentedDeliverSM(ctx, sessionID, sourceAddr, destAddr, text, determineEncoding, encode)
}

// SendDeliverSMTextWithDataCoding sends MT text using the given data_coding (client / application choice),
// encoding the UTF-8 string into GSM7 / UCS2 / Latin1 / Cyrillic bytes matching SMPP semantics.
func (s *Server) SendDeliverSMTextWithDataCoding(ctx context.Context, sessionID, sourceAddr, destAddr, text string, dataCoding uint8) (int, error) {
	determineEncoding := func(_ string) uint8 {
		return dataCoding
	}
	encode := func(t string, dc uint8) ([]byte, error) {
		return EncodeMessage(t, dc)
	}
	return s.SendSegmentedDeliverSM(ctx, sessionID, sourceAddr, destAddr, text, determineEncoding, encode)
}

func splitTextIntoSegments(text string, dataCoding uint8) []string {
	const (
		maxGSM7Length      = 160
		maxGSM7MultiLength = 153
		maxUCS2Length      = 70
		maxUCS2MultiLength = 67
	)

	maxLen := maxUCS2Length
	maxMultiLen := maxUCS2MultiLength
	switch dataCoding {
	case DataCodingDefault:
		maxLen = maxGSM7Length
		maxMultiLen = maxGSM7MultiLength
	case DataCodingLatin1, DataCodingCyrillic:
		maxLen = 140
		maxMultiLen = 134
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	segments := make([]string, 0)
	for start := 0; start < len(runes); start += maxMultiLen {
		end := start + maxMultiLen
		if end > len(runes) {
			end = len(runes)
		}
		segments = append(segments, string(runes[start:end]))
	}
	return segments
}

func isLatin1Encodable(text string) bool {
	for _, r := range text {
		if r > 255 {
			return false
		}
	}
	return true
}

func (s *Server) applyTCPSettings(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}

	if s.cfg.TCPNoDelay {
		_ = tcpConn.SetNoDelay(true)
	}
	if s.cfg.TCPKeepAlive {
		_ = tcpConn.SetKeepAlive(true)
		period := s.cfg.TCPKeepAlivePeriod
		if period > 0 {
			_ = tcpConn.SetKeepAlivePeriod(period)
		}
	}
	if s.cfg.TCPReadBufferSize > 0 {
		_ = tcpConn.SetReadBuffer(s.cfg.TCPReadBufferSize)
	}
	if s.cfg.TCPWriteBufferSize > 0 {
		_ = tcpConn.SetWriteBuffer(s.cfg.TCPWriteBufferSize)
	}
	if s.cfg.TCPLinger >= 0 {
		_ = tcpConn.SetLinger(s.cfg.TCPLinger)
	}
}

type defaultSMPPHandler struct{}

func (h *defaultSMPPHandler) HandleBindTransceiver(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = BindingTypeTransceiver
	session.Bound = true
	session.logger.Info().
		Str("handler", "default_smpp").
		Str("event", "bind_transceiver").
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *defaultSMPPHandler) HandleBindReceiver(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = BindingTypeReceiver
	session.Bound = true
	session.logger.Info().
		Str("handler", "default_smpp").
		Str("event", "bind_receiver").
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *defaultSMPPHandler) HandleBindTransmitter(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = BindingTypeTransmitter
	session.Bound = true
	session.logger.Info().
		Str("handler", "default_smpp").
		Str("event", "bind_transmitter").
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *defaultSMPPHandler) HandleSubmitSM(_ context.Context, params *SubmitSmParams, session *Session) *SmppResponse {
	if !session.BindingType.IsTransmitter() {
		session.logger.Info().
			Str("handler", "default_smpp").
			Str("event", "submit_sm").
			Str("reason", "invalid_binding").
			Msg("mock handler")
		return &SmppResponse{Status: StatusInvBnd}
	}
	if params.SourceAddr == "" || params.DestAddr == "" {
		session.logger.Info().
			Str("handler", "default_smpp").
			Str("event", "submit_sm").
			Str("reason", "missing_addresses").
			Msg("mock handler")
		return &SmppResponse{Status: StatusInvSrcAdr}
	}
	session.logger.Info().
		Str("handler", "default_smpp").
		Str("event", "submit_sm").
		Str("source", params.SourceAddr).
		Str("destination", params.DestAddr).
		Msg("mock handler")
	return &SmppResponse{Status: StatusOK}
}

func (h *defaultSMPPHandler) HandleUnbind(_ context.Context, session *Session) (uint32, error) {
	session.Bound = false
	session.logger.Info().
		Str("handler", "default_smpp").
		Str("event", "unbind").
		Msg("mock handler")
	return StatusOK, nil
}

func (h *defaultSMPPHandler) HandleEnquireLink(_ context.Context, session *Session) (uint32, error) {
	session.logger.Debug().
		Str("handler", "default_smpp").
		Str("event", "enquire_link").
		Msg("mock handler")
	return StatusOK, nil
}

func (h *defaultSMPPHandler) HandleDeliverSMResp(_ context.Context, sequenceNumber uint32, status uint32, session *Session) error {
	session.logger.Debug().
		Str("handler", "default_smpp").
		Str("event", "deliver_sm_resp").
		Uint32("sequence", sequenceNumber).
		Uint32("status", status).
		Msg("mock handler")
	return nil
}
