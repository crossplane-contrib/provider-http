# provider-http

`provider-http` is a Crossplane Provider designed to facilitate sending HTTP requests as resources.

## Installation

To install `provider-http`, you have two options:

1. Using the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

   ```console
   crossplane xpkg install provider xpkg.upbound.io/crossplane-contrib/provider-http:v1.0.13
   ```

2. Manually creating a Provider by applying the following YAML:

   ```yaml
   apiVersion: pkg.crossplane.io/v1
   kind: Provider
   metadata:
     name: provider-http
   spec:
     package: 'xpkg.upbound.io/crossplane-contrib/provider-http:v1.0.13'
   ```

## Supported Resources

`provider-http` supports resources in two scopes:

### Cluster-scoped Resources (`http.crossplane.io`)

- **DisposableRequest:** Initiates a one-time HTTP request. See [DisposableRequest CRD documentation](resources-docs/disposablerequest_docs.md).
- **Request:** Manages a resource through HTTP requests. See [Request CRD documentation](resources-docs/request_docs.md).

### Namespaced Resources (`http.m.crossplane.io`)

- **DisposableRequest:** Namespace-scoped version of the disposable HTTP request.
- **Request:** Namespace-scoped version of the managed HTTP resource.
- **ProviderConfig:** Namespace-scoped provider configuration.
- **ClusterProviderConfig:** Cluster-scoped provider configuration for cross-namespace access.

**When to use each:**

- Use **cluster-scoped** resources for shared infrastructure and when you have cluster-admin privileges
- Use **namespaced** resources for tenant isolation, application-specific resources, and when working with namespace-level permissions

## TLS Certificate Authentication

The provider supports TLS certificate-based authentication for secure API communication:

- **CA Certificates:** Trust custom certificate authorities
- **Client Certificates:** Mutual TLS (mTLS) authentication
- **Flexible Configuration:** Set TLS at provider or resource level
- **Secret References:** Load certificates from Kubernetes secrets

### Quick Start

1. **Create certificate secrets:**

```bash
# CA certificate
kubectl create secret generic ca-certs \
  --from-file=ca.crt=./ca-cert.pem \
  --namespace=crossplane-system

# Client certificate for mTLS
kubectl create secret tls client-certs \
  --cert=./client.crt \
  --key=./client.key \
  --namespace=crossplane-system
```

2. **Configure ProviderConfig:**

```yaml
apiVersion: http.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: secure-http
spec:
  credentials:
    source: None
  tls:
    caCertSecretRef:
      name: ca-certs
      namespace: crossplane-system
      key: ca.crt
    clientCertSecretRef:
      name: client-certs
      namespace: crossplane-system
      key: tls.crt
    clientKeySecretRef:
      name: client-certs
      namespace: crossplane-system
      key: tls.key
```

3. **Use in requests:**

```yaml
apiVersion: http.crossplane.io/v1alpha2
kind: Request
metadata:
  name: secure-api-call
spec:
  providerConfigRef:
    name: secure-http
  forProvider:
    url: https://api.example.com/resource
    method: GET
```

See [examples/provider/tls-config.yaml](examples/provider/tls-config.yaml) for more configuration options.

## Sensitive Data Masking

When a request carries credentials — API tokens, Basic-auth headers, database passwords — those values should never be placed in plaintext in a resource `spec`. Anything written to `spec` is persisted in etcd, visible to anyone with `get` RBAC on the resource, rendered in the ArgoCD UI, and (under GitOps) stored in your Git history.

`provider-http` addresses this with **secret-reference placeholders**. You store the sensitive value in a Kubernetes `Secret` and reference it from the request body or headers using the syntax:

```
{{ secret-name:secret-namespace:secret-key }}
```

At reconcile time the provider:

1. Resolves each placeholder to the live value from the referenced `Secret` and sends the **resolved** value over the wire.
2. Stores only the **placeholder** form in `status.requestDetails` and writes only the placeholder form to controller logs.

As a result the outgoing credential never appears in the resource `spec`, in `status.requestDetails` or in controller logs — it lives only in the `Secret`.

> Placeholders are supported in the request **body** and **headers** only.

