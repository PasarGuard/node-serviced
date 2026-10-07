package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type server struct {
	cfg config
}

var coreVersionRegexp = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)
var geofileRegionRegexp = regexp.MustCompile(`^(iran|russia|china)$`)

type coreUpdateRequest struct {
	CoreVersion string `json:"core_version"`
}

func (r *coreUpdateRequest) validate() error {
	if r.CoreVersion == "" {
		return fmt.Errorf("core_version is required")
	}
	// pg-node core-update understands "latest" (newest Xray release) and looks versions up by their
	// release tag, which always has a "v" prefix; the panel accepts both "latest" and "X.Y.Z".
	if r.CoreVersion == "latest" {
		return nil
	}
	if !coreVersionRegexp.MatchString(r.CoreVersion) {
		return fmt.Errorf("core_version must be \"latest\" or match pattern vX.X.X")
	}
	if !strings.HasPrefix(r.CoreVersion, "v") {
		r.CoreVersion = "v" + r.CoreVersion
	}
	return nil
}

type geofilesRequest struct {
	Region string `json:"region"`
}

func (r *geofilesRequest) validate() (string, error) {
	if r.Region == "" {
		return "", fmt.Errorf("region is required (iran, russia, china)")
	}
	region := strings.ToLower(r.Region)
	if !geofileRegionRegexp.MatchString(region) {
		return "", fmt.Errorf("Unsupported region %s", r.Region)
	}
	return region, nil
}

func (s *server) respond(w http.ResponseWriter, code int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func (s *server) authenticate(w http.ResponseWriter, r *http.Request) bool {
	key := r.Header.Get("x-api-key")
	if key == "" {
		s.respond(w, http.StatusUnauthorized, map[string]string{"detail": "missing api key"})
		logf("Unauthorized: missing x-api-key for %s %s", r.Method, r.URL.Path)
		return false
	}
	if key != s.cfg.APIKey {
		s.respond(w, http.StatusUnauthorized, map[string]string{"detail": "invalid api key"})
		logf("Unauthorized: invalid x-api-key for %s %s", r.Method, r.URL.Path)
		return false
	}
	return true
}

func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}
	s.respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}
	// Deliberately not derived from r.Context(): the panel's HTTP client uses a
	// short request timeout and will disconnect long before an update finishes
	// (self-update -> docker pull -> container recreate can take minutes). If
	// this context were tied to the request, that disconnect would cancel it
	// and SIGKILL the in-progress update, potentially leaving the node with no
	// container at all. Let the update run to completion regardless of whether
	// the caller is still listening for the response.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if stderr, code, err := runCommand(ctx, s.cfg.AppName, "update", "--no-update-service"); err != nil {
		logf("update failed with exit code %d: %v", code, err)
		if stderr != "" {
			logf("stderr: %s", stderr)
		}
		s.respond(w, http.StatusInternalServerError, map[string]string{"detail": "update failed on server"})
		return
	}

	s.respond(w, http.StatusOK, map[string]string{"detail": "node updated successfully"})
}

func (s *server) handleCoreUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	var payload coreUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.respond(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON body"})
		return
	}
	if err := payload.validate(); err != nil {
		s.respond(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	// Not derived from r.Context(); see handleUpdate for why.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	args := []string{"core-update", "--version", payload.CoreVersion}
	stderr, code, err := runCommand(ctx, s.cfg.AppName, args...)
	if err != nil {
		logf("core-update failed for version %s (exit code: %d): %v", payload.CoreVersion, code, err)
		if stderr != "" {
			logf("Error output: %s", stderr)
		}
		cleanErr := cleanANSI(stderr)
		if cleanErr != "" {
			s.respond(w, http.StatusNotFound, map[string]string{
				"detail": fmt.Sprintf("core-update failed for version %s: %s", payload.CoreVersion, cleanErr),
			})
		} else {
			s.respond(w, http.StatusInternalServerError, map[string]string{
				"detail": fmt.Sprintf("core-update failed for version %s: %v", payload.CoreVersion, err),
			})
		}
		return
	}

	s.respond(w, http.StatusOK, map[string]string{"detail": "node core updated successfully"})
}

func (s *server) handleGeofiles(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	var payload geofilesRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.respond(w, http.StatusBadRequest, map[string]string{"detail": "Invalid JSON body"})
		return
	}
	region, err := payload.validate()
	if err != nil {
		s.respond(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	flag := "--" + region

	// Not derived from r.Context(); see handleUpdate for why.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if stderr, code, err := runCommand(ctx, s.cfg.AppName, "geofiles", flag); err != nil {
		logf("geofiles update failed (exit code: %d): %v", code, err)
		if stderr != "" {
			logf("stderr: %s", stderr)
		}
		s.respond(w, http.StatusInternalServerError, map[string]string{"detail": "geofiles update failed on server"})
		return
	}

	s.respond(w, http.StatusOK, map[string]string{"detail": "geofiles updated successfully"})
}

func (s *server) handleHardReset(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}

	logf("hard reset requested: restarting node %s", s.cfg.AppName)

	// Not derived from r.Context(); see handleUpdate for why.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if stderr, code, err := runCommand(ctx, s.cfg.AppName, "restart"); err != nil {
		logf("hard reset failed for %s (exit code: %d): %v", s.cfg.AppName, code, err)
		if stderr != "" {
			logf("stderr: %s", stderr)
		}
		detail := fmt.Sprintf("hard reset failed for %s: %v", s.cfg.AppName, err)
		if cleaned := cleanANSI(stderr); cleaned != "" {
			detail = fmt.Sprintf("hard reset failed for %s: %s", s.cfg.AppName, cleaned)
		}
		s.respond(w, http.StatusInternalServerError, map[string]string{"detail": detail})
		return
	}

	logf("hard reset succeeded: %s restarted", s.cfg.AppName)
	s.respond(w, http.StatusOK, map[string]string{"detail": "node restarted successfully"})
}
