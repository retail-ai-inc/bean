package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v2/internal/dbdrivers"
	"github.com/spf13/viper"
)

// Hold the useful configuration settings of bean so that we can use it quickly from anywhere.
var Bean *Config

type Config struct {
	ProjectName  string
	Environment  string
	DebugLogPath string
	Secret       string
	AccessLog    struct {
		On                bool
		BodyDump          bool
		Path              string
		BodyDumpMaskParam []string
		ReqHeaderParam    []string
		SkipEndpoints     []string
	}
	Prometheus struct {
		On            bool
		SkipEndpoints []string
		Subsystem     string
	}
	HTTP struct {
		Port            string
		Host            string
		BodyLimit       string
		IsHttpsRedirect bool
		Timeout         time.Duration
		ErrorMessage    struct {
			E404 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E405 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E500 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			E504 struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
			Default struct {
				Json []struct {
					Key   string
					Value string
				}
				Html struct {
					File string
				}
			}
		}
		KeepAlive     bool
		AllowedMethod []string
		SSL           struct {
			On            bool
			CertFile      string
			PrivFile      string
			MinTLSVersion uint16
		}
		ShutdownTimeout time.Duration
	}
	NetHttpFastTransporter struct {
		On                  bool
		MaxIdleConns        *int
		MaxIdleConnsPerHost *int
		MaxConnsPerHost     *int
		IdleConnTimeout     *time.Duration
		DNSCacheTimeout     *time.Duration
	}
	HTML struct {
		ViewsTemplateCache bool
	}
	Database struct {
		Tenant struct {
			On bool
		}
		MySQL  dbdrivers.SQLConfig
		Mongo  dbdrivers.MongoConfig
		Redis  dbdrivers.RedisConfig
		Memory dbdrivers.MemoryConfig
	}
	Sentry   Sentry
	Security struct {
		HTTP struct {
			Header struct {
				XssProtection         string
				ContentTypeNosniff    string
				XFrameOptions         string
				HstsMaxAge            int
				ContentSecurityPolicy string
			}
		}
	}
	AsyncPool []struct {
		Name       string
		Size       *int
		BlockAfter *int
	}
	AsyncPoolReleaseTimeout time.Duration
}

type Sentry struct {
	On                  bool
	Debug               bool
	Dsn                 string
	Timeout             time.Duration
	TracesSampleRate    float64
	SkipTracesEndpoints []string
	ClientOptions       *sentry.ClientOptions
	ConfigureScope      func(scope *sentry.Scope)
}

// LoadConfig parses a given config file into global Bean variable.
func LoadConfig(filename string) (*Config, error) {

	ext := filepath.Ext(filename)
	if ext == "" {
		return nil, fmt.Errorf("file extension is missing in the filename")
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}
	path := filepath.Dir(absPath)
	name := filepath.Base(filename[:len(filename)-len(ext)])

	viper.AddConfigPath(path)
	viper.SetConfigType(ext[1:])
	viper.SetConfigName(name)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file, %s", err)
	}

	Bean = &Config{}
	if err := viper.Unmarshal(Bean); err != nil {
		Bean = nil
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	return Bean, nil
}
