# smpp-smsc

`smpp-smsc` is an SMPP 3.4 SMSC runtime for Go.

It provides server-side SMPP flow handling (bind / submit_sm / deliver_sm / enquire_link / unbind), session lifecycle management, segmented message processing, and fast in-memory routing using Otter cache.

## Features

- SMPP server runtime over TCP
- Session lifecycle and inactivity management
- Submit/Deliver flow with handler-based business logic
- Segmented message reassembly with shard-based manager
- O(1) message-to-session routing for delivery/report flows
- Configurable pretty/json logs (color support for local dev)

## Quick Start

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/whereMyCleanCode/smpp-smsc/internal/smsc"
)

type demoHandler struct{}

func (h *demoHandler) HandleBindTransceiver(ctx context.Context, params map[string]string, s *smsc.Session) (uint32, error) {
	s.SystemID = params["system_id"]
	s.Password = params["password"]
	s.BindingType = smsc.BindingTypeTransceiver
	s.Bound = true
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleBindReceiver(ctx context.Context, params map[string]string, s *smsc.Session) (uint32, error) {
	s.SystemID = params["system_id"]
	s.Password = params["password"]
	s.BindingType = smsc.BindingTypeReceiver
	s.Bound = true
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleBindTransmitter(ctx context.Context, params map[string]string, s *smsc.Session) (uint32, error) {
	s.SystemID = params["system_id"]
	s.Password = params["password"]
	s.BindingType = smsc.BindingTypeTransmitter
	s.Bound = true
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleSubmitSM(_ context.Context, p *smsc.SubmitSmParams, s *smsc.Session) *smsc.SmppResponse {
	if !s.BindingType.IsTransmitter() {
		return smsc.ToSmppResponse(smsc.StatusInvBnd)
	}
	if p.SourceAddr == "" || p.DestAddr == "" {
		return smsc.ToSmppResponse(smsc.StatusInvSrcAdr)
	}
	return smsc.ToSmppResponse(smsc.StatusOK)
}

func (h *demoHandler) HandleUnbind(_ context.Context, s *smsc.Session) (uint32, error) {
	s.Bound = false
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleEnquireLink(_ context.Context, _ *smsc.Session) (uint32, error) {
	return smsc.StatusOK, nil
}

func (h *demoHandler) HandleDeliverSMResp(_ context.Context, _ uint32, _ uint32, _ *smsc.Session) error {
	return nil
}

func main() {
	cfg := smsc.DefaultConfig()
	cfg.Address = ":2775"
	cfg.PrettyLogs = true
	cfg.ColorLogs = true
	cfg.StartupVerbose = true

	logger := smsc.NewLoggerWithOptions(
		os.Stdout,
		smsc.ParseLogLevel(cfg.LogLevel),
		smsc.LoggerOptions{
			Pretty: cfg.PrettyLogs,
			Color:  cfg.ColorLogs,
		},
	)

	idGen, err := smsc.NewSnowflakeGenerator(1)
	if err != nil {
		panic(err)
	}

	server, err := smsc.NewServer(cfg, logger, idGen)
	if err != nil {
		panic(err)
	}

	server.SetHandler(&demoHandler{})
	errCh := server.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
	case err := <-errCh:
		if err != nil {
			logger.Error().Err(err).Msg("server exited with error")
		}
	}

	server.Shutdown()
}
```

## Architecture Overview

### Core Components

- `Server`
  - Listens for TCP connections
  - Applies socket-level settings
  - Creates and initializes sessions
  - Coordinates startup/shutdown lifecycle

- `Session`
  - Owns connection read/write loop
  - Parses incoming SMPP PDUs
  - Handles bind/submit/enquire/unbind flows
  - Tracks pending requests and activity timestamps

- `SessionsManager`
  - Stores active sessions
  - Provides lookup by session ID, app ID, pod ID, and message ID
  - Runs inactivity checks and EnquireLink retry workflow

- `SegmentsManager`
  - Reassembles long/segmented messages
  - Uses shard-based storage to reduce lock contention
  - Cleans expired segment buckets

### Otter Cache Design

`smpp-smsc` uses two cache layers powered by Otter:

1. **Session cache**: `sessionID -> *Session`
   - Access-based expiration for inactive session cleanup
   - Automatic eviction callback for session stop/cleanup

2. **Message cache**: `messageID -> sessionID`
   - Write TTL for delivery/report correlation window
   - O(1) lookup for routing delivery-related events back to a session

This design keeps hot-path routing fast and avoids stale memory buildup in long-running processes.

## Logging

You can configure logging style using `Config`:

- `LogLevel`: trace/debug/info/warn/error/fatal
- `PrettyLogs`: human-readable console output
- `ColorLogs`: colorized levels in pretty mode
- `StartupVerbose`: extended startup diagnostics

## Development

```bash
go test ./...
```

## Scope

This repository focuses on SMPP runtime features (sessions, managers, connections, submit/deliver flow) and intentionally excludes infra concerns like Helm, CI lint pipelines, and Prometheus integration.
