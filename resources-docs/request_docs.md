# Request

## Overview

The `Request` resource is designed for managing a resource through HTTP requests. It allows you to define how the provider should interact with the remote system by specifying HTTP requests for create, update, and delete operations.


### Specification
Here is an example `Request` resource definition:

  ```yaml
  apiVersion: http.crossplane.io/v1alpha1
  kind: Request
  metadata:
    name: user-dan
  spec:
    forProvider:
      headers:
        Content-Type:
          - application/json
      payload:
        baseUrl: "http://host.docker.internal:5000/users"
        body: |
          {
            "username": "Dan"
          }
      mappings:
        - method: "POST"
          body: |
            {
              username: .payload.body.name, 
              managedby: "crossplane"
            }
          url: .payload.baseUrl
        - method: "GET"
          url: (.payload.baseUrl + "/" + (.response.body.id|tostring)) 
        - method: "PUT"
          body: |
            {
              username: .payload.body.name, 
            }
          url: (.payload.baseUrl + "/" + (.response.body.id|tostring)) 
        - method: "DELETE"
          url: (.payload.baseUrl + "/" + (.response.body.id|tostring)) 
  ```

- headers: Default HTTP request headers.
- payload: Customizable values for HTTP requests, with jq query support [jq Documentation](https://lzone.de/cheat-sheet/jq).
- mappings: List of mappings, each specifying the HTTP method, URL, and optional request body.


## PUT Mapping - Desired State
The PUT mapping represents your desired state. The body in this mapping should be contained in the GET response. If it's not, a PUT request will be sent with the according body.

Example PUT mapping:

  ```yaml
  apiVersion: http.crossplane.io/v1alpha1
    ...
      mappings:
        ...
        - method: "PUT"
          body: |
            {
              username: .payload.body.name, 
            }
          url: (.payload.baseUrl + "/" + (.response.body.id|tostring)) 
  ```


## Status
The status field of the `Request` resource provides information about the execution status and results of the HTTP requests.

Example `Request` status:
  ```yaml
  status:
  conditions:
    ...
  response:
    body: >-
      {
        "id":"65565b69681e0b47dcea4464",
        "todo_name":"Do Laundry",
        "reminder":"Every 1 hour",
        "responsible":"Dan"
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


### Usage

Here's an example of using variables from the response:

  ```yaml
  apiVersion: http.crossplane.io/v1alpha1
  kind: Request
  metadata:
    name: user-dan
  spec:
    forProvider:
      ...
      mappings:
        - method: "GET"
          url: (.payload.baseUrl + "/" + (.response.body.id|tostring)) 
      ...
  ```

**Important:** Ensure that every response includes the required parameters, such as `body.id` in this example. This is crucial because each new response overrides the previous one, and missing parameters may lead to unexpected behavior in subsequent requests.
