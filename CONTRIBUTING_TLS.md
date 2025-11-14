# TLS Authentication Feature - Development Guide

This document describes the TLS certificate authentication feature implementation for developers and contributors.

## Feature Overview

This feature adds TLS certificate authentication support to provider-http, enabling:
- Custom CA certificate validation
- Mutual TLS (mTLS) with client certificates
- Configuration at both provider and resource levels
- Kubernetes secret-based certificate management

## Implementation Details

### API Changes

**ProviderConfig (`apis/v1alpha1/providerconfig_types.go`)**
- Added `TLS *common.TLSConfig` field

**Request (`apis/request/v1alpha2/request_types.go`)**  
- Added `TLSConfig *common.TLSConfig` field to `RequestParameters`

**DisposableRequest (`apis/disposablerequest/v1alpha2/disposablerequest_types.go`)**
- Added `TLSConfig *common.TLSConfig` field to `DisposableRequestParameters`

**Common Types (`apis/common/common.go`)**
```go
type TLSConfig struct {
    CABundle              *string           `json:"caBundle,omitempty"`
    CACertSecretRef       *SecretReference  `json:"caCertSecretRef,omitempty"`
    ClientCertSecretRef   *SecretReference  `json:"clientCertSecretRef,omitempty"`
    ClientKeySecretRef    *SecretReference  `json:"clientKeySecretRef,omitempty"`
    InsecureSkipVerify    *bool             `json:"insecureSkipVerify,omitempty"`
}
```

### Core Implementation

**HTTP Client (`internal/clients/http/client.go`)**
- `SendRequestWithTLS()`: HTTP request with TLS configuration
- `buildTLSConfig()`: Constructs `tls.Config` from TLSConfigData

**TLS Loader (`internal/clients/http/tls_loader.go`)**
- `LoadTLSConfig()`: Loads certificates from Kubernetes secrets
- `MergeTLSConfigs()`: Merges provider and resource-level TLS configs

**Controllers**
- `internal/controller/request/request.go`: Integrated TLS support
- `internal/controller/disposablerequest/disposablerequest.go`: Integrated TLS support

## Building and Testing

### Prerequisites

```bash
# Install Go 1.16+
go version

# Install Crossplane CLI (optional, for XPKG packaging)
curl -sL https://raw.githubusercontent.com/crossplane/crossplane/master/install.sh | sh
```

### Build Provider

```bash
# Build binary
make build

# Build Docker image (replace with your registry)
docker build -t myregistry/provider-http:v1.0.0-tls .
```

### Run Tests

```bash
# Unit tests
make test

# Check coverage
go test -coverprofile=coverage.out ./internal/clients/http/
go tool cover -func=coverage.out

# E2E tests
make e2e
```

### Test Coverage

Current test coverage for TLS implementation:
- `internal/clients/http/tls_loader.go`: 95.1%
- `internal/clients/http/client.go`: TLS functions covered

Key test files:
- `internal/clients/http/tls_loader_test.go`: 24 test cases
- `internal/clients/http/client_test.go`: 23 test cases (including TLS tests)

## Docker Build for Custom Registry

### For Azure Container Registry (ACR)

```bash
# Login to ACR
az acr login --name yourregistry

# Build for ARM64 (Mac M1/M2/M3)
docker buildx build \
  --platform linux/arm64 \
  --build-arg ARCH=arm64 \
  -t yourregistry.azurecr.io/provider-http:v1.0.0-tls \
  -f cluster/Dockerfile .

# Push image
docker push yourregistry.azurecr.io/provider-http:v1.0.0-tls
```

### For Other Registries

```bash
# Build multi-platform
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourregistry/provider-http:v1.0.0-tls \
  -f cluster/Dockerfile \
  --push .
```

## Deployment

### Install Provider from Custom Registry

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http-tls
spec:
  package: yourregistry.azurecr.io/provider-http:v1.0.0-tls
  packagePullSecrets:
    - name: acr-credentials
```

### Create Pull Secrets (for private registries)

```bash
kubectl create secret docker-registry acr-credentials \
  --docker-server=yourregistry.azurecr.io \
  --docker-username=<username> \
  --docker-password=<password> \
  --namespace=crossplane-system
```

## Examples and Testing

### Test with Local Cluster

1. **Setup kind cluster with Crossplane:**
```bash
kind create cluster
kubectl apply -f https://raw.githubusercontent.com/crossplane/crossplane/master/cluster/charts/crossplane/crds/
```

2. **Create test certificates:**
```bash
# Generate self-signed CA
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout ca.key -out ca.crt -subj "/CN=Test CA"

# Generate client cert
openssl req -newkey rsa:4096 -nodes \
  -keyout client.key -out client.csr -subj "/CN=Test Client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 365

# Create secrets
kubectl create secret generic ca-certs \
  --from-file=ca.crt=ca.crt \
  --namespace=crossplane-system
kubectl create secret tls client-certs \
  --cert=client.crt \
  --key=client.key \
  --namespace=crossplane-system
```

3. **Test with examples:**
```bash
kubectl apply -f examples/provider/tls-config.yaml
kubectl apply -f examples/sample/request-with-tls.yaml
kubectl get requests
```

## Code Review Checklist

- [ ] API changes follow Crossplane conventions
- [ ] All fields have proper JSON tags and validation
- [ ] New code has >90% test coverage
- [ ] Tests include error cases and edge cases
- [ ] Documentation updated (README, examples)
- [ ] No hardcoded secrets or sensitive data
- [ ] Backward compatibility maintained
- [ ] RBAC permissions documented if needed

## Future Enhancements

Potential improvements for future PRs:
- Certificate rotation support
- Certificate expiration monitoring
- CertificateRequest integration
- Vault/cert-manager integration
- Per-request timeout configuration

## Resources

- [Crossplane Provider Development](https://docs.crossplane.io/latest/concepts/providers/)
- [Go TLS Package](https://pkg.go.dev/crypto/tls)
- [Kubernetes TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)