> **Important — this covers outgoing request data only.** Placeholders protect what you *send*. They do **not** mask the HTTP *response*. If the endpoint echoes a sensitive value back (for example returns the password you just submitted), that value arrives in plaintext and — unless you configure `secretInjectionConfigs` (see [Masking response data](#masking-response-data)) — is stored verbatim in `status.response.body` and `status.cache.response`.

### Example: credentials in a header and request body

A credential in the `Authorization` header and a password in the request body are both sourced from Secrets via placeholders:

```yaml
apiVersion: http.crossplane.io/v1alpha2
kind: Request
metadata:
  name: manage-user
spec:
  providerConfigRef:
    name: http-conf
  forProvider:
    headers:
      Content-Type:
        - application/json
      Authorization:
        - "Bearer {{ auth:default:token }}"
    payload:
      baseUrl: http://test-server.default.svc.cluster.local/v1/users
      body: |
        {
          "username": "mock_user",
          "password": "{{ user-password:crossplane-system:password }}"
        }
    mappings:
      - action: CREATE
        url: .payload.baseUrl
        body: |
          {
            username: .payload.body.username,
            password: .payload.body.password
          }
```

With this configuration, `kubectl get request manage-user -o yaml` and the ArgoCD UI show only the `{{ ... }}` placeholders in `spec` and `status.requestDetails`, while the actual request contains the resolved credentials. If the API echoes the password back in its response, add a matching `secretInjectionConfig` (next section) — otherwise it lands in `status.response.body` in plaintext.

The complete, end-to-end example is [examples/sample/request.yaml](examples/sample/request.yaml); its masking behavior is asserted in e2e by [test/hooks/verify-secret-templating.sh](test/hooks/verify-secret-templating.sh).

### Masking response data

By default the **full HTTP response is stored verbatim** in `status.response.body`, `status.response.headers`, and `status.cache.response`. The provider cannot know which response fields are sensitive, so it masks nothing in the response unless you configure it to via `secretInjectionConfigs`. For each entry the provider:

1. Extracts the value at the configured JQ path (e.g. `.body.password`, `.headers."X-Secret-Header"[0]`) and writes it into a Kubernetes `Secret`.
2. Replaces that exact value in the stored response with a `{{ secret-name:secret-namespace:secret-key }}` placeholder.

Masking a response value is therefore a **side effect of extracting it into a Secret** — the two cannot be separated. A response field that has no corresponding `secretInjectionConfig` entry is **not** masked and remains in plaintext in `status`.

```yaml
spec:
  forProvider:
    secretInjectionConfigs:
      - secretRef:
          name: response-user-password
          namespace: default
        keyMappings:
          - secretKey: extracted-user-password
            responseJQ: .body.password   # this value is written to the Secret AND masked in status.response.body
```

See the [Request CRD documentation](resources-docs/request_docs.md) and [examples/sample/request.yaml](examples/sample/request.yaml) for the complete set of options (`keyMappings`, `missingFieldStrategy`, `setOwnerReference`, metadata).

## Usage

### DisposableRequest

Create a `DisposableRequest` resource to initiate a single-use HTTP interaction:

```yaml
apiVersion: http.crossplane.io/v1alpha2
kind: DisposableRequest
metadata:
  name: example-disposable-request
spec:
  # Add your DisposableRequest specification here
```

For more detailed examples and configuration options, refer to the [examples directory](examples/sample/).

### Request

Manage a resource through HTTP requests with a `Request` resource:

```yaml
apiVersion: http.crossplane.io/v1alpha2
kind: Request
metadata:
  name: example-request
spec:
  # Add your Request specification here
```

For more detailed examples and configuration options, refer to the [examples directory](examples/sample/).

### Namespaced Resources

For namespace-scoped resources, use the `http.m.crossplane.io` API group:

```yaml
apiVersion: http.m.crossplane.io/v1alpha2
kind: Request
metadata:
  name: example-namespaced-request
  namespace: my-namespace
spec:
  # Add your Request specification here
  providerConfigRef:
    name: my-namespaced-config
```

For namespaced examples and configuration options, refer to the [namespaced examples directory](examples/namespaced/).

## Developing locally

Run controller against the cluster:

```
make run
```

## Run tests

```
make test
make e2e
```

## Troubleshooting

If you encounter any issues during installation or usage, refer to the [troubleshooting guide](https://docs.crossplane.io/knowledge-base/guides/troubleshoot/) for common problems and solutions.
