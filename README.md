[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/kube-oidc-gateway.svg)](https://github.com/UnitVectorY-Labs/kube-oidc-gateway/releases/latest) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Active](https://img.shields.io/badge/Status-Active-green)](https://guide.unitvectorylabs.com/bestpractices/status/#active)

# kube-oidc-gateway

A reverse proxy that exposes Kubernetes OIDC discovery and JWKS endpoints with lightweight in-memory caching so external systems can configure workload identity federation without requiring anonymous authentication on the Kubernetes API server.

## Why Expose Kubernetes OIDC Endpoints?

Kubernetes can act as an OIDC identity provider, allowing workloads running in the cluster to authenticate to external systems using ServiceAccount tokens. This is commonly known as **workload identity federation**.

For workload identity federation to work, the external system (such as a cloud provider, HashiCorp Vault, or any OIDC-compliant service) needs to:

1. **Discover the OIDC configuration** - by fetching `/.well-known/openid-configuration` from the Kubernetes API server
2. **Validate JWT signatures** - by fetching the public keys from `/openid/v1/jwks`

**The challenge**: Even if your Kubernetes cluster is entirely private (no public API server endpoint), these OIDC endpoints **must be publicly accessible** for external systems to validate tokens. External systems cannot reach into your private network to fetch these documents.

**Common scenarios requiring public OIDC endpoints:**

- **Cloud Provider Workload Identity**: AWS IAM Roles for Service Accounts (IRSA), GCP Workload Identity, Azure Workload Identity
- **HashiCorp Vault**: JWT/OIDC authentication method
- **CI/CD Systems**: GitHub Actions OIDC, GitLab CI OIDC
- **Any OIDC-relying party**: Services that accept Kubernetes ServiceAccount tokens for authentication

## The Problem This Gateway Solves

When running a Kubernetes cluster with `--anonymous-auth=false` (a common security hardening practice), external systems cannot access the OIDC discovery and JWKS endpoints because:

1. The Kubernetes API server requires authentication for all requests
2. External systems don't have credentials to authenticate to your cluster
3. You don't want to enable anonymous authentication just to serve these public OIDC documents

This gateway solves the problem by:

- Running inside the cluster with a ServiceAccount
- Authenticating to the API server using the ServiceAccount token
- Fetching and re-serving the OIDC discovery and JWKS documents
- Providing lightweight in-memory caching with configurable TTL
- Pretty-printing JSON responses by default

**Important**: The OIDC discovery document and JWKS contain only public information (public keys and metadata). They do not expose any secrets or sensitive cluster information. Making these endpoints publicly accessible is safe and necessary for workload identity federation to function.

## Supported Endpoints

The gateway exposes exactly two OIDC endpoints:

1. `GET /.well-known/openid-configuration` - OIDC discovery document
2. `GET /openid/v1/jwks` - JSON Web Key Set

Additionally, health check endpoints are available:

- `GET /healthz` - Liveness check (fetches and caches both OIDC endpoints)
- `GET /readyz` - Readiness check (fetches and caches both OIDC endpoints)

All other paths return `404 Not Found`.

## Usage Examples

### Query the OIDC discovery endpoint

```bash
curl http://kube-oidc-gateway/.well-known/openid-configuration
```

### Query the JWKS endpoint

```bash
curl http://kube-oidc-gateway/openid/v1/jwks
```

### Check health

```bash
curl http://kube-oidc-gateway/healthz
```

## Configuration

All configuration is done via environment variables with safe defaults for in-cluster operation.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LISTEN_ADDR` | string | `0.0.0.0` | Bind address |
| `LISTEN_PORT` | string | `8080` | HTTP listen port |
| `UPSTREAM_HOST` | string | `https://kubernetes.default.svc` | Kubernetes API server base URL |
| `UPSTREAM_TIMEOUT_SECONDS` | int | `5` | Timeout for upstream HTTP calls |
| `CACHE_TTL_SECONDS` | int | `60` | In-memory cache TTL in seconds |
| `PRETTY_PRINT_JSON` | bool | `true` | Pretty-print JSON responses |
| `SA_TOKEN_PATH` | string | `/var/run/secrets/kubernetes.io/serviceaccount/token` | ServiceAccount token path |
| `SA_CA_CERT_PATH` | string | `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt` | ServiceAccount CA certificate path |

## Kubernetes Deployment

### RBAC Requirements

The ServiceAccount used by the gateway requires minimal permissions to access only the two non-resource OIDC endpoints:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-oidc-gateway
rules:
- nonResourceURLs:
  - "/openid/v1/jwks"
  - "/.well-known/openid-configuration"
  verbs: ["get"]
```

### Complete Deployment Example

Deploy with the following manifest that includes all necessary resources:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kube-oidc-gateway
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-oidc-gateway
rules:
- nonResourceURLs:
  - "/openid/v1/jwks"
  - "/.well-known/openid-configuration"
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-oidc-gateway
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kube-oidc-gateway
subjects:
- kind: ServiceAccount
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kube-oidc-gateway
  template:
    metadata:
      labels:
        app: kube-oidc-gateway
    spec:
      serviceAccountName: kube-oidc-gateway
      containers:
      - name: kube-oidc-gateway
        image: ghcr.io/unitvectory-labs/kube-oidc-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: CACHE_TTL_SECONDS
          value: "60"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "32Mi"
            cpu: "10m"
          limits:
            memory: "64Mi"
            cpu: "100m"
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 65534
          capabilities:
            drop:
            - ALL
---
apiVersion: v1
kind: Service
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
spec:
  selector:
    app: kube-oidc-gateway
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

Save this to a file and deploy with:

```bash
kubectl apply -f kube-oidc-gateway.yaml
```

### Exposing the Service

The default deployment creates a ClusterIP service. You can expose it externally using:

**Ingress:**
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
spec:
  rules:
  - host: oidc.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kube-oidc-gateway
            port:
              number: 80
```

**LoadBalancer Service:**
```bash
kubectl patch svc kube-oidc-gateway -n kube-oidc-gateway -p '{"spec":{"type":"LoadBalancer"}}'
```

## Security Considerations

- **No Built-in Authentication**: This service does not implement authentication or authorization. It serves public OIDC discovery data but access control is your responsibility.
- **Network Exposure**: Control who can access the service using Kubernetes NetworkPolicies, Ingress authentication, or firewall rules.
- **Minimal RBAC**: The ServiceAccount has minimal permissions (only read access to two non-resource URLs).
- **Works with --anonymous-auth=false**: Designed specifically to work when the API server disables anonymous authentication.

## Architecture

The gateway follows a simple request flow:

```
External Client → kube-oidc-gateway → Kubernetes API Server
```

1. **Request Handling**: The gateway exposes only two OIDC endpoints plus health checks
2. **Authentication**: Uses the mounted ServiceAccount token to authenticate to the Kubernetes API server
3. **Caching**: Maintains an in-memory cache with configurable TTL to reduce load on the API server
4. **Response Processing**: Optionally pretty-prints JSON responses for easier debugging

Key design principles:
- **Minimal attack surface**: Only two read-only endpoints are exposed
- **No secrets in responses**: OIDC documents contain only public keys and metadata
- **Resilient**: Serves stale cache entries on upstream failures
- **Lightweight**: Single binary with minimal resource requirements

## Operations

### Monitoring

The gateway logs all requests with the following information:
- Request path
- HTTP status code
- Cache hit/miss
- Request duration

Example log output:
```
path=/.well-known/openid-configuration status=200 cache_hit=true duration=1.234ms
```

### Troubleshooting

**503 Service Unavailable on /healthz or /readyz**
- The gateway cannot reach the Kubernetes API server
- Check ServiceAccount token is mounted correctly
- Verify ClusterRole permissions are applied

**502 Bad Gateway on OIDC endpoints**
- Upstream request to Kubernetes API server failed
- Check network connectivity to `kubernetes.default.svc`
- Verify the API server is healthy

### Cache Behavior

- Default TTL is 60 seconds
- On cache miss, fetches from upstream and caches the result
- On upstream failure with cached data, serves stale cache (stale-on-error)
- ETags are generated for cache validation

## Building

### Local Build

```bash
go build -o kube-oidc-gateway .
```

### Docker Build

```bash
docker build -t kube-oidc-gateway:latest .
```
