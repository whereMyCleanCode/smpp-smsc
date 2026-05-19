package smsc

import (
	"context"
	"testing"
	"time"
)

type containerTestHandler struct{}

func (h *containerTestHandler) HandleBindTransceiver(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.logger.Info().
		Str("handler", "container_test").
		Str("event", "bind_transceiver").
		Str("system_id", params["system_id"]).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *containerTestHandler) HandleBindReceiver(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.logger.Info().
		Str("handler", "container_test").
		Str("event", "bind_receiver").
		Str("system_id", params["system_id"]).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *containerTestHandler) HandleBindTransmitter(_ context.Context, params map[string]string, session *Session) (uint32, error) {
	session.logger.Info().
		Str("handler", "container_test").
		Str("event", "bind_transmitter").
		Str("system_id", params["system_id"]).
		Msg("mock handler")
	return StatusOK, nil
}

func (h *containerTestHandler) HandleSubmitSM(_ context.Context, params *SubmitSmParams, session *Session) *SmppResponse {
	session.logger.Info().
		Str("handler", "container_test").
		Str("event", "submit_sm").
		Str("source", params.SourceAddr).
		Str("destination", params.DestAddr).
		Str("text", params.Text).
		Msg("mock handler")
	return &SmppResponse{Status: StatusOK}
}

func (h *containerTestHandler) HandleUnbind(_ context.Context, session *Session) (uint32, error) {
	session.logger.Info().
		Str("handler", "container_test").
		Str("event", "unbind").
		Msg("mock handler")
	return StatusOK, nil
}

func (h *containerTestHandler) HandleEnquireLink(_ context.Context, session *Session) (uint32, error) {
	session.logger.Debug().
		Str("handler", "container_test").
		Str("event", "enquire_link").
		Msg("mock handler")
	return StatusOK, nil
}

func (h *containerTestHandler) HandleDeliverSMResp(_ context.Context, sequenceNumber uint32, status uint32, session *Session) error {
	session.logger.Debug().
		Str("handler", "container_test").
		Str("event", "deliver_sm_resp").
		Uint32("sequence", sequenceNumber).
		Uint32("status", status).
		Msg("mock handler")
	return nil
}

func TestConfigSettersFluentAPI(t *testing.T) {
	cfg := DefaultConfig().
		SetAddress(":3000").
		SetPodID("pod-x").
		SetSystemID("sys-x").
		SetLogLevel("debug").
		SetPrettyLogs(false).
		SetColorLogs(false).
		SetStartupVerbose(false).
		SetTimeout(5 * time.Second).
		SetInactivityTimeout(3 * time.Second).
		SetSegmentsTTL(2 * time.Minute).
		SetMaxEnquireLinkRetryCount(9).
		SetWindowSize(1024).
		SetDecoderBufferSize(16 * 1024).
		SetMaxWriteWorkers(2).
		SetMaxReadWorkers(2).
		SetDefaultMaxRPSLimit(200).
		SetDefaultBurstRPSLimit(250).
		SetDefaultMaxSegmentsCount(10).
		SetMaxSubmitSMSegments(22).
		SetTCPNoDelay(false).
		SetTCPKeepAlive(false).
		SetTCPKeepAlivePeriod(30 * time.Second).
		SetTCPReadBufferSize(64 * 1024).
		SetTCPWriteBufferSize(64 * 1024).
		SetTCPLinger(1).
		SetSessionCacheCapacity(321).
		SetSessionCacheInactiveTimeout(11 * time.Second)

	if cfg.Address != ":3000" || cfg.PodID != "pod-x" || cfg.SystemID != "sys-x" {
		t.Fatalf("identity fields were not set correctly")
	}
	if cfg.SessionCache.Cap != 321 || cfg.SessionCache.InactiveTimeout != 11*time.Second {
		t.Fatalf("session cache fields were not set correctly")
	}
	if cfg.Timeout != 5*time.Second || cfg.TCPKeepAlivePeriod != 30*time.Second {
		t.Fatalf("duration fields were not set correctly")
	}
	if cfg.MaxSubmitSMSegments != 22 {
		t.Fatalf("MaxSubmitSMSegments was not set correctly")
	}
}

func TestContainerBuildServerWithDependencies(t *testing.T) {
	cfg := DefaultConfig().SetAddress(":0")
	logger := newTestLogger()
	idGen := &stubIDGenerator{}
	handler := &containerTestHandler{}

	srv, err := NewContainer().
		WithConfig(cfg).
		WithLogger(logger).
		WithIDGenerator(idGen).
		WithHandler(handler).
		BuildServer()
	if err != nil {
		t.Fatalf("BuildServer returned error: %v", err)
	}

	if srv.cfg != cfg {
		t.Fatalf("server config mismatch")
	}
	if srv.idGenerator != idGen {
		t.Fatalf("server id generator mismatch")
	}
	if srv.handler != handler {
		t.Fatalf("server handler mismatch")
	}
}
