# TLS/mTLS Examples

These examples demonstrate TLS and mutual TLS authentication features. They are provided for reference and documentation purposes.

**Note**: These examples use `https://api.example.com` which is a placeholder. To use these examples in production, replace the URL with your actual HTTPS API endpoint and provide valid certificates.

## Examples Included

- `request-tls.yaml` - Request resource with ProviderConfig-level TLS
- `request-with-tls.yaml` - Request resource with resource-level TLS override
- `disposablerequest-tls.yaml` - DisposableRequest with ProviderConfig-level TLS
- `disposablerequest-with-tls.yaml` - DisposableRequest with resource-level TLS override

## Usage

These examples require:
1. A valid HTTPS endpoint (replace `https://api.example.com`)
2. TLS certificates stored in Kubernetes secrets (see `examples/secrets/certificates-secret.yaml`)
3. A ProviderConfig with TLS configuration (see `examples/provider/tls-config.yaml`)
