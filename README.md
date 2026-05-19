# smpp-smsc

`smpp-smsc` is an SMPP 3.4 SMSC runtime for Go.

It provides server-side SMPP flow handling (bind / submit_sm / deliver_sm / enquire_link / unbind), session lifecycle management, segmented message processing, and fast in-memory routing using Otter cache.

## Features

- SMPP server runtime over TCP
- Session lifecycle and inactivity management
- Submit/Deliver flow with handler-based business logic
- Segmented message reassembly with shard-based manager
- O(1) message-to-session routing for delivery/report flows
- Optional per-session application metadata (`SetSessionMeta` / `GetSessionMeta` / `SessionMeta`)
- Configurable pretty/json logs (color support for local dev)
- Full `submit_sm` mandatory field parsing passed to external handlers
- Raw access to all `submit_sm` TLVs via `SubmitSmParams.TLVParams`

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

type demoHandler struct {
	lgr smsc.Logger
}

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
		return &smsc.SmppResponse{Status: smsc.StatusInvBnd}
	}
	if p.SourceAddr == "" || p.DestAddr == "" {
		return &smsc.SmppResponse{Status: smsc.StatusInvSrcAdr}
	}
	return &smsc.SmppResponse{Status: smsc.StatusOK}
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

	server.SetHandler(&demoHandler{lgr: logger.WithStr("handler", "demo")})
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
  - Stores optional **application metadata** (not part of SMPP): use `SetSessionMeta(key, value)` from bind handlers or any code that holds `*Session` to attach tenant IDs, product flags, auth claims, routing hints, etc. Read with `GetSessionMeta(key)` or a full copy via `SessionMeta()` (thread-safe; safe to read from other goroutines after publish). This data is never written to the wire unless your own logic sends it.

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

## Session application metadata

Each `*smsc.Session` can carry an internal `map[string]string` for **your** data (tenant, API key id, A/B flags, etc.). It is **not** an SMPP field: nothing is serialized to the client unless you use it in your handlers or when sending PDUs yourself.

API:

- `SetSessionMeta(key, value string)` — set or overwrite one entry
- `GetSessionMeta(key string) (string, bool)` — read one entry
- `SessionMeta() map[string]string` — copy of all entries (mutating the returned map does not affect the session)

Typical use: call `SetSessionMeta` from `HandleBindTransceiver` / `HandleBindReceiver` / `HandleBindTransmitter` after you validate `system_id`, then read metadata in `HandleSubmitSM` or in code that resolves `*Session` by ID.

## Logging

You can configure logging style using `Config`:

- `LogLevel`: trace/debug/info/warn/error/fatal
- `PrettyLogs`: human-readable console output
- `ColorLogs`: colorized levels in pretty mode
- `StartupVerbose`: extended startup diagnostics

## HandleSubmitSM Parameters

`HandleSubmitSM` receives `*smsc.SubmitSmParams` with:

- All mandatory `submit_sm` fields parsed (addresses, TON/NPI, esm/protocol/priority, schedule/validity, registered_delivery, replace_if_present, data_coding, sm_default_msg_id, short_message).
- `TLVParams map[uint16][]byte` containing all optional TLVs in raw bytes.

This allows handler implementations to apply custom business logic without losing protocol-level data.

`registered_delivery` handling follows SMPP semantics by receipt type in the lower 2 bits:

- `0x00`: no delivery receipt requested
- `0x01`: receipt on final outcome (success or failure)
- `0x02`: receipt on failure only
- `0x03`: receipt on success only

`replace_if_present`: when set on `submit_sm`, the runtime looks up the last **accepted** submit (same `service_type`, `source_addr`, `destination_addr`, `sm_default_msg_id`) on that session. If found, pending DLR correlation for the previous internal message ID is dropped before the new submit is processed, so delivery-report routing does not keep stale IDs.

## Delivery receipts (MT → ESME)

When a client requested a receipt on `submit_sm`, the SMSC stores `PendingRequest` (segment count and raw `registered_delivery`) under the internal message id. To emit a GSM **delivery receipt** toward the bound receiver/transceiver, use `Server.SendDeliveryReport` with:

- `messageIDStr`: the same `message_id` string you returned in `submit_sm_resp` (it becomes the `id:` field in the receipt body).
- `internalMessageID`: the internal `uint64` message id used as the key in `PendingRequests` (from `HandleSubmitSM` / segment completion).
- `success`: final delivery outcome; `registered_delivery` policy (success-only / failure-only / both) decides whether a `deliver_sm` is sent; skipped outcomes return `DeliveryReportSkipped*` and still clear the pending entry.
- **Addresses**: `source_addr` / `destination_addr` on the receipt are typically the MT destination (recipient) and original submit source respectively (swap relative to `submit_sm` direction).

Pending Requests by DLR have ttl by 74h  inb default if you want use other val make issue. Cleanup star work every hour

The receipt short message is formatted as `id:… sub:… dlvrd:… submit date:… done date:… stat:… err:… text:…`. For failures, `dlvrd` is `000` while `sub` reflects the submitted segment count; for success, `sub` and `dlvrd` match (at least one segment). Helpers `FormatDeliveryReceiptString` and `BuildDeliveryReceiptFromPending` live in the same package for tests or custom send paths.

## Development

```bash
go test ./...
```

## Scope

This repository focuses on SMPP runtime features (sessions, managers, connections, submit/deliver flow) and intentionally excludes infra concerns like Helm, CI lint pipelines, and Prometheus integration.
