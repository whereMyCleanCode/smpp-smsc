package smsc

import "context"

type SMPPHandler interface {
	HandleBindTransceiver(ctx context.Context, params map[string]string, session *Session) (uint32, error)
	HandleBindReceiver(ctx context.Context, params map[string]string, session *Session) (uint32, error)
	HandleBindTransmitter(ctx context.Context, params map[string]string, session *Session) (uint32, error)
	HandleSubmitSM(ctx context.Context, params *SubmitSmParams, session *Session) *SmppResponse
	HandleUnbind(ctx context.Context, session *Session) (uint32, error)
	HandleEnquireLink(ctx context.Context, session *Session) (uint32, error)
	HandleDeliverSMResp(ctx context.Context, sequenceNumber uint32, status uint32, session *Session) error
}

type SessionStorage interface {
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetSessionsByPod(ctx context.Context, podID string) ([]StorageSession, error)
	GetSessionsByApplicationID(ctx context.Context, applicationID string) ([]StorageSession, error)
	DeletePodSessions(ctx context.Context, podID string) error
}

type StorageSession struct {
	SessionID     string
	ApplicationID string
	PodID         string
	Type          string
	UpdatedAt     string
}
