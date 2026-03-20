# AGENTS.md - EasyDNS Development Guide

## Project Overview

EasyDNS is a high-performance DNS proxy server written in Go, supporting domain-based DNS splitting, caching, local hosts, and multiple DNS protocols (UDP, TCP, TLS) with SOCKS5 proxy forwarding.

**Module**: `easydns`
**Go Version**: 1.23.0+
**Architecture**: cmd/ (main), internal/ (cache, config, dns), pkg/ (util)

---

## Build Commands

### Build the project
```bash
go build -o easydns ./cmd/easydns
```

### Run locally (development)
```bash
go run ./cmd/easydns -c config.yaml
```

### Run with verbose output
```bash
go run ./cmd/easydns -c .config.yaml
```

### Build for release (uses goreleaser)
```bash
goreleaser build --snapshot --clean
```

---

## Testing

**Note**: This project currently has no test files. Tests should be added before making significant changes.

### Run all tests
```bash
go test ./...
```

### Run tests with verbose output
```bash
go test -v ./...
```

### Run tests for a specific package
```bash
go test -v ./internal/dns
go test -v ./internal/cache
go test -v ./internal/config
```

### Run tests with coverage
```bash
go test -cover ./...
```

### Run a single test function
```bash
go test -v -run TestFunctionName ./internal/dns
```

---

## Linting & Code Quality

### Format code (run before committing)
```bash
gofmt -w .
```

### Vet all packages
```bash
go vet ./...
```

### Tidy dependencies
```bash
go mod tidy
```

### Check for dependencies updates
```bash
go list -u -m all
```

---

## Code Style Guidelines

### General Principles

1. **Run `gofmt`** before every commit to ensure consistent formatting
2. **Run `go vet`** to catch common errors before submitting
3. Keep code simple and readable; avoid premature optimization
4. Use meaningful variable and function names in English

### Imports

Import order (three groups with blank lines between):
1. Go standard library (`fmt`, `net`, `context`, etc.)
2. External packages (`github.com/sirupsen/logrus`, `github.com/miekg/dns`)
3. Internal packages (`easydns/internal/config`, `easydns/pkg/util`)

```go
import (
    "context"
    "fmt"
    "net"
    "time"

    "github.com/miekg/dns"
    "github.com/sirupsen/logrus"

    "easydns/internal/config"
    "easydns/pkg/util"
)
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Packages | lowercase, short | `cache`, `dns`, `util` |
| Variables | camelCase | `cacheID`, `upstreamServers` |
| Constants | camelCase or UPPER_SNAKE | `minCacheDuration`, `DNSQueryTimeout` |
| Functions | PascalCase (exported), camelCase (unexported) | `NewHandler`, `parseDNSServer` |
| Types/Structs | PascalCase | `Config`, `DNSCache`, `Handler` |
| Interfaces | PascalCase, often `er` suffix | - |
| Acronyms | Same case | `IP`, `UDP`, `TLS` (not `Ip`, `Udp`) |

### Error Handling

- Use `fmt.Errorf("context: %w", err)` for wrapped errors
- Return errors early; avoid nesting
- Log errors with context using logrus:

```go
// Good
logrus.WithFields(logrus.Fields{
    "server": srv,
    "error":  err,
    "domain": requestedDomain,
}).Debug("DNS query failed")

// At call site
if err != nil {
    return nil, fmt.Errorf("failed to parse server %s: %w", server, err)
}
```

### Logging (logrus)

- Use `logrus.WithFields()` for structured logging with context
- Log levels: `Debug` (verbose), `Info` (important events), `Warn` (recoverable issues), `Error` (failures)
- Never log sensitive data (proxies, passwords)

```go
logrus.WithFields(logrus.Fields{
    "clientIP":        clientIP,
    "requestedDomain": requestedDomain,
    "requestType":     requestType,
}).Info("query success by cache")
```

### Documentation

- Comment exported functions with descriptions in Chinese (project convention)
- Use standard Go doc comments for public APIs
- Document complex logic with inline comments

```go
// HandleDNSRequest 处理DNS请求入口
func (h *Handler) HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    // ...
}
```

### Concurrency

- Use `sync.RWMutex` for read-heavy workloads
- Always use `context.Context` for cancellation and timeouts
- Ensure goroutines are properly cleaned up

```go
ctx, cancel := context.WithTimeout(context.Background(), dnsQueryTimeout)
defer cancel()
```

### Project Structure

```
cmd/easydns/          # Main application entry point
internal/
  cache/              # DNS cache implementation (LRU)
  config/             # Configuration loading and validation
  dns/                # DNS request handling, forwarding, protocols
pkg/util/             # Shared utility functions
scripts/              # Helper scripts
```

### Configuration (YAML)

- Use snake_case for YAML keys
- Provide sensible defaults in code, not just config files
- Validate configuration early at startup

---

## Commit Guidelines

1. Run `gofmt -w .` and `go vet ./...` before committing
2. Commit message format: `type: short description`
   - `feat:`, `fix:`, `refactor:`, `docs:`, `test:`
3. Keep commits focused and atomic
