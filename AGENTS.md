# geons AI Agent Guide

## What this repository does
- `geons` is a lightweight Go DNS server that returns geolocation information in TXT records.
- It resolves queries of the form `<ip><domain_suffix>` using MaxMind GeoLite2/GeoIP2 databases (Country, City, ASN).
- Supports **multi-zone configuration**: multiple domain suffixes (`.geons`, `.geocity`, `.asn`) each pointing to different databases.
- The server is configured through `config.yaml`; the main implementation is in `main.go`.
- Uses geoip2-golang/v2 for database access with modern Go types (netip.Addr, typed Names fields)

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
- Config is loaded from `config.yaml` in the current working directory (or `$GEONS_CONFIG` in containers).
- Multiple zones support: `.geons` (Country), `.geocity` (City), `.asn` (ASN) each with independent databases and field configurations.
- `SIGHUP` reloads config and all zone databases atomically.
- `SIGINT` / `SIGTERM` perform graceful shutdown.
- Only DNS TXT queries are handled; other query types return `NOTIMP`.
- Requests are allowed only if the client IP matches `server.allowed_clients` CIDR ranges.
- Domain names are parsed by stripping the zone's `name` (domain suffix); invalid IPs and suffix mismatches return `NXDOMAIN`.
- GeoIP field extraction uses reflection with paths like `Country.ISOCode`, `Country.Names.English`, `Subdivisions[0].Names.English`, `Location.Latitude` (geoip2 v2 API).
- Supports array indexing: `Subdivisions[0]`, `Subdivisions[1]` (returns empty string if out of bounds).
- Bind address defaults to `127.0.0.1` (configurable via `server.bind_address`).

## Code conventions and expectations
- Keep the code minimal and dependency-light.
- `sync.RWMutex` protects config and database access during reloads and query handling.
- Avoid introducing global mutable state beyond the existing guarded config/database structures.
- Keep README configuration examples consistent with actual YAML field names.

## Notes for agents
- If making changes to runtime behavior, verify both normal DNS TXT responses and ACL/failure cases.
- If adding new features, add Go tests in `_test.go` files and keep `.github/workflows/test.yml` in sync.
- When editing docs, link to `README.md` rather than duplicating user-facing configuration details.
