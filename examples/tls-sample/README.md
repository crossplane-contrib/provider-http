# TLS/mTLS Examples

These examples demonstrate TLS and mutual TLS (mTLS) authentication features. Each example is self-contained and includes all required resources (secrets, ProviderConfig, and Request/DisposableRequest).

**Important**: These examples use `https://api.example.com` as placeholders. Replace with your actual HTTPS endpoints and provide valid certificates before deploying.

## Examples

### 1. complete-tls-example.yaml
Basic TLS setup with custom CA certificate verification.

**What it demonstrates:**
- Creating a secret with CA certificate
- Configuring ProviderConfig with TLS
- Using the TLS configuration in a Request

**Use case:** API secured with a self-signed certificate or private CA.

### 2. complete-mtls-example.yaml
Mutual TLS (mTLS) authentication with both server verification and client certificates.

**What it demonstrates:**
- Server certificate verification with CA bundle
- Client certificate authentication
- DisposableRequest with mTLS

**Use case:** High-security APIs requiring client certificate authentication.

### 3. tls-override-example.yaml
Overriding TLS settings at the resource level.

**What it demonstrates:**
- Default TLS in ProviderConfig
- Overriding CA bundle for specific requests
- Using `insecureSkipTLSVerify` for testing

**Use case:** Multiple endpoints with different certificates, or testing/development scenarios.

## Quick Start

1. **Create the required secrets** with your actual certificates:
   ```bash
   kubectl create secret generic my-api-ca-cert \
     --from-file=ca.crt=path/to/your/ca.crt \
     -n crossplane-system
   ```

2. **Apply the ProviderConfig**:
   ```bash
   kubectl apply -f complete-tls-example.yaml
   ```

3. **Verify the setup**:
   ```bash
   kubectl get providerconfig https-with-tls
   kubectl get request https-request-example
   ```

## Configuration Reference

### ProviderConfig TLS Fields

```yaml
tlsConfig:
  # CA bundle for server certificate verification
  caBundleConfig:
    source: Secret|Inline
    secretRef:
      name: secret-name
      namespace: crossplane-system
      key: ca.crt
  
  # Client certificate for mTLS (optional)
  clientCertificateConfig:
    source: Secret|Inline
    secretRef:
      name: client-cert-secret
      key: tls.crt
    clientKeySecretRef:
      name: client-cert-secret
      key: tls.key
  
  # Skip TLS verification (NOT recommended for production)
  insecureSkipTLSVerify: true
```

### Resource-level TLS Override

Request and DisposableRequest resources can override ProviderConfig TLS settings:

```yaml
spec:
  forProvider:
    # ... other fields
    tlsConfig:
      # Same structure as ProviderConfig.tlsConfig
      caBundleConfig:
        source: Secret
        secretRef:
          name: different-ca
          key: ca.crt
```

## Legacy Examples

The following files are kept for reference but may not be complete:
- `disposablerequest-tls.yaml` - Basic DisposableRequest examples (requires separate ProviderConfig)
- `disposablerequest-with-tls.yaml` - DisposableRequest with inline TLS config
- `request-tls.yaml` - Request resource examples
- `request-with-tls.yaml` - Request with TLS override

For production use, prefer the complete examples above.
