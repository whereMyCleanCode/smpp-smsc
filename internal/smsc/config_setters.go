package smsc

import "time"

func (c *Config) SetAddress(address string) *Config {
	c.Address = address
	return c
}

func (c *Config) SetPodID(podID string) *Config {
	c.PodID = podID
	return c
}

func (c *Config) SetSystemID(systemID string) *Config {
	c.SystemID = systemID
	return c
}

func (c *Config) SetLogLevel(level string) *Config {
	c.LogLevel = level
	return c
}

func (c *Config) SetPrettyLogs(enabled bool) *Config {
	c.PrettyLogs = enabled
	return c
}

func (c *Config) SetColorLogs(enabled bool) *Config {
	c.ColorLogs = enabled
	return c
}

func (c *Config) SetStartupVerbose(enabled bool) *Config {
	c.StartupVerbose = enabled
	return c
}

func (c *Config) SetTimeout(timeout time.Duration) *Config {
	c.Timeout = timeout
	return c
}

func (c *Config) SetInactivityTimeout(timeout time.Duration) *Config {
	c.InactivityTimeout = timeout
	return c
}

func (c *Config) SetSegmentsTTL(ttl time.Duration) *Config {
	c.SegsBucketTtl = ttl
	return c
}

func (c *Config) SetMaxEnquireLinkRetryCount(count int) *Config {
	c.MaxEnquireLinkRetry = count
	return c
}

func (c *Config) SetWindowSize(size int) *Config {
	c.WindowSize = size
	return c
}

func (c *Config) SetDecoderBufferSize(size int) *Config {
	c.DecoderBufferSize = size
	return c
}

func (c *Config) SetMaxWriteWorkers(count int) *Config {
	c.MaxWriteWorkers = count
	return c
}

func (c *Config) SetMaxReadWorkers(count int) *Config {
	c.MaxReadWorkers = count
	return c
}

func (c *Config) SetDefaultMaxRPSLimit(limit int) *Config {
	c.DefaultMaxRPSLimit = limit
	return c
}

func (c *Config) SetDefaultBurstRPSLimit(limit int) *Config {
	c.DefaultBurstRPSLimit = limit
	return c
}

func (c *Config) SetDefaultMaxSegmentsCount(limit int) *Config {
	c.DefaultMaxSegsCount = limit
	return c
}

func (c *Config) SetMaxSubmitSMSegments(limit int) *Config {
	c.MaxSubmitSMSegments = limit
	return c
}

func (c *Config) SetTCPNoDelay(enabled bool) *Config {
	c.TCPNoDelay = enabled
	return c
}

func (c *Config) SetTCPKeepAlive(enabled bool) *Config {
	c.TCPKeepAlive = enabled
	return c
}

func (c *Config) SetTCPKeepAlivePeriod(period time.Duration) *Config {
	c.TCPKeepAlivePeriod = period
	return c
}

func (c *Config) SetTCPReadBufferSize(size int) *Config {
	c.TCPReadBufferSize = size
	return c
}

func (c *Config) SetTCPWriteBufferSize(size int) *Config {
	c.TCPWriteBufferSize = size
	return c
}

func (c *Config) SetTCPLinger(linger int) *Config {
	c.TCPLinger = linger
	return c
}

func (c *Config) SetSessionCacheCapacity(capacity int) *Config {
	c.SessionCache.Cap = capacity
	return c
}

func (c *Config) SetSessionCacheInactiveTimeout(timeout time.Duration) *Config {
	c.SessionCache.InactiveTimeout = timeout
	return c
}
