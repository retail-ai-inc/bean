package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/v3/internal/dbdrivers"
	"github.com/spf13/viper"
)

// Hold the useful configuration settings of bean so that we can use it quickly from anywhere.
var Bean *Config

type Config struct {
	ProjectName  string
	Environment  string
	DebugLogPath string
	// DebugLogLevel is the minimum level of the application logs: debug (default), info, warn or error.
	DebugLogLevel string
	Secret        string
	AccessLog     struct {
		On              bool
		Async           bool
		AsyncQueueSize  int
		BodyLimit       int
		BodyDump        bool
		Path            string
		RuntimePlatform string
		// MaskFields lists field names whose values are masked in every
		// structured access-log entry (headers, JSON/XML bodies, any nested key).
		MaskFields     []string
		ReqHeaderParam []string
		ResHeaderParam []string
		SkipEndpoints  []string
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
		// TrustIPRange lists proxy CIDRs or single IPs allowed to supply X-Forwarded-For.
		// Loopback peers are trusted. Private and link-local addresses are not trusted unless listed here.
		// Empty means the service is reached directly and RealIP uses the connection peer.
		TrustIPRange []string
		Timeout      time.Duration
		ErrorMessage struct {
			E404    ErrorPage
			E405    ErrorPage
			E500    ErrorPage
			E504    ErrorPage
			Default ErrorPage
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
				ReferrerPolicy        string
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

	if err := CheckRemovedKeys(); err != nil {
		return nil, err
	}

	Bean = &Config{}
	if err := viper.Unmarshal(Bean); err != nil {
		Bean = nil
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	return Bean, nil
}

// ErrorPage customizes the error response of an HTTP status (`http.errorMessage.*` in env.json).
type ErrorPage struct {
	// Json is the JSON response body as key/value pairs. Empty means bean's default error JSON.
	Json []struct {
		Key   string
		Value string
	}
	Html struct {
		// File is the HTML template rendered for the error. Empty means bean's default template.
		File string
	}
}

// JSONBody returns the configured JSON response body, or nil if none is configured.
func (p ErrorPage) JSONBody() map[string]any {
	if len(p.Json) == 0 {
		return nil
	}
	body := make(map[string]any, len(p.Json))
	for _, kv := range p.Json {
		body[kv.Key] = kv.Value
	}
	return body
}

// CheckRemovedKeys rejects config keys that were removed in a breaking change
// when they still carry a value. Unknown keys are otherwise ignored silently by
// viper, which for a security setting would quietly turn the protection off.
// It checks the config loaded into viper; `LoadConfig` and `bean.New` call it.
func CheckRemovedKeys() error {
	if len(viper.GetStringSlice("accessLog.bodyDumpMaskParam")) > 0 {
		return fmt.Errorf("config: accessLog.bodyDumpMaskParam has been removed, rename it to accessLog.maskFields")
	}
	return nil
}
