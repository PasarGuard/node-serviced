package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultAppName   = "pg-node"
	defaultAPIPort   = "3000"
	defaultEnvFile   = "/opt/pg-node/.env"
	localEnvFileName = ".env"
	maxBodyBytes     = int64(1 << 20) // 1MB
)

type config struct {
	TLSEnabled bool
	NodeHost   string
	AppName    string
	APIPort    string
	APIKey     string
	SSLCert    string
	SSLKey     string
	EnvFile    string
}

type certInfo struct {
	SelfSigned bool
	NotAfter   time.Time
}

func determineEnvFile() (string, bool) {
	if envOverride := os.Getenv("ENV_FILE"); envOverride != "" {
		return envOverride, false
	}

	envFile := defaultEnvFile

	if exePath, err := os.Executable(); err == nil {
		scriptDir := filepath.Dir(exePath)
		localEnv := filepath.Join(scriptDir, localEnvFileName)
		if _, err := os.Stat(envFile); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat(localEnv); err == nil {
				envFile = localEnv
			}
		}
	}

	return envFile, true
}

func loadEnv(path string) error {
	return godotenv.Overload(path)
}

// LoadTLSCredentials loads server cert/key into a tls.Config.
func LoadTLSCredentials(certPath, keyPath string) (*tls.Config, error) {
	holder, err := NewCertificateHolder(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return holder.TLSConfig(), nil
}

func checkCertificate(certPath string) (*certInfo, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read cert: %w", err)
	}

	if block, _ := pem.Decode(data); block != nil {
		data = block.Bytes
	}

	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("parse cert: %w", err)
	}

	selfSigned := false
	if cert.CheckSignatureFrom(cert) == nil && cert.Subject.String() == cert.Issuer.String() {
		selfSigned = true
	}

	return &certInfo{
		SelfSigned: selfSigned,
		NotAfter:   cert.NotAfter,
	}, nil
}

func loadConfig() (config, error) {
	cfg := config{
		AppName:    defaultAppName,
		TLSEnabled: true,
		APIPort:    defaultAPIPort,
	}

	envFile, fallbackUsed := determineEnvFile()
	cfg.EnvFile = envFile

	if _, err := os.Stat(envFile); err == nil {
		if err := loadEnv(envFile); err != nil {
			return cfg, fmt.Errorf("load env file %s: %w", envFile, err)
		}
		logf("Loaded env file: %s", envFile)
	} else {
		if fallbackUsed {
			logf("Env file not found, using defaults: %s", envFile)
		} else {
			logf("Env file not found: %s", envFile)
		}
	}

	cfg.APIKey = os.Getenv("API_KEY")
	cfg.SSLCert = os.Getenv("SSL_CERT_FILE")
	cfg.SSLKey = os.Getenv("SSL_KEY_FILE")
	if port := os.Getenv("API_PORT"); port != "" {
		cfg.APIPort = port
	}
	if app := os.Getenv("APP_NAME"); app != "" {
		cfg.AppName = app
	}

	if cfg.APIKey == "" {
		return cfg, errors.New("API_KEY must be set in the env file")
	}
	if value, exists := os.LookupEnv("TLS_ENABLED"); exists {
		var err error
		cfg.TLSEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("invalid TLS_ENABLED %q: expected a boolean", value)
		}
	}
	cfg.NodeHost = "0.0.0.0"
	if !cfg.TLSEnabled {
		cfg.NodeHost = "127.0.0.1"
	}
	if host, exists := os.LookupEnv("NODE_HOST"); exists {
		cfg.NodeHost = strings.TrimSpace(host)
	}
	addr, err := netip.ParseAddr(cfg.NodeHost)
	if !cfg.TLSEnabled {
		if err != nil || addr.Zone() != "" {
			return cfg, fmt.Errorf("invalid private NODE_HOST %q", cfg.NodeHost)
		}
		addr = addr.Unmap()
		if !addr.IsLoopback() && !addr.IsPrivate() {
			return cfg, fmt.Errorf("TLS_ENABLED=false requires a private NODE_HOST, got %q", cfg.NodeHost)
		}
		cfg.NodeHost = addr.String()
		return cfg, nil
	}
	if strings.EqualFold(cfg.NodeHost, "localhost") {
		cfg.NodeHost = "127.0.0.1"
	} else if err != nil {
		logf("Invalid NODE_HOST %q; using 127.0.0.1", cfg.NodeHost)
		cfg.NodeHost = "127.0.0.1"
	}
	if cfg.SSLCert == "" || cfg.SSLKey == "" {
		return cfg, errors.New("TLS required: set SSL_CERT_FILE and SSL_KEY_FILE in the env file")
	}
	if _, err := os.Stat(cfg.SSLCert); err != nil {
		return cfg, fmt.Errorf("cannot read SSL_CERT_FILE: %s", cfg.SSLCert)
	}
	if _, err := os.Stat(cfg.SSLKey); err != nil {
		return cfg, fmt.Errorf("cannot read SSL_KEY_FILE: %s", cfg.SSLKey)
	}

	return cfg, nil
}
