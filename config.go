package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	AppName string
	APIPort string
	APIKey  string
	SSLCert string
	SSLKey  string
	EnvFile string
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
	serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12, // reject SSLv3 / TLS 1.0 / TLS 1.1
	}, nil
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
		AppName: defaultAppName,
		APIPort: defaultAPIPort,
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
