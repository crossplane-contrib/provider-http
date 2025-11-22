# TLS Configuration Examples for Namespaced Resources

This directory contains examples showing how to configure TLS authentication for namespaced HTTP provider resources.

## Overview

The HTTP provider supports TLS configuration at two levels:

1. **ProviderConfig level**: TLS settings applied to all resources using that ProviderConfig
2. **Resource level**: TLS settings that override ProviderConfig settings for specific resources

## TLS Configuration Options

### CA Certificate Verification
- **caCertSecretRef**: Reference to a secret containing the CA certificate for server verification
- **caBundle**: Inline CA certificate bundle (base64 encoded)

### Mutual TLS (mTLS) Authentication
- **clientCertSecretRef**: Reference to a secret containing the client certificate
- **clientKeySecretRef**: Reference to a secret containing the client private key

### Skip TLS Verification (Not Recommended for Production)
- **insecureSkipVerify**: Skip TLS certificate verification entirely

## Files

### Provider Configuration
- `providerconfig-tls.yaml`: Examples of namespaced ProviderConfigs with different TLS configurations

### Resource Examples
- `disposablerequest-tls.yaml`: DisposableRequest examples with TLS configurations
- `request-tls.yaml`: Request examples with TLS configurations

### Supporting Resources
- `tls-secrets.yaml`: Example Kubernetes secrets for storing TLS certificates

## Usage

1. **Create TLS secrets** (replace placeholder certificates with real ones):
   ```bash
   kubectl apply -f tls-secrets.yaml
   ```

2. **Create ProviderConfig**:
   ```bash
   kubectl apply -f providerconfig-tls.yaml
   ```

3. **Create HTTP resources**:
   ```bash
   kubectl apply -f disposablerequest-tls.yaml
   kubectl apply -f request-tls.yaml
   ```

## TLS Configuration Precedence

When both ProviderConfig and resource-level TLS configurations are present:

1. Resource-level TLS settings override ProviderConfig settings
2. Fields can be mixed (e.g., CA from ProviderConfig, client cert from resource)
3. `insecureSkipTLSVerify` and `tlsConfig` are mutually exclusive

## Examples Explained

### Basic CA Verification
Uses a custom CA certificate to verify server certificates:
```yaml
tlsConfig:
  caCertSecretRef:
    name: my-ca-cert-secret
    namespace: default
    key: ca.crt
```

### Mutual TLS Authentication
Provides both CA verification and client certificate authentication:
```yaml
tlsConfig:
  caCertSecretRef:
    name: my-ca-cert-secret
    namespace: default
    key: ca.crt
  clientCertSecretRef:
    name: my-client-cert-secret
    namespace: default
    key: tls.crt
  clientKeySecretRef:
    name: my-client-cert-secret
    namespace: default
    key: tls.key
```

### Skip TLS Verification (Testing Only)
Disables TLS verification entirely:
```yaml
forProvider:
  insecureSkipTLSVerify: true
```

## Security Notes

- Never use `insecureSkipTLSVerify: true` in production environments
- Store certificates securely in Kubernetes secrets
- Use proper RBAC to restrict access to certificate secrets
- Regularly rotate certificates and update secrets accordingly
- Consider using cert-manager for automated certificate management