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
	"os"
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

func newTLSConfig(ssl SSLConfig) (*tls.Config, error) {
	if !ssl.On {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: !ssl.ShouldVerifyCertificate(),
	}

	if !ssl.ShouldVerifyCertificate() {
		return tlsConfig, nil
	}

	rootCAs, err := loadRootCAs(ssl.CertFile)
	if err != nil {
		return nil, err
	}

	if ssl.HotReload && ssl.CertFile != "" {
		tlsConfig.InsecureSkipVerify = true

		cm := &certManager{}
		cm.rootCAs.Store(rootCAs)

		tlsConfig.VerifyConnection = func(cs tls.ConnectionState) error {
			pool, _ := cm.rootCAs.Load().(*x509.CertPool)
			opts := x509.VerifyOptions{
				Roots:         pool,
				Intermediates: x509.NewCertPool(),
				DNSName:       cs.ServerName,
			}

			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("tls: no certificates from peer")
			}

			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}

			_, err := cs.PeerCertificates[0].Verify(opts)
			if err != nil {
				if newPool, loadErr := loadRootCAs(ssl.CertFile); loadErr == nil {
					cm.rootCAs.Store(newPool)
					opts.Roots = newPool
					_, err = cs.PeerCertificates[0].Verify(opts)
				}
			}

			return err
		}

		watchCertFile(ssl.CertFile, cm)
	} else {
		tlsConfig.RootCAs = rootCAs
	}

	return tlsConfig, nil
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

func watchCertFile(certFile string, cm *certManager) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	if err := watcher.Add(certFile); err != nil {
		_ = watcher.Close()
		return
	}

	go func() {
		//nolint:errcheck
		defer watcher.Close()
		var timer *time.Timer
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
}

func (ssl SSLConfig) ShouldVerifyCertificate() bool {
	if ssl.VerifyCertificate == nil {
		return false
	}

	return *ssl.VerifyCertificate
}
