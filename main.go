package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		logf("%v", err)
		os.Exit(1)
	}

	var tlsConfig *tls.Config
	if cfg.TLSEnabled {
		tlsConfig, err = LoadTLSCredentials(cfg.SSLCert, cfg.SSLKey)
		if err != nil {
			logf("failed to load TLS credentials: %v", err)
			os.Exit(1)
		}

		info, err := checkCertificate(cfg.SSLCert)
		if err != nil {
			logf("Certificate validation failed for %s: %v", cfg.SSLCert, err)
		} else {
			switch {
			case info.SelfSigned:
				logf("Certificate is self-signed: %s", cfg.SSLCert)
			default:
				logf("Certificate appears to be CA-signed: %s", cfg.SSLCert)
			}

			if !info.NotAfter.IsZero() {
				now := time.Now()
				if now.After(info.NotAfter) {
					logf("Warning: Certificate has expired: %s", cfg.SSLCert)
					logf("Expiration: %s", info.NotAfter.Format(time.RFC3339))
				} else {
					days := int(info.NotAfter.Sub(now).Hours() / 24)
					logf("Certificate valid for %d more days (expires: %s)", days, info.NotAfter.Format(time.RFC3339))
				}
			}
		}

		logf("TLS enabled on port %s with cert=%s key=%s", cfg.APIPort, cfg.SSLCert, cfg.SSLKey)
	} else {
		logf("External TLS backend on %s:%s", cfg.NodeHost, cfg.APIPort)
	}
	logf("API key protection enabled")
	logf("node app (hard reset target): %s", cfg.AppName)

	s := &server{cfg: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("POST /node/update", s.handleUpdate)
	mux.HandleFunc("POST /node/core_update", s.handleCoreUpdate)
	mux.HandleFunc("POST /node/geofiles", s.handleGeofiles)
	mux.HandleFunc("POST /node/hard_reset", s.handleHardReset)

	httpServer := &http.Server{
		Addr:              net.JoinHostPort(cfg.NodeHost, cfg.APIPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		TLSConfig:         tlsConfig,
	}

	var serveErr error
	if cfg.TLSEnabled {
		serveErr = httpServer.ListenAndServeTLS("", "")
	} else {
		serveErr = httpServer.ListenAndServe()
	}
	if err := serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logf("server error: %v", err)
		os.Exit(1)
	}
}
