# What is this?
This tool allows you to easily create and delete mock resources for [stoplightio/prism](https://github.com/stoplightio/prism) within your Kubernetes cluster. Prism responds to requests based on your OpenAPI definition.

The tool not only creates a Pod for Prism, but also provisions related resources such as AWS ECR, Kubernetes Namespace, Deployment, Service, and VirtualService. By editing the VirtualService, you can introduce fixed delays using [fault injection](https://istio.io/latest/docs/tasks/traffic-management/fault-injection/), providing a more realistic mock environment.

This should be especially useful for load testing or developing microservice clients.

![Overview](https://github.com/user-attachments/assets/0666cc59-160e-441e-8f90-b7f2ab2a602a)

# Step-by-Step Usage
## Requirements on Your Local Machine
- AWS and Kubernetes credentials
- Go
- AWS CLI
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
Set the necessary parameters in `params.go`.

At a minimum, you need to set the following parameters:

- microserviceName
  - Your microservice name
- microserviceNamespace
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
You can set these parameters in `params.go`.

| Parameter Name                | Description                               | default                        | required |
|-------------------------------|-------------------------------------------|--------------------------------|----------|
| microserviceName              | Name of microservice                      | ""                             | Yes      |
| microserviceNamespace         | Namespace of microservice                 | ""                             | Yes      |
| prismMockSuffix               | Suffix for the mock service name          | "-prism-mock"                  | Yes      |
| prismPort                     | Port number for Prism                     | "80"                           | Yes      |
| prismCPU                      | CPU request for Prism                     | "1"                            | Yes      |
| prismMemory                   | Memory request for Prism                  | "1Gi"                          | Yes      |
| istioProxyCPU                 | CPU request of istio                      | "500m"                         | Yes      |
| istioProxyMemory              | Memory request for istio                  | "512Mi"                        | Yes      |
| timeout                       | Timeout for this tool                     | 10 * time.Minute               | Yes      |
| ecrTagEnv                     | Value of the CostEnv tag of ECR           | "stg"                          | No       |
