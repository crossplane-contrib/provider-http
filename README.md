# provider-http

`provider-http` is a Crossplane Provider that enables sending one-time HTTP requests as disposable resources.

## Installation

If you would like to install `provider-http` without modifications, you can use the Crossplane CLI in a Kubernetes cluster where Crossplane is installed:

```console
kubectl crossplane install provider crossplanecontrib/provider-http:master
```

You can also manually install `provider-http` by creating a Provider directly:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-http
spec:
  package: "crossplanecontrib/provider-http:master"
```

## Usage
Currently, `provider-http` supports only one-time requests using the `DisposableRequest` Custom Resource Definition (CRD). This enables you to initiate a single-use HTTP interaction by creating a `DisposableRequest` resource.

## Example

Take a look at `examples/sample/desposiblerequest.yaml`

