package smsc

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"time"
)

type stubIDGenerator struct {
	mu   sync.Mutex
	next uint64
}

func (g *stubIDGenerator) GenerateID() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return g.next, nil
}

type mockAddr struct {
	network string
	addr    string
}

func (a mockAddr) Network() string { return a.network }
func (a mockAddr) String() string  { return a.addr }

type mockConn struct {
	mu sync.Mutex

	readBuf  bytes.Buffer
	writeBuf bytes.Buffer

	closed bool
}

func newMockConn() *mockConn {
	return &mockConn{}
}

func (c *mockConn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.EOF
	}
	return c.readBuf.Read(b)
}

func (c *mockConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	return c.writeBuf.Write(b)
}

func (c *mockConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *mockConn) LocalAddr() net.Addr {
	return mockAddr{network: "tcp", addr: "127.0.0.1:2775"}
}

func (c *mockConn) RemoteAddr() net.Addr {
	return mockAddr{network: "tcp", addr: "127.0.0.1:30000"}
}

func (c *mockConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *mockConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *mockConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func (c *mockConn) writeData(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readBuf.Write(data)
}

func (c *mockConn) writtenBytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, c.writeBuf.Len())
	copy(out, c.writeBuf.Bytes())
	return out
}

func newTestConfig() *Config {
	cfg := DefaultConfig()
	cfg.Timeout = 200 * time.Millisecond
	cfg.InactivityTimeout = 50 * time.Millisecond
	cfg.MaxEnquireLinkRetry = 1
	cfg.WindowSize = 64
	cfg.DecoderBufferSize = 4096
	return cfg
}

func newTestLogger() Logger {
	return NewLogger(io.Discard, DebugLevel)
}

func newTestContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}
