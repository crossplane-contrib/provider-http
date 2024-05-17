# provider-http

`provider-http` is a Crossplane Provider designed to facilitate sending HTTP requests as resources.


## Installation

To install `provider-http`, you have two options:

1. Using the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

    ```console
    kubectl crossplane install provider xpkg.upbound.io/crossplane-contrib/provider-http:v1.0.0
    ```

2. Manually creating a Provider by applying the following YAML:

    ```yaml
    apiVersion: pkg.crossplane.io/v1
    kind: Provider
    metadata:
      name: provider-http
    spec:
      package: "xpkg.upbound.io/crossplane-contrib/provider-http:v0.2.0"
    ```


## Supported Resources

`provider-http` supports the following resources:

- **DisposableRequest:** Initiates a one-time HTTP request. See [DisposableRequest CRD documentation](resources-docs/disposablerequest_docs.md).
- **Request:** Manages a resource through HTTP requests. See [Request CRD documentation](resources-docs/request_docs.md).

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


## Developing locally

Run controller against the cluster:
```
make run
```


## Troubleshooting
If you encounter any issues during installation or usage, refer to the [troubleshooting guide](https://docs.crossplane.io/knowledge-base/guides/troubleshoot/) for common problems and solutions.