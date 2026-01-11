# Architecture

## Overview

The kube-oidc-gateway is a simple HTTP proxy service designed to expose Kubernetes API server OIDC discovery and JWKS endpoints while maintaining security when anonymous authentication is disabled.

## Request Flow

```
┌─────────────┐          ┌──────────────────┐          ┌──────────────────┐
│   External  │          │  kube-oidc-      │          │   Kubernetes     │
│   Client    │─────────▶│  gateway         │─────────▶│   API Server     │
│             │          │                  │          │                  │
└─────────────┘          └──────────────────┘          └──────────────────┘
                                │
                                │
                         ┌──────▼───────┐
                         │  In-Memory   │
                         │  Cache       │
                         │  (TTL-based) │
                         └──────────────┘
```

### Request Processing Steps

1. **Client Request**: External client sends GET request to gateway endpoint
2. **Method Validation**: Gateway ensures only GET methods are allowed (405 for others)
3. **Path Validation**: Gateway validates the path is one of the two allowed endpoints (404 for others)
4. **Cache Check**: Gateway checks in-memory cache for non-expired entry
   - **Cache Hit**: Returns cached response immediately with `Cache-Control` header
   - **Cache Miss**: Proceeds to upstream fetch
5. **Upstream Request**: 
   - Constructs request to Kubernetes API server
   - Adds `Authorization: Bearer <sa-token>` header
   - Uses TLS with in-cluster CA validation
   - Applies configured timeout
6. **Response Processing**:
   - If `PRETTY_PRINT_JSON=true`: Parse and re-format JSON with indentation
   - If `PRETTY_PRINT_JSON=false`: Return upstream bytes as-is
7. **Cache Storage**: Store processed response in cache with TTL
8. **Client Response**: Return formatted JSON with appropriate headers

## Components

### Config (`config.go`)

- Loads all configuration from environment variables
- Provides safe defaults for in-cluster operation
- Validates and parses environment values (strings, integers, booleans)

### Cache (`cache.go`)

- Simple in-memory key-value store with TTL
- Thread-safe with read-write mutex
- Stores separate entries for each endpoint
- Expires entries based on wall-clock time
- No background cleanup (lazy expiration on read)

### Upstream Client (`upstream.go`)

- Manages connection to Kubernetes API server
- Reads ServiceAccount token and CA certificate on initialization
- Configures HTTP client with:
  - Custom TLS config using in-cluster CA
  - Timeout from configuration
- Adds Bearer token to all requests
- Returns 502 on upstream errors

### HTTP Handlers (`handlers.go`)

- **HandleOIDCDiscovery**: Serves `/.well-known/openid-configuration`
- **HandleJWKS**: Serves `/openid/v1/jwks`
- **HandleHealthz**: Basic liveness probe (always returns 200)
- **HandleReadyz**: Readiness probe (returns 200 after first successful upstream fetch)
- **HandleNotFound**: Catch-all for undefined paths
- Logs each request with path, status, cache hit/miss, and latency

### Main (`main.go`)

- Entry point and HTTP server setup
- Initializes configuration and components
- Registers route handlers
- Starts HTTP server on configured address/port

## Cache Behavior

### TTL-Based Expiration

- Default TTL: 60 seconds (configurable via `CACHE_TTL_SECONDS`)
- Each cache entry stores:
  - Response body (processed JSON bytes)
  - Expiration timestamp
- Entries are considered expired when `time.Now() > ExpiresAt`

### Cache Key Strategy

- Simple path-based keys:
  - `/.well-known/openid-configuration`
  - `/openid/v1/jwks`
- No query parameter or header variation

### Cache Semantics

- **First Request**: Cache miss → fetch from upstream → store → return
- **Subsequent Requests (within TTL)**: Cache hit → return immediately
- **After TTL Expiration**: Cache miss → fetch from upstream → update cache → return
- **Upstream Error**: Return 502 (no stale-if-error support in default configuration)

### Cache-Control Header

The gateway sets `Cache-Control: max-age=<ttlSeconds>` on all successful responses. This:
- Informs downstream caches (CDNs, browsers) about freshness
- Aligns with the gateway's own cache TTL
- Helps prevent unnecessary requests during the cache period

## Error Handling

### Status Code Mapping

