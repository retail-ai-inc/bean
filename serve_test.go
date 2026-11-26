package bean

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
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/retail-ai-inc/bean/v3/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServeTestBean() *Bean {
	b := &Bean{Echo: echo.New(), Config: config.Config{}, Validate: validator.New()}
	b.Echo.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "Pong") })
	return b
}

func serveInBackground(b *Bean, host, port string) <-chan error {
	errCh := make(chan error, 1)
	ready := make(chan struct{})
	b.BeforeServe = func() { close(ready) }
	go func() { errCh <- b.ServeAt(host, port) }()
	select { // `Shutdown` may be called once `BeforeServe` ran.
	case <-ready:
	case <-time.After(5 * time.Second):
	}
	return errCh
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestBean_ServeAt_StartupError(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	select {
	case err := <-serveInBackground(newServeTestBean(), "localhost", port):
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error during server startup")
	case <-time.After(5 * time.Second):
		t.Fatal("ServeAt did not return on startup error")
	}
}

func TestBean_ServeAt_ShutdownByCaller(t *testing.T) {
	b := newServeTestBean()
	host, port := "localhost", strconv.Itoa(getFreePort(t))
	errCh := serveInBackground(b, host, port)

	waitUntil(t, func() bool {
		resp, err := http.Get("http://" + host + ":" + port)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	require.NoError(t, b.Shutdown())
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ServeAt did not return after Shutdown")
	}
}

func TestBean_ServeAt_TLS(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t) // absolute paths
	b := newServeTestBean()
	b.Config.HTTP.SSL.On = true
	b.Config.HTTP.SSL.CertFile = certFile
	b.Config.HTTP.SSL.PrivFile = keyFile
	b.Config.HTTP.SSL.MinTLSVersion = tls.VersionTLS12

	host, port := "localhost", strconv.Itoa(getFreePort(t))
	errCh := serveInBackground(b, host, port)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed test certificate
		ForceAttemptHTTP2: true,
	}}
	var proto int
	waitUntil(t, func() bool {
		resp, err := client.Get("https://" + host + ":" + port)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		proto = resp.ProtoMajor
		return resp.StatusCode == http.StatusOK
	})
	assert.Equal(t, 2, proto, "HTTP/2 should be negotiated over TLS")

	require.NoError(t, b.Shutdown())
	assert.NoError(t, <-errCh)
}

func TestBean_ServeAt_TLSMissingCert(t *testing.T) {
	b := newServeTestBean()
	b.Config.HTTP.SSL.On = true
	b.Config.HTTP.SSL.CertFile = filepath.Join(t.TempDir(), "missing.pem")

	err := b.ServeAt("localhost", strconv.Itoa(getFreePort(t)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error during server startup")
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certFile, keyFile
}
