package dbdrivers

import "testing"

func TestSSLConfigShouldVerifyCertificate(t *testing.T) {
	t.Run("defaults to skip certificate verification", func(t *testing.T) {
		ssl := SSLConfig{On: true}

		if ssl.ShouldVerifyCertificate() {
			t.Fatal("expected certificate verification to be disabled by default")
		}
	})

	t.Run("allows enabling certificate verification", func(t *testing.T) {
		verifyCertificate := true
		ssl := SSLConfig{On: true, VerifyCertificate: &verifyCertificate}

		if !ssl.ShouldVerifyCertificate() {
			t.Fatal("expected certificate verification to be enabled")
		}

		tlsConfig, err := newTLSConfig(ssl)
		if err != nil {
			t.Fatalf("expected TLS config: %v", err)
		}
		if tlsConfig.InsecureSkipVerify {
			t.Fatal("expected InsecureSkipVerify to be disabled")
		}
	})

	t.Run("parses hotReload from map", func(t *testing.T) {
		cfg := map[string]interface{}{
			"ssl": map[string]interface{}{
				"on":        true,
				"hotReload": true,
			},
		}
		ssl := sslConfigFromMap(cfg)
		if !ssl.HotReload {
			t.Fatal("expected hotReload to be enabled")
		}
	})
}
