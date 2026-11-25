package dbdrivers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

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

		tlsConfig, _, err := newTLSConfig(ssl, nil)
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

// testCA issues server certificates signed by a test CA, written to a PEM file usable as `ssl.certFile`.
type testCA struct {
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
	certFile string
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "bean test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	certFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: cert, key: key, certFile: certFile}
}

// serve starts a TLS server presenting a certificate for the given DNS names / IP addresses and returns its address.
func (ca *testCA) serve(t *testing.T, names ...string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = conn.(*tls.Conn).Handshake()
				_ = conn.Close()
			}()
		}
	}()
	return ln.Addr().String()
}

// With hotReload the certificate is verified by bean (VerifyConnection); the server name must be verified too,
// including when the client connects to an IP address (no SNI).
func TestNewTLSConfig_HotReloadVerifiesServerName(t *testing.T) {
	ca := newTestCA(t)
	verify := true
	ssl := SSLConfig{On: true, CertFile: ca.certFile, VerifyCertificate: &verify, HotReload: true}

	dial := func(t *testing.T, addr, serverName string, hosts []string) error {
		t.Helper()
		cfg, release, err := newTLSConfig(ssl, hosts)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		cfg = cfg.Clone()
		cfg.ServerName = serverName // like the drivers: the dialed host, or empty
		conn, err := tls.Dial("tcp", addr, cfg)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}

	tests := []struct {
		name       string
		certNames  []string
		serverName string
		hosts      []string
		wantErr    bool
	}{
		{name: "ip host, certificate for the ip", certNames: []string{"127.0.0.1"}, serverName: "127.0.0.1", hosts: []string{"127.0.0.1"}},
		{name: "ip host, certificate for another server", certNames: []string{"evil.example"}, serverName: "127.0.0.1", hosts: []string{"127.0.0.1"}, wantErr: true},
		{name: "dns host, certificate for the host", certNames: []string{"db.example"}, serverName: "db.example", hosts: []string{"db.example"}},
		{name: "dns host, certificate for another server", certNames: []string{"evil.example"}, serverName: "db.example", hosts: []string{"db.example"}, wantErr: true},
		{name: "no server name and no host", certNames: []string{"db.example"}, serverName: "", hosts: nil, wantErr: true},
		{name: "ip of one of several hosts", certNames: []string{"127.0.0.1"}, serverName: "127.0.0.1", hosts: []string{"10.0.0.1", "127.0.0.1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := ca.serve(t, tt.certNames...)
			err := dial(t, addr, tt.serverName, tt.hosts)
			if tt.wantErr && err == nil {
				t.Fatal("expected the connection to be rejected")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected the connection to succeed: %v", err)
			}
		})
	}
}

func TestHostsWithoutPort(t *testing.T) {
	got := hostsWithoutPort("a.example, 10.0.0.1:6379,[::1]:6380,,b.example:27017")
	want := []string{"a.example", "10.0.0.1", "::1", "b.example"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The connections using the same certificate file share one watcher, stopped when the last one is released.
func TestNewTLSConfig_HotReloadWatcherIsSharedAndStopped(t *testing.T) {
	ca := newTestCA(t)
	verify := true
	ssl := SSLConfig{On: true, CertFile: ca.certFile, VerifyCertificate: &verify, HotReload: true}

	baseline := runtime.NumGoroutine()

	var releases []func()
	for range 10 {
		_, release, err := newTLSConfig(ssl, []string{"db.example"})
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}

	certWatchersMu.Lock()
	watchers, refs := len(certWatchers), certWatchers[ca.certFile].refs
	certWatchersMu.Unlock()
	if watchers != 1 || refs != 10 {
		t.Fatalf("got %d watchers with %d refs, want 1 watcher with 10 refs", watchers, refs)
	}
	if got := runtime.NumGoroutine() - baseline; got > 3 {
		t.Fatalf("%d goroutines started for 10 configs, want one watcher", got)
	}

	for _, release := range releases {
		release()
		release() // idempotent
	}

	certWatchersMu.Lock()
	watchers = len(certWatchers)
	certWatchersMu.Unlock()
	if watchers != 0 {
		t.Fatalf("got %d watchers after release, want 0", watchers)
	}
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline {
		t.Fatalf("%d goroutines left after release, baseline %d", got, baseline)
	}
}
