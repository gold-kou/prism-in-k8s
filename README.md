# What is this?
This tool allows you to easily create and delete mock resources for [stoplightio/prism](https://github.com/stoplightio/prism) within your Kubernetes cluster. Prism responds to requests based on your OpenAPI definition.

The tool not only creates a Pod for Prism, but also provisions related resources such as AWS ECR, Kubernetes Namespace, Deployment, Service, and VirtualService. By editing the VirtualService, you can introduce fixed delays using [fault injection](https://istio.io/latest/docs/tasks/traffic-management/fault-injection/), providing a more realistic mock environment.

This should be especially useful for load testing or developing microservice clients.

![Overview](https://github.com/user-attachments/assets/0666cc59-160e-441e-8f90-b7f2ab2a602a)

# Usage
## Step0. Requirements on Your Local Machine
- AWS and Kubernetes credentials
- Go
- kubectl
- Docker

## Step1. OpenAPI
Copy and paste your OpenAPI definition into `openapi.yaml`.

## Step2. Credentials
In my case, I use [awsp](https://github.com/johnnyopao/awsp) and [kubie](https://github.com/sbstp/kubie).

```bash
awsp <your_profile>
kubie ctx <your_context>
```

Regardless of the tools you use, please ensure your credentials are set.

## Step3. Set Parameters
Set the necessary parameters in `config/params.yaml`.

At a minimum, you need to set the following parameters:

- MicroserviceName
  - Your microservice name
- MicroserviceNamespace
  - Your microservice namespace

## Step4. Create Mock Resources
Run the following command:

```
$ make run-create
```

The following resources will be created:

- AWS
  - ECR
- Kubernetes
  - Namespace
  - Deployment
  - Service
  - VirtualService

## Step5. Modify VirtualService (Optional)
To make your mock more realistic, set `spec.http.fault.delay.fixedDelay` in the VirtualService resource.

```
$ kubectl edit VirtualService -n <your_namespace> <your_virtual_service_name>
```

## Step6. Load Testing
You can now perform load testing!

Make sure to specify the mock Service.

## Step7. Delete Mock Resources
When you're done, delete the mock resources with:

```
$ make run-delete
```

# Parameters

| Parameter Name                | Description                               | Default                        | Required |
|-------------------------------|-------------------------------------------|--------------------------------|----------|
| `microserviceName`           | Name of microservice                      | -                              | Yes      |
| `microserviceNamespace`      | Namespace of microservice                 | -                              | Yes      |
| `prismMockSuffix`           | Suffix for the mock service name          | `"-prism-mock"`                | Yes      |
| `prismPort`                  | Port number for Prism                     | `80`                           | Yes      |
| `prismCpu`                   | CPU request for Prism                     | `"500m"`                       | Yes      |
| `prismMemory`                | Memory request for Prism                  | `"512Mi"`                      | Yes      |
| `istioProxyCpu`             | CPU request for Istio                     | `"500m"`                       | Yes      |
| `istioProxyMemory`          | Memory request for Istio                  | `"512Mi"`                      | Yes      |
| `priorityClassName`         | Value of priorityClassName                | -                              | No       |
| `timeout`                     | Timeout for this tool                     | `10m`                          | Yes      |
| `ecrTags`                    | Pairs of ECR tag                          | -                              | No       |

sample:

```
microserviceName: "pet-store"
microserviceNamespace: "pet-store"
prismMockSuffix: "-prism-mock"
prismPort: 80
prismCpu: "1"
prismMemory: "1Gi"
istioProxyCpu: "500m"
istioProxyMemory: "512Mi"
priorityClassName: "high-priority"
timeout: "10m"
ecrTags:
  - key: "CostEnv"
    value: "stg"
  - key: "CostService"
    value: "pet-store"

```

# Development
## Unit Test
Please install the following tools before running the test:

- kind
- istio-ctl

```
$ make test
```

## Lint
Please install `golangci-lint` before running the lint:
https://golangci-lint.run/welcome/install/#local-installation

```
$ make lint
```
