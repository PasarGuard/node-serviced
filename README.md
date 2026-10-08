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

