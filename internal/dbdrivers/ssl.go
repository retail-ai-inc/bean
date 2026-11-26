// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package dbdrivers

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

type SSLConfig struct {
	On                bool
	CertFile          string
	VerifyCertificate *bool
	HotReload         bool
}

type certManager struct {
	rootCAs atomic.Value
}

func sslConfigFromMap(cfg map[string]interface{}) SSLConfig {
	sslValue, ok := cfg["ssl"]
	if !ok {
		return SSLConfig{}
	}

	sslMap, ok := sslValue.(map[string]interface{})
	if !ok {
		return SSLConfig{}
	}

	ssl := SSLConfig{}
	if on, ok := sslMap["on"].(bool); ok {
		ssl.On = on
	}
	if certFile, ok := sslMap["certFile"].(string); ok {
		ssl.CertFile = certFile
	}
	if verifyCertificate, ok := sslMap["verifyCertificate"].(bool); ok {
		ssl.VerifyCertificate = &verifyCertificate
	}
	if hotReload, ok := sslMap["hotReload"].(bool); ok {
		ssl.HotReload = hotReload
	}

	return ssl
}

func noRelease() {}

// newTLSConfig returns the TLS config of a database connection, nil if SSL is off, and a function releasing its
// resources (the certificate file watcher with `hotReload`) to call when the connection is closed. hosts are the hosts of the
// connection as configured (without port); with `hotReload` they are used to verify the server name, see
// `verifyServerName`.
func newTLSConfig(ssl SSLConfig, hosts []string) (*tls.Config, func(), error) {
	if !ssl.On {
		return nil, noRelease, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: !ssl.ShouldVerifyCertificate(),
	}

	if !ssl.ShouldVerifyCertificate() {
		return tlsConfig, noRelease, nil
	}

	rootCAs, err := loadRootCAs(ssl.CertFile)
	if err != nil {
		return nil, noRelease, err
	}

	if ssl.HotReload && ssl.CertFile != "" {
		tlsConfig.InsecureSkipVerify = true

		// One watcher per certificate file, shared by the connections using it and stopped with the last one.
		cm, release := acquireCertManager(ssl.CertFile, rootCAs)

		// The root CAs can change at runtime, so the certificate is verified here instead of by crypto/tls
		// (InsecureSkipVerify only disables the built-in verification): the chain against the current root CAs,
		// then the server name.
		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("tls: no certificates from peer")
			}

			verify := func(pool *x509.CertPool) error {
				opts := x509.VerifyOptions{
					Roots:         pool,
					Intermediates: x509.NewCertPool(),
				}
				for _, cert := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(cert)
				}
				_, err := cs.PeerCertificates[0].Verify(opts)
				return err
			}

			pool, _ := cm.rootCAs.Load().(*x509.CertPool)
			err := verify(pool)
			if err != nil {
				// The certificate may have been rotated before the watcher reloaded the file.
				if newPool, loadErr := loadRootCAs(ssl.CertFile); loadErr == nil {
					cm.rootCAs.Store(newPool)
					err = verify(newPool)
				}
			}
			if err != nil {
				return err
			}

			return verifyServerName(cs.PeerCertificates[0], cs.ServerName, hosts)
		}

		return tlsConfig, release, nil
	}

	tlsConfig.RootCAs = rootCAs
	return tlsConfig, noRelease, nil
}

func loadRootCAs(certFile string) (*x509.CertPool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool()
	}

	if certFile != "" {
		certs, err := os.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ssl certificate file: %w", err)
		}

		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, fmt.Errorf("failed to parse ssl certificate file")
		}
	}

	return rootCAs, nil
}

// certWatcher is the certificate manager of a certificate file and the watcher reloading it, shared by the
// connections using that file.
type certWatcher struct {
	cm   *certManager
	refs int
	stop func()
}

var (
	certWatchersMu sync.Mutex
	certWatchers   = map[string]*certWatcher{}
)

// acquireCertManager returns the certificate manager of certFile, watched for changes, and the function releasing
// it: the watcher is stopped when the last user releases it.
func acquireCertManager(certFile string, rootCAs *x509.CertPool) (*certManager, func()) {
	certWatchersMu.Lock()
	defer certWatchersMu.Unlock()

	w, ok := certWatchers[certFile]
	if !ok {
		cm := &certManager{}
		cm.rootCAs.Store(rootCAs)
		w = &certWatcher{cm: cm, stop: watchCertFile(certFile, cm)}
		certWatchers[certFile] = w
	}
	w.refs++

	return w.cm, sync.OnceFunc(func() {
		certWatchersMu.Lock()
		defer certWatchersMu.Unlock()
		w.refs--
		if w.refs == 0 {
			delete(certWatchers, certFile)
			w.stop()
		}
	})
}

// watchCertFile reloads the root CAs of cm when certFile changes, until the returned stop function is called.
func watchCertFile(certFile string, cm *certManager) (stop func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return noRelease
	}

	if err := watcher.Add(certFile); err != nil {
		_ = watcher.Close()
		return noRelease
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		var timer *time.Timer
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					if event.Op&fsnotify.Rename == fsnotify.Rename {
						_ = watcher.Remove(event.Name)
						_ = watcher.Add(certFile)
					}

					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(500*time.Millisecond, func() {
						if newPool, err := loadRootCAs(certFile); err == nil {
							cm.rootCAs.Store(newPool)
						}
					})
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return func() {
		_ = watcher.Close() // closes the channels: the goroutine returns
		<-done
	}
}

// verifyServerName checks that the server certificate is valid for the server the client connected to.
//
// serverName is the name sent in the TLS handshake (SNI). It is empty when the client connected to an IP address
// (SNI does not carry IP addresses), so in that case the certificate must be valid for one of the configured hosts
// (an IP address host is matched against the IP SANs). Without any name the certificate cannot be verified, which
// is an error: accepting it would accept any certificate signed by a trusted CA.
//
// Limitation: a server reached by IP address that is not a configured host (e.g. a Redis cluster node discovered
// at runtime) must present a certificate valid for one of the configured hosts.
func verifyServerName(cert *x509.Certificate, serverName string, hosts []string) error {
	if serverName != "" {
		return cert.VerifyHostname(serverName)
	}

	var firstErr error
	for _, host := range hosts {
		if host == "" {
			continue
		}
		err := cert.VerifyHostname(host)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		return fmt.Errorf("tls: cannot verify the server certificate: no server name")
	}
	return firstErr
}

// hostsWithoutPort returns the hosts of a comma separated `host` (or `host:port`) list, without the ports.
func hostsWithoutPort(hostList string) []string {
	var hosts []string
	for _, h := range strings.Split(hostList, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		}
		hosts = append(hosts, strings.Trim(h, "[]"))
	}
	return hosts
}

func (ssl SSLConfig) ShouldVerifyCertificate() bool {
	if ssl.VerifyCertificate == nil {
		return false
	}

	return *ssl.VerifyCertificate
}
