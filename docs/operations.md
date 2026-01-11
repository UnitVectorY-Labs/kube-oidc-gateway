# Operations Guide

## Deployment

### Prerequisites

- Kubernetes cluster with RBAC enabled
- Cluster API server configured with OIDC (optional but typical)
- `kubectl` configured with cluster admin access

### Standard Deployment

Deploy using the provided manifest:

```bash
kubectl apply -f deploy/deployment.yaml
```

This creates:
- `kube-oidc-gateway` namespace
- ServiceAccount with minimal RBAC
- Deployment with 2 replicas
- ClusterIP Service on port 80

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n kube-oidc-gateway

# Check logs
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway

# Test from within cluster
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://kube-oidc-gateway.kube-oidc-gateway/.well-known/openid-configuration
```

## Configuration

### Recommended Settings

#### Production

```yaml
env:
- name: CACHE_TTL_SECONDS
  value: "60"  # Balance freshness with API server load
- name: PRETTY_PRINT_JSON
  value: "true"  # Easier debugging, minimal overhead
- name: UPSTREAM_TIMEOUT_SECONDS
  value: "5"  # Sufficient for most clusters
- name: LOG_LEVEL
  value: "info"  # Adequate for normal operations
```

#### High-Traffic Environments

```yaml
env:
- name: CACHE_TTL_SECONDS
  value: "300"  # 5 minutes - reduce API server load
- name: PRETTY_PRINT_JSON
  value: "false"  # Slight performance improvement
```

#### Development/Debugging

```yaml
env:
- name: CACHE_TTL_SECONDS
  value: "10"  # See changes quickly
- name: LOG_LEVEL
  value: "debug"  # Verbose logging
```

### Resource Sizing

#### Starting Point (recommended)

```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 200m
    memory: 128Mi
```

#### High-Traffic Clusters

```yaml
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

### Replica Count

- **Minimum**: 2 replicas for high availability
- **Production**: 2-3 replicas (distributes load, handles pod failures)
- **High-Traffic**: 3-5 replicas

The service is lightweight, so more replicas are better for availability than performance.

### Probes

Default probe configuration is appropriate for most use cases:

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: http
  initialDelaySeconds: 2
  periodSeconds: 10
  
livenessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 2
  periodSeconds: 10
```

Adjust if your cluster has slow API server responses:

```yaml
readinessProbe:
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 5
```

## Exposing the Service

### Internal Only (ClusterIP)

Default deployment - accessible only within cluster:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kube-oidc-gateway
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: 8080
```

### External Access via Ingress

For external workload identity systems:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - oidc.example.com
    secretName: oidc-tls
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

### External Access via LoadBalancer

```bash
kubectl patch svc kube-oidc-gateway -n kube-oidc-gateway \
  -p '{"spec":{"type":"LoadBalancer"}}'
```

### Network Policies

Restrict access to specific namespaces:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
spec:
  podSelector:
    matchLabels:
      app: kube-oidc-gateway
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          allowed-to-oidc: "true"
    ports:
    - protocol: TCP
      port: 8080
```

## Monitoring

### Log Analysis

Successful requests:
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep "status=200"
```

Cache hit rate:
```bash
# Cache hits
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep "cache_hit=true" | wc -l

# Cache misses
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep "cache_hit=false" | wc -l
```

Errors:
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep "error"
```

### Health Checks

```bash
# Liveness check
kubectl exec -n kube-oidc-gateway deployment/kube-oidc-gateway -- \
  wget -q -O- http://localhost:8080/healthz

# Readiness check
kubectl exec -n kube-oidc-gateway deployment/kube-oidc-gateway -- \
  wget -q -O- http://localhost:8080/readyz
```

### Performance Metrics

Watch response times in logs:
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway -f | grep duration=
```

## Troubleshooting

### Issue: Pods Not Ready

**Symptom**: Pods stuck in `0/1 Ready` state

**Diagnosis**:
```bash
kubectl get pods -n kube-oidc-gateway
kubectl describe pod -n kube-oidc-gateway <pod-name>
kubectl logs -n kube-oidc-gateway <pod-name>
```

**Common Causes**:

1. **RBAC Denied**
   
   Error log: `upstream returned status 403`
   
   **Fix**: Verify ClusterRole and ClusterRoleBinding:
   ```bash
   kubectl get clusterrole kube-oidc-gateway -o yaml
   kubectl get clusterrolebinding kube-oidc-gateway -o yaml
   ```
   
   Ensure the ClusterRole has:
   ```yaml
   rules:
   - nonResourceURLs:
     - "/openid/v1/jwks"
     - "/.well-known/openid-configuration"
     verbs: ["get"]
   ```

2. **API Server Unreachable**
   
   Error log: `upstream request failed` or `connection refused`
   
   **Fix**: Check `UPSTREAM_HOST` setting:
   ```bash
   kubectl get svc kubernetes -o yaml
   kubectl exec -n kube-oidc-gateway <pod-name> -- \
     nslookup kubernetes.default.svc
   ```

3. **ServiceAccount Token Missing**
   
   Error log: `failed to read service account token`
   
   **Fix**: Verify ServiceAccount is configured:
   ```bash
   kubectl get sa kube-oidc-gateway -n kube-oidc-gateway
   kubectl describe pod -n kube-oidc-gateway <pod-name> | grep serviceAccount
   ```

### Issue: 502 Bad Gateway Errors

**Symptom**: Client receives `502 Bad Gateway`