| Scenario | Status Code | Description |
|----------|-------------|-------------|
| Successful response | 200 | Data fetched and returned |
| Non-GET method | 405 | Only GET is allowed |
| Unknown path | 404 | Path not in allowed list |
| Upstream network/TLS error | 502 | Cannot reach API server |
| Upstream non-200 status | 502 | API server error |
| JSON parse error (pretty-print=true) | 502 | Upstream returned invalid JSON |
| JSON marshal error (internal) | 500 | Unexpected serialization error |
| ServiceAccount token/CA unreadable | 503 | App not ready (at startup or /readyz) |

### Error Logging

All errors are logged with:
- Error type/category
- Request path
- Error message
- Latency information

No sensitive data (ServiceAccount token) is logged.

## Security Design

### In-Cluster Authentication

- Uses mounted ServiceAccount token at `/var/run/secrets/kubernetes.io/serviceaccount/token`
- Token is read once at startup (no refresh logic - rely on Kubernetes token auto-rotation)
- Token is sent as `Authorization: Bearer <token>` header

### TLS Validation

- Uses in-cluster CA certificate at `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`
- Configured in HTTP client's TLS config
- Prevents MITM attacks between gateway and API server

### Minimal RBAC

The gateway requires only:
```yaml
rules:
- nonResourceURLs:
  - "/openid/v1/jwks"
  - "/.well-known/openid-configuration"
  verbs: ["get"]
```

This follows the principle of least privilege:
- No access to any Kubernetes resources (pods, secrets, etc.)
- No access to other API server endpoints
- Read-only access to two specific non-resource URLs

### No Client Authentication

The gateway itself does **not** authenticate or authorize clients. This is intentional:
- OIDC discovery and JWKS are public metadata
- Access control is delegated to Kubernetes networking (NetworkPolicy, Ingress, Service type)
- Deployment topology determines exposure level

## Scalability Considerations

### Horizontal Scaling

- Gateway is stateless (cache is local to each pod)
- Can run multiple replicas for high availability
- Each replica maintains its own cache
- No coordination between replicas needed

### Cache Efficiency

- Reduces load on Kubernetes API server
- Default 60s TTL balances freshness with efficiency
- OIDC discovery and JWKS change infrequently, so caching is effective

### Resource Usage

- Minimal memory footprint (cache stores only 2 JSON documents)
- No disk I/O (fully in-memory)
- Low CPU usage (simple HTTP proxy with JSON parsing)
- Recommended resources:
  - Requests: 10m CPU, 32Mi memory
  - Limits: 200m CPU, 128Mi memory

## Observability

### Logging

Each request logs:
- Path
- HTTP status code
- Cache hit/miss indicator
- Upstream latency (if fetched)
- Total request latency

Example log line:
```
path=/.well-known/openid-configuration status=200 cache_hit=true duration=245µs
```

### Health Checks

- **Liveness (`/healthz`)**: Simple "is process running" check
- **Readiness (`/readyz`)**: Validates upstream connectivity
  - Returns 503 until first successful upstream fetch
  - Prevents traffic to pods that can't reach API server

### Future: Metrics

Prometheus-style metrics could include:
- `http_requests_total{path, status}` - Request counter
- `cache_hits_total{path}` - Cache hit counter
- `cache_misses_total{path}` - Cache miss counter
- `upstream_request_duration_seconds{path}` - Upstream latency histogram
- `request_duration_seconds{path}` - Total request latency histogram

## Configuration Philosophy

### Environment Variable Driven

- No configuration files
- All settings via environment variables
- Easy to configure in Kubernetes manifests
- Twelve-factor app compliance

### Safe Defaults

- Works out-of-box in standard Kubernetes environments
- No required configuration
- Defaults assume in-cluster deployment
- Override only when necessary

## Limitations

### No Persistent Cache

- Cache is in-memory only
- Lost on pod restart
- Not shared between replicas

### No Token Refresh

- ServiceAccount token is read once at startup
- Relies on Kubernetes' automatic token rotation
- Long-running pods (days/weeks) may need restart if token mount changes

### Fixed Paths

- Only two endpoints supported
- No support for proxying other API server paths
- This is intentional (minimal scope)

### No Client-Side Caching Control

- Cache TTL is server-side only
- Cannot vary cache by client headers
- Cannot handle `If-None-Match`, `If-Modified-Since`

## Future Enhancements

Potential improvements (not currently implemented):

1. **Metrics Endpoint**: Add `/metrics` for Prometheus scraping
2. **Stale-If-Error**: Optionally serve stale cache on upstream errors
3. **Structured Logging**: JSON-formatted logs for better parsing
4. **Graceful Shutdown**: Wait for in-flight requests before terminating
5. **Token Refresh**: Watch and reload ServiceAccount token on changes
6. **Vary Support**: Cache variations based on request headers
