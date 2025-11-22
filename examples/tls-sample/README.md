# TLS/mTLS Examples

These examples demonstrate TLS and mutual TLS authentication features. They are provided for reference and documentation purposes.

**Note**: These examples use `https://api.example.com` which is a placeholder. To use these examples in production, replace the URL with your actual HTTPS API endpoint and provide valid certificates.

## Examples Included

### Cluster-scoped Resources
- `request-tls.yaml` - Request resource with ProviderConfig-level TLS
- `request-with-tls.yaml` - Request resource with resource-level TLS override
- `disposablerequest-tls.yaml` - DisposableRequest with ProviderConfig-level TLS
- `disposablerequest-with-tls.yaml` - DisposableRequest with resource-level TLS override

### Namespaced Resources
- `namespaced-providerconfig-tls.yaml` - Namespaced ProviderConfig examples with various TLS configurations
- `namespaced-request-tls.yaml` - Namespaced Request resources with TLS configurations
- `namespaced-disposablerequest-tls.yaml` - Namespaced DisposableRequest resources with TLS configurations
- `namespaced-tls-secrets.yaml` - Example Kubernetes secrets for storing TLS certificates
- `namespaced-TLS-README.md` - Detailed documentation for namespaced TLS configuration

## Usage

These examples require:
1. A valid HTTPS endpoint (replace `https://api.example.com`)
2. TLS certificates stored in Kubernetes secrets (see `namespaced-tls-secrets.yaml` for examples)
3. A ProviderConfig with TLS configuration:
   - For cluster-scoped resources: see `examples/provider/tls-config.yaml`
   - For namespaced resources: see `namespaced-providerconfig-tls.yaml`

For detailed information about configuring TLS for namespaced resources, refer to `namespaced-TLS-README.md`.
