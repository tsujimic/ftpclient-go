package ftpclient

import (
	"crypto/tls"
	"time"
)

// Config ...
type Config struct {
	tlsConfig        *tls.Config
	tlsImplicit      bool
	logger           Logger
	readWriteTimeout time.Duration
}

// NewConfig ...
func NewConfig() *Config {
	return &Config{
		tlsImplicit:      false,
		readWriteTimeout: 120 * time.Second,
	}
}

// WithLogger sets a config Logger value returning a Config pointer for chaining.
func (c *Config) WithLogger(logger Logger) *Config {
	c.logger = logger
	return c
}

// WithTLSConfig sets a config tlsConfig value returning a Config pointer for chaining.
func (c *Config) WithTLSConfig(config *tls.Config) *Config {
	c.tlsConfig = config
	return c
}

// WithTLSImplicit sets a config tlsImplicit value returning a Config pointer for chaining.
func (c *Config) WithTLSImplicit(implicit bool) *Config {
	c.tlsImplicit = implicit
	return c
}

// WithReadWriteTimeout sets a config ReadWriteTimeout value returning a Config pointer for chaining.
func (c *Config) WithReadWriteTimeout(time time.Duration) *Config {
	c.readWriteTimeout = time
	return c
}
