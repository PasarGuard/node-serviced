# node-serviced

Minimal HTTPS control API rewritten from the original bash script. Uses only the Go standard library plus `github.com/joho/godotenv` for loading `.env`.

## Requirements
- Go 1.25.5
- TLS files: `SSL_CERT_FILE`, `SSL_KEY_FILE`
- Environment: `API_KEY` (required), optional `API_PORT` (default 3000), `APP_NAME` (default `pg-node`), `ENV_FILE` override

## Setup
1. Place your `.env` at `/opt/pg-node/.env` or alongside the binary (`.env`); set `ENV_FILE` to override.
2. Populate at least:
   ```
   API_KEY=...
   SSL_CERT_FILE=/path/to/cert.pem
   SSL_KEY_FILE=/path/to/key.pem
   ```
3. Install deps and verify:
   ```
   go mod tidy
   go run .
   ```

## Endpoints (all require `x-api-key`)
- `GET /` → `{"status":"ok"}`
- `POST /node/update` → runs `pg-node update --no-update-service`
- `POST /node/core_update` with JSON `{"core_version":"<ver>"}` → runs `pg-node core-update --version <ver>`. `<ver>` is `latest` (newest Xray-core release) or `vX.Y.Z`; a bare `X.Y.Z` is sent as `vX.Y.Z`
- `POST /node/geofiles` with JSON `{"region":"iran|russia|china"}` → runs `pg-node geofiles --<region>`
- `POST /node/hard_reset` → runs `pg-node restart` (60 s timeout)


## TLS and private backends

`TLS_ENABLED` defaults to `true` and invalid boolean values fail startup.
Both node services use `NODE_HOST`, `SSL_CERT_FILE`, and `SSL_KEY_FILE` from the
same environment file. A missing `NODE_HOST` binds to `0.0.0.0` with TLS.

For an external TLS web server, set `TLS_ENABLED=false`; certificates are then
unnecessary and the default bind address is `127.0.0.1`. Explicit bindings must
be literal IPv4 loopback, `::1`, RFC1918 IPv4, or IPv6 unique-local addresses.
IPv4-mapped IPv6 addresses are normalized. Public, wildcard, link-local,
multicast, scoped, and malformed addresses fail startup. API-key authentication
remains required. The panel always connects to the web server over HTTPS.

Use the same frontend certificate on the node and maintenance ports when pinned.
Do not expose private backend ports through firewall/NAT/container publishing.
See the node repository's `docs/node-tls.md` and `examples/nginx/` for deployment
examples, renewal behavior and rollback instructions.

## Certificate hot reload

At startup, the certificate/key pair and supplied certificate dates must be valid.
After deploying a consistent replacement at the configured paths, send `SIGHUP`
to the node-serviced main process. Reload attempts are serialized and switch the
in-memory certificate atomically. Invalid replacements retain the active pair.
Startup and reload logs include the active SHA-256 fingerprint and expiry.

Reloads preserve listeners and established connections, including after the old
certificate expires. Fresh connections receive the replacement and validate it
under the client's trust policy. With `TLS_ENABLED=false`, SIGHUP logs a no-op;
the external TLS server remains responsible for its own certificate.

This adds no renewal scheduler or automatic self-signed regeneration. Installation
of hot-reload support requires one initial restart.
