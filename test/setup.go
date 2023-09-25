// package test temporarily contains all commonly used, generic utils for testing.
package test

import (
	"fmt"

	"github.com/retail-ai-inc/bean"
	"github.com/spf13/viper"
)

type environment string

var env environment

const (
	LOCAL environment = "local"
	STG   environment = "staging"
	PROD  environment = "production"
)

func (e environment) IsCurrEnv() bool {
	return e == env
}

func setEnv(envStr string) error {
	switch envStr {
	case string(LOCAL):
		env = LOCAL
	case string(STG):
		env = STG
	case string(PROD):
		env = PROD
	default:
		return fmt.Errorf("invalid environment: %s", envStr)
	}
	return nil
}

type CfgOption func(*TestConfig)

// SetupConfig initializes the configuration and accepts optional configuration functions
func SetupConfig(configPath string, opts ...CfgOption) error {
	for _, opt := range opts {
		opt(&TestCfg)
	}

	viper.AddConfigPath(configPath)
	viper.SetConfigType("json")
	viper.SetConfigName("env")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	if err := viper.Unmarshal(&bean.BeanConfig); err != nil {
		return err
	}
	if err := setEnv(bean.BeanConfig.Environment); err != nil {
		return err
	}
	if PROD.IsCurrEnv() {
		return fmt.Errorf("tests are not run in the %s environment", env)
	}

	if err := viper.UnmarshalKey("test", &TestCfg); err != nil {
		return err
	}

	return nil
}

// SetupConfigWithFixture initializes the configuration and custom fixture.
// It finally overwrites a given fixture as argument with an initialized one which should have some values you set beforehand.
func SetupConfigWithFixture[T any](configPath string, custom *T) error {

	if err := SetupConfig(
		configPath,
		WithFixture(custom),
	); err != nil {
		return err
	}
	initialized, ok := TestCfg.Fixture.(*T)
	if !ok {
		return fmt.Errorf("Failed to assert type of custom fixture")
	}
	*custom = *initialized

	return nil
}
