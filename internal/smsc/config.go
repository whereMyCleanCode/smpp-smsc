package smsc

import "time"

type SessionCacheConfig struct {
	Cap             int
	InactiveTimeout time.Duration
}

type Config struct {
	Address string

	PodID    string
	SystemID string

	LogLevel string
	// PrettyLogs enables human-readable console logs instead of JSON.
	PrettyLogs bool
	// ColorLogs enables ANSI colors for pretty console output.
	ColorLogs bool
	// StartupVerbose enables additional startup diagnostics.
	StartupVerbose bool

	Timeout             time.Duration
	InactivityTimeout   time.Duration
	SegsBucketTtl       time.Duration
	MaxEnquireLinkRetry int

	PendingRequestTtl              time.Duration `default:"74h"`
	PendingRequestsCleanupInterval time.Duration `default:"1h"`

	WindowSize        int
	DecoderBufferSize int
	MaxWriteWorkers   int
	MaxReadWorkers    int

	// Rate limiting: global and per-session
	// GlobalRateLimiterEnabled enables a shared rate limiter across all sessions.
	// If true, GlobalMaxRPSLimit and GlobalBurstRPSLimit are used.
	GlobalRateLimiterEnabled bool
	GlobalMaxRPSLimit        int
	GlobalBurstRPSLimit      int

	// Per-session rate limiter (default enabled).
	// Each session gets its own rate limiter configured via DefaultMaxRPSLimit/DefaultBurstRPSLimit.
	PerSessionRateLimiterEnabled bool
	DefaultMaxRPSLimit           int
	DefaultBurstRPSLimit         int
	DefaultMaxSegsCount          int
	// MaxSubmitSMSegments is the maximum number of inbound multipart Submit SM segments (SAR/UDH)
	// accepted per logical message. Default 10; configurable up to 25 (hard cap). Not related to
	// DefaultMaxSegsCount, which limits outgoing Deliver SM text length before splitting.
	MaxSubmitSMSegments int
	// MaxMessagePayloadLen is the maximum allowed length of the optional message_payload TLV (0x0424).
	// A value of 0 means no limit is enforced. Default 4096.
	MaxMessagePayloadLen int

	TCPNoDelay         bool
	TCPKeepAlive       bool
	TCPKeepAlivePeriod time.Duration
	TCPReadBufferSize  int
	TCPWriteBufferSize int
	TCPLinger          int

	SessionCache SessionCacheConfig
}

func DefaultConfig() *Config {
	return &Config{
		Address:                        ":2775",
		PodID:                          "smsc-1",
		SystemID:                       "SMSC",
		LogLevel:                       "info",
		PrettyLogs:                     true,
		ColorLogs:                      true,
		StartupVerbose:                 true,
		Timeout:                        90 * time.Second,
		InactivityTimeout:              30 * time.Second,
		SegsBucketTtl:                  3 * time.Minute,
		MaxEnquireLinkRetry:            3,
		PendingRequestTtl:              74 * time.Hour,
		PendingRequestsCleanupInterval: time.Hour,
		WindowSize:                     2000,
		DecoderBufferSize:              128 * 1024,
		MaxWriteWorkers:                1,
		MaxReadWorkers:                 1,
		GlobalRateLimiterEnabled:       false,
		GlobalMaxRPSLimit:              0,
		GlobalBurstRPSLimit:            0,
		PerSessionRateLimiterEnabled:   true,
		DefaultMaxRPSLimit:             1500,
		DefaultBurstRPSLimit:           1800,
		DefaultMaxSegsCount:            200,
		MaxSubmitSMSegments:            10,
		MaxMessagePayloadLen:           4096,
		TCPNoDelay:                     true,
		TCPKeepAlive:                   true,
		TCPKeepAlivePeriod:             60 * time.Second,
		TCPReadBufferSize:              256 * 1024,
		TCPWriteBufferSize:             256 * 1024,
		TCPLinger:                      5,
		SessionCache: SessionCacheConfig{
			Cap:             10000,
			InactiveTimeout: 30 * time.Second,
		},
	}
}
