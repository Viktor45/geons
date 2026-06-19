# geons AI Agent Guide

## What this repository does
- `geons` is a lightweight Go DNS server that returns geolocation information in TXT records.
- It resolves queries of the form `<ip><domain_suffix>` using a MaxMind GeoLite2 Country database.
- The server is configured through `config.yaml`; the main implementation is in `main.go`.

## Key files
- `main.go` — application entrypoint, DNS request handling, config loading, hot reload, graceful shutdown.
- `config.yaml` — runtime configuration for server port, domain suffix, ACLs, database path, and TXT response fields.
- `README.md` — user-facing usage, config examples, and architecture overview.
- `Dockerfile` — multistage build producing a static Go binary and minimal runtime image.
- `.github/workflows/test.yml` — CI run commands for tests, vet, and build.

## Build and test commands
- `go mod download`
- `go run main.go`
- `go test ./...`
- `go vet ./...`
- `go build ./geons`
- Docker build is defined by `Dockerfile` and compiles the binary into `/out/geons`.

## Runtime behavior to preserve
- Config is loaded from `config.yaml` in the current working directory.
- `SIGHUP` reloads config and database atomically.
- `SIGINT` / `SIGTERM` perform graceful shutdown.
- Only DNS TXT queries are handled; other query types return `NOTIMP`.
- Requests are allowed only if the client IP matches `server.allowed_cidrs`.
- Domain names are parsed by stripping `server.domain_suffix`; invalid IPs and suffix mismatches return `NXDOMAIN`.
- GeoIP field extraction uses reflection with paths like `Country.IsoCode`, `Country.Names.en`, or `Continent.Code`.

## Code conventions and expectations
- Keep the code minimal and dependency-light.
- `sync.RWMutex` protects config and database access during reloads and query handling.
- Avoid introducing global mutable state beyond the existing guarded config/database structures.
- Keep README configuration examples consistent with actual YAML field names.

## Notes for agents
- If making changes to runtime behavior, verify both normal DNS TXT responses and ACL/failure cases.
- If adding new features, add Go tests in `_test.go` files and keep `.github/workflows/test.yml` in sync.
- When editing docs, link to `README.md` rather than duplicating user-facing configuration details.
