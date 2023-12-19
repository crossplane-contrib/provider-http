# DisposableRequest

## Overview

The `DisposableRequest` resource is designed for initiating one-time HTTP requests. It allows you to specify the details of the HTTP request in the resource's specification, and the provider will execute the request. This is useful for scenarios where you need to trigger an HTTP action as part of your infrastructure provisioning or management process.


### Specification

Here is an example `DisposableRequest` resource definition:
```yaml
    apiVersion: http.crossplane.io/v1alpha1
    kind: DisposableRequest
    metadata:
      name: example-disposable-request
    spec:
      deletionPolicy: Orphan
      forProvider:
        url: https://enwgarmh79yh.x.pipedream.net/
        method: POST
        body: '{"key": "value"}'
        headers:
          Content-Type:
            - application/json
          Authorization:
            - Bearer myToken
```

-  deletionPolicy: specifies what will happen to the underlying external when this managed resource is   deleted. in this case it should be set to "Orphan" the external resource.
-  url: The URL endpoint for the HTTP request.
-  method: The HTTP method for the request (e.g., GET, POST, PUT, DELETE).
-  body: Optional body of http request.
-  headers: Optional list of headers to include in the request.
-  waitTimeout: Optional timeout for the HTTP request.
-  rollbackLimit: Optional limit for retries.


### Status
The status field of the `DisposableRequest` resource will provide information about the execution status and results of the HTTP request.

Example `DesposibleRequest` status:
  ```yaml
  status:
    conditions:
      ...
    requestDetails:
      ...
    response:
      body: >-
        {
          "id":"65565b69681e0b47dcea4464",
          "key":"value"
        }
      headers:
        Content-Length:
          - '104'
        Content-Type:
          - application/json
        Date:
          - Thu, 16 Nov 2023 18:11:53 GMT
        Server:
          - uvicorn
      statusCode: 200
  ```
