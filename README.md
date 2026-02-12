# iCal Proxy Server

A Go-based proxy server for iCalendar (.ics) files that automatically validates, fixes, and filters calendar events to ensure [RFC 5545](https://www.rfc-editor.org/rfc/rfc5545) compliance. Feed it any iCal URL and get back a standards-compliant calendar -- malformed properties are corrected, missing required fields are added, and events can be filtered by date range.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
  - [Docker Compose](#docker-compose)
  - [Docker](#docker)
  - [Pre-built Binaries](#pre-built-binaries)
  - [Building from Source](#building-from-source)
- [API Reference](#api-reference)
  - [GET /proxy](#get-proxy)
  - [GET /health](#get-health)
- [RFC 5545 Compliance Fixes](#rfc-5545-compliance-fixes)
  - [Calendar-Level Fixes](#calendar-level-fixes)
  - [Event-Level Fixes](#event-level-fixes)
  - [Alarm Fixes](#alarm-fixes)
  - [TODO Fixes](#todo-fixes)
  - [Post-Serialization Fixes](#post-serialization-fixes)
- [Configuration](#configuration)
- [Development](#development)
  - [Prerequisites](#prerequisites)
  - [Project Structure](#project-structure)
  - [Building](#building)
  - [Testing](#testing)
  - [Linting](#linting)
- [Deployment](#deployment)
  - [Docker Image](#docker-image)
  - [Kubernetes](#kubernetes)
- [CI/CD](#cicd)
- [Security](#security)
- [License](#license)

## Features

- **iCal Proxying** -- Fetches iCalendar feeds from remote URLs and serves them through a single endpoint.
- **RFC 5545 Auto-Repair** -- Detects and fixes common compliance issues in malformed calendar data, including missing required properties, invalid values, and incorrect date-time formats.
- **Date Range Filtering** -- Optionally filters events to a specified date window using `from` and `to` query parameters.
- **VTODO Support** -- Validates and fixes TODO components in addition to events.
- **Alarm Validation** -- Ensures VALARM components have all required properties for their action type (DISPLAY, EMAIL, AUDIO).
- **TZID Cleanup** -- Removes invalid TZID parameters from UTC date-time values as required by RFC 5545.
- **Health Check Endpoint** -- Built-in `/health` endpoint for load balancer and orchestrator probes.
- **Production-Ready** -- Hardened Docker image running as non-root, configurable timeouts, multi-platform builds, Kubernetes manifests with HPA and network policies.

## Architecture

The server is a single Go binary with no external runtime dependencies. It uses the standard library `net/http` server with the [`golang-ical`](https://github.com/arran4/golang-ical) library for iCalendar parsing and manipulation.

```
Client Request                      Upstream Calendar
     |                                     |
     v                                     |
  /proxy?url=...  ──> Fetch upstream ──────┘
                          |
                          v
                    Parse iCal data
                          |
                          v
                   Filter by date range (optional)
                          |
                          v
                   Apply RFC 5545 fixes
                    - Calendar properties
                    - Event properties
                    - Alarm components
                    - TODO components
                          |
                          v
                   Serialize with CRLF line endings
                          |
                          v
                   Post-serialization fixes (TZID cleanup)
                          |
                          v
                   Return text/calendar response
```

### Source Files

| File | Purpose |
|------|---------|
| `server/main.go` | HTTP server, proxy handler, date filtering, request routing |
| `server/fixing.go` | RFC 5545 compliance fixes for calendars, events, alarms, and TODOs |
| `server/validation.go` | Property value validators for CLASS, STATUS, TRANSP, and ACTION |
| `server/main_test.go` | Test suite covering all endpoints, fixes, and edge cases |

## Getting Started

### Docker Compose

The simplest way to run the server:

```bash
git clone https://github.com/konairius/ical-proxy.git
cd ical-proxy
docker-compose up -d
```

The service is available at `http://localhost:8080`.

### Docker

```bash
# Build the image
docker build -t ical-proxy .

# Run the container
docker run -d -p 8080:8080 --name ical-proxy ical-proxy
```

Or use the pre-built image from GitHub Container Registry:

```bash
docker run -d -p 8080:8080 ghcr.io/konairius/ical-proxy:latest
```

### Pre-built Binaries

Download binaries for your platform from the [Releases](https://github.com/konairius/ical-proxy/releases) page. Available platforms:

| Platform | Binary |
|----------|--------|
| Linux (x86_64) | `ical-proxy-linux-amd64` |
| Linux (ARM64) | `ical-proxy-linux-arm64` |
| macOS (Intel) | `ical-proxy-darwin-amd64` |
| macOS (Apple Silicon) | `ical-proxy-darwin-arm64` |
| Windows (x86_64) | `ical-proxy-windows-amd64.exe` |

Each release includes a `checksums.txt` file with SHA-256 hashes for verification.

### Building from Source

Requires Go 1.24+:

```bash
cd server
go run .
```

Or build a binary:

```bash
CGO_ENABLED=0 go build -ldflags="-w -s" -o ical-proxy ./server
./ical-proxy
```

## API Reference

### GET /proxy

Fetches an iCalendar feed from the specified URL, applies RFC 5545 compliance fixes, and optionally filters events by date range.

**Parameters:**

| Parameter | Required | Format | Description |
|-----------|----------|--------|-------------|
| `url` | Yes | Absolute URL | URL of the iCalendar feed to proxy |
| `from` | No | `YYYY-MM-DD` | Start date for event filtering (inclusive) |
| `to` | No | `YYYY-MM-DD` | End date for event filtering (inclusive) |

**Response:**

- **Content-Type:** `text/calendar`
- **Body:** RFC 5545 compliant iCalendar data with CRLF line endings

**Error Responses:**

| Status | Condition |
|--------|-----------|
| 400 Bad Request | Missing `url` parameter |
| 400 Bad Request | Invalid `url` (not absolute) |
| 400 Bad Request | Invalid `from` or `to` date format |
| 400 Bad Request | Empty or unparseable iCal data from upstream |
| 405 Method Not Allowed | Non-GET request |
| 500 Internal Server Error | Failed to fetch upstream iCal feed |

**Examples:**

```bash
# Proxy a calendar feed
curl "http://localhost:8080/proxy?url=https://example.com/calendar.ics"

# Proxy with date filtering (events in 2025 only)
curl "http://localhost:8080/proxy?url=https://example.com/calendar.ics&from=2025-01-01&to=2025-12-31"

# Filter events from a specific date onwards
curl "http://localhost:8080/proxy?url=https://example.com/calendar.ics&from=2025-06-01"

# Filter events up to a specific date
curl "http://localhost:8080/proxy?url=https://example.com/calendar.ics&to=2025-12-31"
```

**Usage with calendar applications:**

Most calendar applications (Google Calendar, Apple Calendar, Thunderbird, Outlook) support subscribing to a calendar via URL. Use the proxy URL as the subscription URL:

```
http://your-server:8080/proxy?url=https://example.com/calendar.ics
```

### GET /health

Returns the health status of the service.

**Response:**

- **Content-Type:** `application/json`
- **Status:** 200 OK
- **Body:**

```json
{"status":"healthy","service":"ical-proxy"}
```

## RFC 5545 Compliance Fixes

The proxy automatically detects and corrects common issues in iCalendar data. All applied fixes are logged for debugging. The following sections detail every fix the proxy applies.

### Calendar-Level Fixes

| Property | Fix Applied |
|----------|-------------|
| `VERSION` | Set to `2.0` if missing or incorrect |
| `PRODID` | Added as `-//iCal Proxy Server//EN` if missing; existing values are preserved |
| `CALSCALE` | Set to `GREGORIAN` if missing or set to an unsupported value |

### Event-Level Fixes

**Required properties:**

| Property | Fix Applied |
|----------|-------------|
| `UID` | Generated as a cryptographically random 32-character hex string with `@ical-proxy.local` suffix |
| `DTSTAMP` | Set to current UTC time if missing |
| `SUMMARY` | Set to `"Event"` if missing |

**Date-time properties:**

| Property | Fix Applied |
|----------|-------------|
| `DTSTART` | Set to current UTC time if missing; format is normalized (whitespace and separators removed, `Z` suffix added for 15-char values, `T000000Z` appended for date-only values) |
| `DTEND` | Set to `DTSTART + 1 hour` if missing; format is normalized; corrected to `DTSTART + 1 hour` if not after DTSTART |

**Optional properties (added with defaults if missing):**

| Property | Default | Valid Values (RFC 5545) |
|----------|---------|------------------------|
| `CREATED` | Current UTC time | -- |
| `LAST-MODIFIED` | Current UTC time | -- |
| `CLASS` | `PUBLIC` | `PUBLIC`, `PRIVATE`, `CONFIDENTIAL`, `X-*` |
| `STATUS` | `CONFIRMED` | `TENTATIVE`, `CONFIRMED`, `CANCELLED`, `X-*` |
| `TRANSP` | `OPAQUE` | `OPAQUE`, `TRANSPARENT`, `X-*` |

Invalid values for CLASS, STATUS, and TRANSP are replaced with their defaults. Empty STATUS and TRANSP values are also replaced. All validators accept X-name extensions (values starting with `X-`).

### Alarm Fixes

Each VALARM component within an event is validated:

| Property | Fix Applied |
|----------|-------------|
| `ACTION` | Set to `DISPLAY` if missing, empty, or invalid. Valid: `AUDIO`, `DISPLAY`, `EMAIL`, `X-*` |
| `TRIGGER` | Set to `-PT15M` (15 minutes before) if missing |
| `DESCRIPTION` | Copied from parent event's SUMMARY (or `"Event Reminder"`) if missing and ACTION is DISPLAY or EMAIL |
| `SUMMARY` | Copied from parent event's SUMMARY (or `"Event Reminder"`) if missing and ACTION is EMAIL |

### TODO Fixes

| Property | Fix Applied |
|----------|-------------|
| `UID` | Generated (same as events) if missing |
| `DTSTAMP` | Set to current UTC time if missing |
| `SUMMARY` | Set to `"Task"` if missing |

### Post-Serialization Fixes

After the calendar is serialized to text, the following fix is applied:

- **TZID on UTC times** -- Per RFC 5545, the `TZID` parameter must not appear on date-time values specified in UTC (ending with `Z`). The proxy removes `TZID` parameters from `DTSTART` and `DTEND` lines whose values end with `Z`.

## Configuration

The server is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | TCP port the HTTP server listens on |

**Server timeouts** (hardcoded):

| Timeout | Value |
|---------|-------|
| Read timeout | 10 seconds |
| Write timeout | 10 seconds |
| Idle timeout | 15 seconds |
| Max header size | 1 MB |
| Upstream fetch timeout | 30 seconds |

## Development

### Prerequisites

- Go 1.24+
- Docker (for container builds)
- [golangci-lint](https://golangci-lint.run/) (for linting)

### Project Structure

```
ical-proxy/
├── server/                    # Go application source
│   ├── main.go                # HTTP server, proxy handler, date filtering
│   ├── fixing.go              # RFC 5545 compliance fix engine
│   ├── validation.go          # Property value validators
│   ├── main_test.go           # Test suite
│   └── testdata/              # Test fixture files
├── k8s/                       # Kubernetes manifests
│   ├── config/                # Environment-specific configs
│   ├── namespace.yaml
│   ├── rbac.yaml
│   ├── deployment.yaml
│   ├── autoscaling.yaml
│   ├── network-policy.yaml
│   └── kustomization.yaml
├── .github/
│   ├── workflows/
│   │   ├── ci.yml             # CI pipeline
│   │   └── release.yml        # Release and Docker publish
│   └── dependabot.yml         # Automated dependency updates
├── Dockerfile                 # Multi-stage container build
├── docker-compose.yml         # Local development setup
├── go.mod
├── .golangci.yml              # Linter configuration
└── LICENSE                    # MIT License
```

### Building

```bash
# Standard build
go build -o ical-proxy ./server

# Production build (static, stripped)
CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-w -s" -o ical-proxy ./server

# Cross-compile for Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o ical-proxy-linux-arm64 ./server
```

### Testing

```bash
# Run all tests
go test ./server/...

# Run with verbose output
go test -v ./server/...

# Run with race detector and coverage
go test -v -race -coverprofile=coverage.out ./server/...

# View coverage report
go tool cover -html=coverage.out
```

The test suite covers:

- HTTP proxy endpoint behavior (valid requests, missing/invalid parameters, method validation)
- Date range filtering with various combinations of `from` and `to`
- All RFC 5545 compliance fixes (each fix type has dedicated tests)
- Property validation functions (CLASS, STATUS, TRANSP, ACTION)
- DateTime normalization edge cases
- UID generation
- Post-serialization TZID cleanup
- Health endpoint
- Error handling (empty input, malformed data, unreachable upstream)

### Linting

The project uses [golangci-lint](https://golangci-lint.run/) with the following linters enabled:

- **errcheck** -- Unchecked error return values
- **govet** -- Suspicious constructs
- **ineffassign** -- Ineffective assignments
- **staticcheck** -- Static analysis
- **unused** -- Unused code
- **misspell** -- Misspelled words
- **revive** -- Code style
- **gosec** -- Security issues

```bash
# Run linter
golangci-lint run ./...

# Run with staticcheck separately
staticcheck ./...
```

## Deployment

### Docker Image

The Dockerfile uses a multi-stage build:

1. **Builder stage** (`golang:1.24-alpine`) -- Compiles the Go binary with CGO disabled for a fully static binary.
2. **Runtime stage** (`alpine:latest`) -- Minimal image with only CA certificates, a non-root user (`appuser`, UID 1001), and the compiled binary.

The image includes a built-in health check that probes `/health` every 30 seconds.

The image is published to `ghcr.io/konairius/ical-proxy` on every push to `main` and on tagged releases. Multi-platform images are built for `linux/amd64` and `linux/arm64`.

### Kubernetes

Full Kubernetes manifests are provided in the [`k8s/`](k8s/) directory with Kustomize support. See the [Kubernetes deployment guide](k8s/README.md) for detailed instructions.

Key features of the Kubernetes deployment:

- **Namespace isolation** -- Dedicated `ical-proxy` namespace
- **Horizontal Pod Autoscaler** -- Scales from 2 to 10 replicas based on CPU (70%) and memory (80%) utilization
- **Pod Disruption Budget** -- Guarantees at least 1 pod available during voluntary disruptions
- **Network Policies** -- Restricts ingress to traffic from the ingress controller and monitoring namespaces on port 8080; egress is unrestricted (required for fetching upstream calendars)
- **Security Context** -- Non-root user, read-only root filesystem, all capabilities dropped
- **Health Probes** -- Liveness (30s interval) and readiness (10s interval) probes on `/health`
- **Ingress** -- NGINX ingress with TLS termination and SSL redirect
- **Resource Limits** -- 50m/200m CPU and 64Mi/256Mi memory (requests/limits)

Quick deploy:

```bash
# Using Kustomize
kubectl apply -k k8s/

# Or apply manifests directly
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/autoscaling.yaml
kubectl apply -f k8s/network-policy.yaml
```

## CI/CD

The project uses GitHub Actions with two workflows:

### CI Workflow (`.github/workflows/ci.yml`)

Triggers on pushes to `main`/`develop`, pull requests to `main`, and manual dispatch.

| Job | Description |
|-----|-------------|
| **test** | Runs `go vet` and `go test` with race detection and coverage |
| **static-analysis** | Runs `staticcheck` |
| **lint** | Runs `golangci-lint` |
| **build** | Cross-compiles binaries for Linux, Windows, and macOS (depends on test, static-analysis, lint) |
| **docker** | Builds and pushes multi-platform Docker image to GHCR (main branch only) |

### Release Workflow (`.github/workflows/release.yml`)

Triggers on version tags (`v*`).

1. Runs tests
2. Builds binaries for all platforms
3. Creates SHA-256 checksums
4. Builds and pushes versioned Docker image
5. Creates a GitHub Release with binaries and changelog

Tags containing `alpha`, `beta`, or `rc` are marked as pre-releases.

### Dependabot

Automated dependency updates are configured for:

- **Go modules** -- Weekly (Mondays, 09:00 UTC), max 5 open PRs
- **GitHub Actions** -- Weekly, max 5 open PRs
- **Docker base images** -- Weekly, max 3 open PRs

## Security

### Application

- URL parameters are validated (absolute URL required, date format checked)
- HTTP client uses a 30-second timeout for upstream requests
- Server enforces read/write/idle timeouts and a 1 MB max header size
- All property values are validated against RFC 5545 before being accepted

### Container

- Runs as non-root user (`appuser`, UID 1001)
- Alpine-based minimal image reduces attack surface
- CA certificates included for HTTPS upstream connections
- No shell access required at runtime

### Kubernetes

- Read-only root filesystem
- All Linux capabilities dropped
- Dedicated service account with no additional RBAC bindings
- Network policies restrict ingress to authorized namespaces only
- TLS termination at ingress with enforced SSL redirect

## License

MIT License. Copyright (c) 2025 Konstantin Renner. See [LICENSE](LICENSE) for details.
