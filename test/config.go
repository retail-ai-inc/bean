package test

import (
	"time"
)

var TestCfg = TestConfig{}

type TestConfig struct {
	Skip       []string
	HTTPClient struct {
		Timeout          time.Duration
		RetryCount       int
		RetryWaitTime    time.Duration
		RetryMaxWaitTime time.Duration
	}
	Fixture any
}

// WithFixture is an option function to set the Fixture
func WithFixture[T any](custom T) CfgOption {
	return func(config *TestConfig) {
		config.Fixture = custom
	}
}
