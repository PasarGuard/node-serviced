package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMaintenanceProcess(t *testing.T) {
	if os.Getenv("NODE_SERVICE_TEST_PROCESS") == "1" {
		main()
	}
}

func TestMaintenanceTransports(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("TLS=%v", enabled), func(t *testing.T) {
			dir := t.TempDir()
			certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
			pool := x509.NewCertPool()
			if enabled {
				key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
				der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
				if err != nil {
					t.Fatal(err)
				}
				certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
				keyDER, err := x509.MarshalECPrivateKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
					t.Fatal(err)
				}
				pool.AppendCertsFromPEM(certPEM)
			}
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := l.Addr().String()
			l.Close()
			_, port, _ := net.SplitHostPort(addr)
			envPath := filepath.Join(dir, ".env")
			data := fmt.Sprintf("API_KEY=test-key\nAPI_PORT=%s\nNODE_HOST=127.0.0.1\nTLS_ENABLED=%v\nSSL_CERT_FILE=%s\nSSL_KEY_FILE=%s\n", port, enabled, certPath, keyPath)
			if err := os.WriteFile(envPath, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(exe, "-test.run=^TestMaintenanceProcess$")
			cmd.Env = append(os.Environ(), "NODE_SERVICE_TEST_PROCESS=1", "ENV_FILE="+envPath)
			logfile, err := os.Create(filepath.Join(dir, "service.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer logfile.Close()
			cmd.Stdout, cmd.Stderr = logfile, logfile
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
			scheme := "http"
			if enabled {
				scheme = "https"
			}
			client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
			defer client.CloseIdleConnections()
			url := scheme + "://" + addr + "/"
			deadline := time.Now().Add(5 * time.Second)
			for {
				resp, err := client.Get(url)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode != 401 {
						t.Fatalf("missing API key: %d", resp.StatusCode)
					}
					break
				}
				if time.Now().After(deadline) {
					data, _ := os.ReadFile(logfile.Name())
					t.Fatalf("startup: %v\n%s", err, data)
				}
				time.Sleep(20 * time.Millisecond)
			}
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("x-api-key", "test-key")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("authenticated maintenance: %d", resp.StatusCode)
			}
		})
	}
}
