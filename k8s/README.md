# Kubernetes Deployment for iCal Proxy

This directory contains Kubernetes manifests for deploying the iCal Proxy application.

## Files Overview

- `namespace.yaml` - Creates a dedicated namespace for the application
- `rbac.yaml` - ServiceAccount, ConfigMap, and Secret definitions
- `deployment.yaml` - Main deployment, service, and ingress configuration
- `autoscaling.yaml` - Horizontal Pod Autoscaler and Pod Disruption Budget
- `network-policy.yaml` - Network security policies
- `kustomization.yaml` - Kustomize configuration for easy deployment

## Quick Deployment

### Option 1: Using kubectl directly

```bash
# Deploy all manifests
kubectl apply -f k8s/

# Or deploy in order
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/autoscaling.yaml
kubectl apply -f k8s/network-policy.yaml
```

### Option 2: Using Kustomize

```bash
# Deploy using kustomize
kubectl apply -k k8s/

# Or with specific kustomization
kustomize build k8s/ | kubectl apply -f -
```

## Configuration

### Required Changes

Before deploying, you need to modify:

1. **Domain name** in `deployment.yaml`:
   ```yaml
   spec:
     tls:
     - hosts:
       - ical-proxy.yourdomain.com  # Replace with your domain
   ```

2. **Image tag** (optional) in `deployment.yaml` or via kustomize:
   ```yaml
   image: ghcr.io/konairius/ical-proxy:latest  # Change to specific version
   ```

### Optional Configurations

1. **TLS/SSL Certificate**: Uncomment cert-manager annotations in `deployment.yaml`
2. **Resource limits**: Adjust CPU/memory in `deployment.yaml`
3. **Replica count**: Modify replicas in `deployment.yaml` and HPA in `autoscaling.yaml`
4. **Environment variables**: Add to ConfigMap in `rbac.yaml`

## Environment Variables

Configure the application via the ConfigMap in `rbac.yaml`:

```yaml
data:
  PORT: "8080"
  LOG_LEVEL: "info"
  # Add other environment variables as needed
```

## Security Features

- **Non-root user**: Runs as user 1001
- **Read-only filesystem**: Root filesystem is read-only
- **Security context**: Drops all capabilities
- **Network policies**: Restricts network access
- **Service account**: Dedicated service account with minimal permissions

## Monitoring and Health Checks

- **Liveness probe**: Checks `/health` endpoint every 30 seconds
- **Readiness probe**: Checks `/health` endpoint every 10 seconds
- **Metrics**: Ready for Prometheus scraping (enable annotations in deployment)

## Autoscaling

- **HPA**: Scales based on CPU (70%) and memory (80%) usage
- **Min replicas**: 2
- **Max replicas**: 10
- **PDB**: Ensures at least 1 pod is always available during updates

## Networking

- **Service**: ClusterIP service exposing port 80
- **Ingress**: NGINX ingress with SSL redirect enabled
- **Network Policy**: Restricts ingress/egress traffic

## Troubleshooting

### Check deployment status
```bash
kubectl get pods -n ical-proxy
kubectl describe deployment ical-proxy -n ical-proxy
```

### Check logs
```bash
kubectl logs -f deployment/ical-proxy -n ical-proxy
```

### Check service and ingress
```bash
kubectl get svc,ingress -n ical-proxy
```

### Test health endpoint
```bash
kubectl port-forward -n ical-proxy svc/ical-proxy-service 8080:80
curl http://localhost:8080/health
```

## Cleanup

```bash
# Remove all resources
kubectl delete -k k8s/

# Or remove namespace (will delete everything)
kubectl delete namespace ical-proxy
```

## Production Considerations

1. **Image Tags**: Use specific version tags instead of `latest`
2. **Resource Limits**: Set appropriate CPU/memory limits based on load testing
3. **Monitoring**: Enable Prometheus metrics collection
4. **Logging**: Configure structured logging and log aggregation
5. **Backup**: Consider backing up ConfigMaps and Secrets
6. **Security**: Review and harden network policies based on your environment
7. **SSL/TLS**: Configure proper certificates for production domains

## Example Production Deployment

```bash
# 1. Update image tag for production
kubectl patch deployment ical-proxy -n ical-proxy -p '{"spec":{"template":{"spec":{"containers":[{"name":"ical-proxy","image":"ghcr.io/konairius/ical-proxy:v1.0.0"}]}}}}'

# 2. Scale for production load
kubectl scale deployment ical-proxy -n ical-proxy --replicas=5

# 3. Update HPA for production
kubectl patch hpa ical-proxy-hpa -n ical-proxy -p '{"spec":{"maxReplicas":20}}'
```
