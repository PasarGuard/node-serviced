package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		logf("%v", err)
		os.Exit(1)
	}

	var tlsConfig *tls.Config
	var certificates *CertificateHolder
	if cfg.TLSEnabled {
		certificates, err = NewCertificateHolder(cfg.SSLCert, cfg.SSLKey)
		if err != nil {
			logf("failed to load TLS credentials: %v", err)
			os.Exit(1)
		}

		tlsConfig = certificates.TLSConfig()
		logf("TLS active %s", certificates.Status())

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

	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGHUP)
	defer signal.Stop(reload)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-reload:
				certificates.LogReload(logf)
			case <-done:
				return
			}
		}
	}()
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
