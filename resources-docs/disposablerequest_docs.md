# DisposableRequest

## Overview

The `DisposableRequest` resource is designed for initiating one-time HTTP requests. It allows you to specify the details of the HTTP request in the resource's specification, and the provider will execute the request. This is useful for scenarios where you need to trigger an HTTP action as part of your infrastructure provisioning or management process.


### Specification

Here is an example `DisposableRequest` resource definition:
```yaml
    apiVersion: http.crossplane.io/v1alpha2
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
        rollbackRetriesLimit: 3
        shouldLoopInfinitely: true
        nextReconcile: 3m
        expectedResponse: '.body.job_status == "success"'
        secretInjectionConfigs: 
          - secretRef:
              name: response-secret
              namespace: default
            keyMappings:
              - secretKey: extracted-data
                responseJQ: .body.reminder
              - secretKey: extracted-data-headers
                responseJQ: .headers.Try[0]
```

-  deletionPolicy: specifies what will happen to the underlying external when this managed resource is   deleted. in this case it should be set to "Orphan" the external resource.
-  url: The URL endpoint for the HTTP request.
-  method: The HTTP method for the request (e.g., GET, POST, PUT, DELETE).
-  body: Optional body of http request.
-  headers: Optional list of headers to include in the request.
-  waitTimeout: Optional timeout for the HTTP request.
-  rollbackRetriesLimit: Optional Limits the number of retries.
-  shouldLoopInfinitely: Optional (defaults to false) Indicates whether the reconciliation should loop indefinitely.
-  nextReconcile: Optional Specifies the duration after which the next reconcile should occur.
-  secretInjectionConfigs: Optional Configurations for secrets receiving patches from response data.

### Secrets Injection
The DisposableRequest resource supports injecting data from secrets into the request's body and headers using the following syntax: {{ name:namespace:key }} (supported for body and headers only).

### Status
The status field of the `DisposableRequest` resource will provide information about the execution status and results of the HTTP request.

Example `DisposableRequest` status:
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