**Diagnosis**:
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep "status=502"
```

**Common Causes**:

1. **Upstream API Server Error**
   
   Error log: `upstream returned status <non-200>`
   
   **Fix**: Check API server health:
   ```bash
   kubectl get --raw /.well-known/openid-configuration
   kubectl get --raw /openid/v1/jwks
   ```

2. **TLS/CA Certificate Issues**
   
   Error log: `x509: certificate signed by unknown authority`
   
   **Fix**: Verify CA certificate is mounted:
   ```bash
   kubectl exec -n kube-oidc-gateway <pod-name> -- \
     cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
   ```

3. **Invalid JSON Response (when pretty-print enabled)**
   
   Error log: `json_parse_error`
   
   **Fix**: Check upstream response format or disable pretty-print:
   ```yaml
   env:
   - name: PRETTY_PRINT_JSON
     value: "false"
   ```

### Issue: High Latency

**Symptom**: Slow response times

**Diagnosis**:
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep duration= | tail -20
```

**Common Causes**:

1. **Cache Misses**
   
   Most requests show `cache_hit=false`
   
   **Fix**: Increase cache TTL:
   ```yaml
   env:
   - name: CACHE_TTL_SECONDS
     value: "300"
   ```

2. **Slow API Server**
   
   High `upstream_fetch` durations in logs
   
   **Fix**: Increase timeout or investigate API server performance:
   ```yaml
   env:
   - name: UPSTREAM_TIMEOUT_SECONDS
     value: "10"
   ```

3. **Resource Constraints**
   
   **Fix**: Check resource usage and increase limits:
   ```bash
   kubectl top pods -n kube-oidc-gateway
   ```

### Issue: Anonymous Auth Errors

**Symptom**: Even with gateway, getting anonymous auth errors

**This is expected** - the gateway is designed to be used when `--anonymous-auth=false` is set on the API server. External clients should connect to the gateway, not directly to the API server.

**Fix**: Ensure clients are configured to use the gateway's URL, not the API server URL.

### Issue: Logs Show Token in Output

**Symptom**: Concern about ServiceAccount token in logs

**Expected Behavior**: The gateway explicitly avoids logging the ServiceAccount token. If you see token data in logs, this is a bug - please report it.

**Verify**: Search logs for "Bearer":
```bash
kubectl logs -n kube-oidc-gateway -l app=kube-oidc-gateway | grep -i bearer
```

Should return no results.

## Upgrades

### Rolling Update

The default deployment uses a RollingUpdate strategy:

```bash
kubectl set image deployment/kube-oidc-gateway \
  kube-oidc-gateway=ghcr.io/unitvectory-labs/kube-oidc-gateway:v1.1.0 \
  -n kube-oidc-gateway
```

Monitor rollout:
```bash
kubectl rollout status deployment/kube-oidc-gateway -n kube-oidc-gateway
```

### Rollback

If issues occur:
```bash
kubectl rollout undo deployment/kube-oidc-gateway -n kube-oidc-gateway
```

## Backup and Recovery

### Configuration Backup

Save current configuration:
```bash
kubectl get all,sa,clusterrole,clusterrolebinding -n kube-oidc-gateway -o yaml > backup.yaml
```

### Recovery

Restore from backup:
```bash
kubectl apply -f backup.yaml
```

## Performance Tuning

### Cache Tuning

The cache TTL significantly impacts both performance and freshness:

- **Short TTL (10-30s)**: More API server requests, fresher data
- **Medium TTL (60-120s)**: Balanced (recommended)
- **Long TTL (300-600s)**: Fewer requests, but may not reflect JWKS rotations quickly

JWKS keys rotate infrequently (days/weeks), so longer TTLs are safe.

### Connection Pooling

The Go HTTP client maintains a connection pool automatically. For very high traffic, consider:

```go
// Would require code change
Transport: &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
}
```

## Security Hardening

### Pod Security Standards

Apply restricted Pod Security Standard:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kube-oidc-gateway
  labels:
    pod-security.kubernetes.io/enforce: restricted
```

### Read-Only Filesystem

Add security context:
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

### Network Policies

Implement strict network policies to control egress and ingress.

## Maintenance

### Log Rotation

Logs are written to stdout/stderr and managed by Kubernetes. Configure log retention in your logging system (e.g., Elasticsearch, Loki).

### Cache Clear

To clear cache, restart pods:
```bash
kubectl rollout restart deployment/kube-oidc-gateway -n kube-oidc-gateway
```

Cache is in-memory only and does not persist.

## Common Patterns

### Multi-Cluster Setup

Deploy in each cluster separately. Each cluster's gateway serves its own API server's OIDC endpoints.

### External DNS Integration

Use External DNS to automatically create DNS records:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kube-oidc-gateway
  namespace: kube-oidc-gateway
  annotations:
    external-dns.alpha.kubernetes.io/hostname: oidc.example.com
spec:
  type: LoadBalancer
```

### Behind API Gateway

Place behind an API gateway (Kong, Ambassador, etc.) for:
- Rate limiting
- Authentication
- Request transformation
- Analytics

## Best Practices

1. **Always run at least 2 replicas** for high availability
2. **Use Ingress with TLS** when exposing externally
3. **Monitor logs** for errors and performance issues
4. **Set appropriate cache TTL** based on your JWKS rotation policy
5. **Use NetworkPolicies** to restrict access
6. **Keep the image updated** for security patches
7. **Test RBAC changes** in non-production first
8. **Document your exposure method** (Ingress, LoadBalancer, etc.) for team reference
