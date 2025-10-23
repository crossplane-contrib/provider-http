# provider-http

`provider-http` is a Crossplane Provider designed to facilitate sending HTTP requests as resources.

## Installation

To install `provider-http`, you have two options:

1. Using the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

   ```console
   crossplane xpkg install provider xpkg.upbound.io/crossplane-contrib/provider-http:v1.0.11
   ```

2. Manually creating a Provider by applying the following YAML:

   ```yaml
   apiVersion: pkg.crossplane.io/v1
   kind: Provider
   metadata:
     name: provider-http
   spec:
     package: "xpkg.upbound.io/crossplane-contrib/provider-http:v1.0.11"
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
