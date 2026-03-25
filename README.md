# What is this?
This tool simplifies the creation and management of [stoplightio/prism](https://github.com/stoplightio/prism) mock resources within a Kubernetes cluster. Prism serves requests based on your OpenAPI specifications.

Beyond just creating Prism Pods, this tool automates the provisioning of:

- AWS ECR
- Kubernetes Namespace, Deployment, and Service
- Istio VirtualService (allowing for advanced features like fault injection)

By configuring the VirtualService, you can introduce fixed delays using fault injection to create a more realistic testing environment.

# Prerequisites
Before using this tool, ensure you have the following installed and configured:

- Tools: Go, kubectl, and Docker.
- Credentials: Valid AWS and Kubernetes context/credentials.
  - Note: Ensure your current context is set to the target cluster where you want to deploy the mock resources.

# Usage
## Step 1. OpenAPI
Place your OpenAPI definition in `app/openapi.yaml`.

## Step 2. Set Parameters
Define the necessary parameters in `config/params.yaml`. At a minimum, the following are required:

- `microserviceName`
  - Your microservice name
- `microserviceNamespace`
  - Your microservice namespace

## Step 3. Deploy Mock Resources
Run the following command to provision the Prism Pod and its associated resources (ECR, Namespace, Deployment, Service, VirtualService):

```
$ make run-create
```

## Step 4. (Optional) Advanced Configuration
To simulate more realistic scenarios, you can modify the Istio VirtualService to include fault injections, such as fixed delays (`spec.http.fault.delay.fixedDelay`).

```
$ kubectl edit VirtualService -n <your_namespace> <your_virtual_service_name>
```

# Cleanup
```
$ make run-delete
```

# Parameters

| Parameter Name                | Description                               | Default                        | Required |
|-------------------------------|-------------------------------------------|--------------------------------|----------|
| `microserviceName`            | Name of microservice                      | `sample`                       | Yes      |
| `microserviceNamespace`       | Namespace of microservice                 | `sample`                       | Yes      |
| `prismMockSuffix`             | Suffix for the mock service name          | `"-prism-mock"`                | Yes      |
| `timeout`                     | Timeout for this tool                     | `10m`                          | No       |
| `prismPort`                   | Port number for Prism                     | `80`                           | No       |
| `prismCpu`                    | CPU request for Prism                     | `"500m"`                       | No       |
| `prismMemory`                 | Memory request for Prism                  | `"512Mi"`                      | No       |
| `istioMode`                   | Whether to use istio                      | `true`                         | No       |
| `istioProxyCpu`               | CPU request for Istio                     | `"500m"`                       | No       |
| `istioProxyMemory`            | Memory request for Istio                  | `"512Mi"`                      | No       |
| `priorityClassName`           | Value of priorityClassName                | -                              | No       |
| `ecrTags`                     | Pairs of ECR tag                          | -                              | No       |

sample:

```
microserviceName: "sample"
microserviceNamespace: "sample"
prismMockSuffix: "-prism-mock"
istioMode: false
priorityClassName: "high-priority"
ecrTags:
  - key: "CostEnv"
    value: "stg"
  - key: "CostService"
    value: "pet-store"
```

# For developers
## Testing
Ensure the following tools are installed before running the tests:

- **kind**
- **istioctl**

### Unit tests
```bash
$ make test-go
```

Runs the Go unit tests.

### End-to-end (E2E) tests
```
$ make test-e2e
```

This command automates the following steps:

1. Sets up a kind cluster.
2. Deploys mock resources to the cluster.
3. Launches a curl pod.
4. Executes a curl command from the pod to the mock service.
5. Verifies the response.

Note: This process may take a few minutes for all resources to become ready.

## Linting
Install golangci-lint before running the linter:
https://golangci-lint.run/welcome/install/#local-installation


```
$ make lint
```
