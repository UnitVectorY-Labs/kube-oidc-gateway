# kube-oidc-gateway

A lightweight in-cluster HTTP service that exposes Kubernetes API server OIDC discovery and JWKS endpoints with in-memory caching. This enables external systems (such as workload identity integrations) to access these endpoints without requiring anonymous authentication to be enabled on the Kubernetes API server.

## Overview

When running a Kubernetes cluster with `--anonymous-auth=false` for enhanced security, external systems cannot access the OIDC discovery and JWKS endpoints needed for workload identity federation. This gateway solves that problem by:

- Running inside the cluster with a ServiceAccount
- Authenticating to the API server using the ServiceAccount token
- Fetching and re-serving the OIDC discovery and JWKS documents
- Providing lightweight in-memory caching with configurable TTL
- Pretty-printing JSON responses by default

## Supported Endpoints

The gateway exposes exactly two OIDC endpoints:

1. `GET /.well-known/openid-configuration` - OIDC discovery document
2. `GET /openid/v1/jwks` - JSON Web Key Set

Additionally, health check endpoints are available:

- `GET /healthz` - Basic health check (always returns 200 if process is running)
- `GET /readyz` - Readiness check (returns 200 only after successful upstream connection)

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
| `LOG_LEVEL` | string | `info` | Logging verbosity |
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

A complete deployment manifest is available in `deploy/deployment.yaml`. This includes:

- Namespace
- ServiceAccount
- ClusterRole and ClusterRoleBinding (minimal RBAC)
- Deployment (2 replicas)
- ClusterIP Service

Deploy with:

```bash
kubectl apply -f deploy/deployment.yaml
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

For detailed architecture documentation, see [docs/architecture.md](docs/architecture.md).

## Operations Guide

For operational guidance, troubleshooting, and best practices, see [docs/operations.md](docs/operations.md).

## Building

### Local Build

```bash
go build -o kube-oidc-gateway .
```

### Docker Build

```bash
docker build -t kube-oidc-gateway:latest .
```

## License

See [LICENSE](LICENSE) file.

