package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/whereMyCleanCode/smpp-smsc/internal/smsc"
)

const appVersion = "dev"

type demoHandler struct {
	lgr smsc.Logger
}

func (h *demoHandler) HandleBindTransceiver(_ context.Context, params map[string]string, session *smsc.Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = smsc.BindingTypeTransceiver
	session.Bound = true
	h.lgr.Info().
		Str("handler", "demo").
		Str("event", "bind_transceiver").
		Str("session_id", session.ID).
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleBindReceiver(_ context.Context, params map[string]string, session *smsc.Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = smsc.BindingTypeReceiver
	session.Bound = true
	h.lgr.Info().
		Str("handler", "demo").
		Str("event", "bind_receiver").
		Str("session_id", session.ID).
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleBindTransmitter(_ context.Context, params map[string]string, session *smsc.Session) (uint32, error) {
	session.SystemID = params["system_id"]
	session.Password = params["password"]
	session.BindingType = smsc.BindingTypeTransmitter
	session.Bound = true
	h.lgr.Info().
		Str("handler", "demo").
		Str("event", "bind_transmitter").
		Str("session_id", session.ID).
		Str("system_id", session.SystemID).
		Msg("mock handler")
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleSubmitSM(_ context.Context, params *smsc.SubmitSmParams, session *smsc.Session) *smsc.SmppResponse {
	if !session.BindingType.IsTransmitter() {
		h.lgr.Info().
			Str("handler", "demo").
			Str("event", "submit_sm").
			Str("session_id", session.ID).
			Str("reason", "invalid_binding").
			Msg("mock handler")
		return &smsc.SmppResponse{Status: smsc.StatusInvBnd}
	}
	if params.SourceAddr == "" || params.DestAddr == "" {
		h.lgr.Info().
			Str("handler", "demo").
			Str("event", "submit_sm").
			Str("session_id", session.ID).
			Str("reason", "missing_addresses").
			Msg("mock handler")
		return &smsc.SmppResponse{Status: smsc.StatusInvSrcAdr}
	}
	h.lgr.Info().
		Str("handler", "demo").
		Str("event", "submit_sm").
		Str("session_id", session.ID).
		Str("source", params.SourceAddr).
		Str("destination", params.DestAddr).
		Msg("mock handler")
	return &smsc.SmppResponse{Status: smsc.StatusOK}
}

func (h *demoHandler) HandleUnbind(_ context.Context, session *smsc.Session) (uint32, error) {
	session.Bound = false
	h.lgr.Info().
		Str("handler", "demo").
		Str("event", "unbind").
		Str("session_id", session.ID).
		Msg("mock handler")
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleEnquireLink(_ context.Context, session *smsc.Session) (uint32, error) {
	h.lgr.Debug().
		Str("handler", "demo").
		Str("event", "enquire_link").
		Str("session_id", session.ID).
		Msg("mock handler")
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleDeliverSMResp(_ context.Context, sequenceNumber uint32, status uint32, session *smsc.Session) error {
	h.lgr.Debug().
		Str("handler", "demo").
		Str("event", "deliver_sm_resp").
		Str("session_id", session.ID).
		Uint32("sequence", sequenceNumber).
		Uint32("status", status).
		Msg("mock handler")
	return nil
}

func main() {
	cfg := smsc.DefaultConfig().
		SetAddress(":2775").
		SetLogLevel("info").
		SetPrettyLogs(true).
		SetColorLogs(true).
		SetStartupVerbose(true)

	logger := smsc.NewLoggerWithOptions(
		os.Stdout,
		smsc.ParseLogLevel(cfg.LogLevel),
		smsc.LoggerOptions{
			Pretty:     cfg.PrettyLogs,
			Color:      cfg.ColorLogs,
			TimeFormat: "2006-01-02 15:04:05",
		},
	)

	logMode := "json"
	if cfg.PrettyLogs {
		logMode = "pretty"
	}
	logger.Info().
		Str("version", appVersion).
		Str("address", cfg.Address).
		Str("log_level", cfg.LogLevel).
		Str("log_mode", logMode).
		Bool("color_logs", cfg.ColorLogs).
		Bool("startup_verbose", cfg.StartupVerbose).
		Msg("startup summary")

	idGen, err := smsc.NewSnowflakeGenerator(1)
	if err != nil {
		panic(err)
	}

	container := smsc.NewContainer().
		WithConfig(cfg).
		WithLogger(logger).
		WithIDGenerator(idGen).
		WithHandler(&demoHandler{lgr: logger.WithStr("handler", "demo")})

	server, err := container.BuildServer()
	if err != nil {
		panic(err)
	}
	errCh := server.Start()
	logger.Info().Msg("signal handlers are being registered")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var shutdownReason string

	select {
	case sig := <-sigCh:
		shutdownReason = fmt.Sprintf("signal:%s", sig.String())
		logger.Warn().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error().Err(err).Msg("server exited with error")
			shutdownReason = "server_error"
		} else {
			shutdownReason = "server_stopped"
		}
	}

	logger.Info().Str("reason", shutdownReason).Msg("graceful shutdown started")
	server.Shutdown()
	logger.Info().Msg("graceful shutdown finished")
}
