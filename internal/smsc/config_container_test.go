package smsc

import (
	"context"
	"testing"
	"time"
)

type containerTestHandler struct{}

func (h *containerTestHandler) HandleBindTransceiver(_ context.Context, _ map[string]string, _ *Session) (uint32, error) {
	return StatusOK, nil
}

func (h *containerTestHandler) HandleBindReceiver(_ context.Context, _ map[string]string, _ *Session) (uint32, error) {
	return StatusOK, nil
}

func (h *containerTestHandler) HandleBindTransmitter(_ context.Context, _ map[string]string, _ *Session) (uint32, error) {
	return StatusOK, nil
}

func (h *containerTestHandler) HandleSubmitSM(_ context.Context, _ *SubmitSmParams, _ *Session) *SmppResponse {
	return &SmppResponse{Status: StatusOK}
}

func (h *containerTestHandler) HandleUnbind(_ context.Context, _ *Session) (uint32, error) {
	return StatusOK, nil
}

func (h *containerTestHandler) HandleEnquireLink(_ context.Context, _ *Session) (uint32, error) {
	return StatusOK, nil
}

func (h *containerTestHandler) HandleDeliverSMResp(_ context.Context, _ uint32, _ uint32, _ *Session) error {
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
