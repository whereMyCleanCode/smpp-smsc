package smsc

import "fmt"

type Container struct {
	config      *Config
	logger      Logger
	idGenerator IDGenerator
	handler     SMPPHandler
}

func NewContainer() *Container {
	return &Container{
		config: DefaultConfig(),
	}
}

func (c *Container) WithConfig(cfg *Config) *Container {
	if cfg != nil {
		c.config = cfg
	}
	return c
}

func (c *Container) WithLogger(logger Logger) *Container {
	c.logger = logger
	return c
}

func (c *Container) WithIDGenerator(gen IDGenerator) *Container {
	c.idGenerator = gen
	return c
}

func (c *Container) WithHandler(handler SMPPHandler) *Container {
	c.handler = handler
	return c
}

func (c *Container) Config() *Config {
	if c.config == nil {
		c.config = DefaultConfig()
	}
	return c.config
}

func (c *Container) BuildServer() (*Server, error) {
	srv, err := NewServer(c.Config(), c.logger, c.idGenerator)
	if err != nil {
		return nil, fmt.Errorf("build server from container: %w", err)
	}
	if c.handler != nil {
		srv.SetHandler(c.handler)
	}
	return srv, nil
}
