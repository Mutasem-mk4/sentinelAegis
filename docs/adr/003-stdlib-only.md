# ADR-003: Go Standard Library Only Architecture

**Date:** 2026-04-24  
**Status:** Accepted  
**Authors:** Mutasem Kharma

## Context

The project must be deployable on Cloud Run's free tier with minimal Docker image size. Framework choices affect build time, image size, attack surface, and long-term maintainability.

## Decision

Use **Go's standard library exclusively** for the HTTP server, routing, middleware, JSON handling, and concurrency primitives. The only external dependencies are:
- `golang.org/x/oauth2` — required for Gmail API authentication
- `google.golang.org/api` — required for Gmail API client

### Specific Choices
| Concern | Standard Library Solution |
|---|---|
| HTTP routing | `net/http` (Go 1.22 method-based routing) |
| JSON encoding | `encoding/json` |
| Middleware | Function composition: `handler(handler(mux))` |
| Concurrency | `sync.WaitGroup`, `sync.RWMutex`, `sync/atomic` |
| Logging | `log/slog` (structured JSON) |
| Configuration | `os.Getenv()` |
| Server-Sent Events | Manual `http.Flusher` implementation |
| Graceful shutdown | `signal.NotifyContext` + `http.Server.Shutdown` |

## Consequences

**Positive:**
- Docker image: **~15MB** (vs. 50–100MB with frameworks)
- Build time: **<5 seconds** (vs. minutes with large dependency trees)
- Zero CVEs from third-party dependencies
- Judges can read the code without framework knowledge
- Full control over middleware behavior

**Negative:**
- No automatic request validation (manual JSON decoding)
- No built-in OpenAPI generation
- SSE implementation requires manual flusher management
- Rate limiting must be hand-implemented

**Alternatives Considered:**
1. **Gin/Echo/Fiber:** Rejected — adds 20+ transitive dependencies, increases image size, obscures Go patterns
2. **gRPC + Connect:** Rejected — over-engineered for HTTP JSON APIs
3. **Chi router:** Considered — minimal overhead, but Go 1.22's built-in method routing eliminated the need
