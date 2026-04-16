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
Prepare your OpenAPI definition file. You can either:
- Place it at `app/openapi.yaml` (default), or
- Set the `OPENAPI_PATH` environment variable to the path of your OpenAPI file

## Step 2. Set Parameters
Define the necessary parameters in `config/params.yaml` and set `PARAMS_CONFIG_PATH` to its path. At a minimum, the following are required:

- `microserviceName`
  - Your microservice name
- `microserviceNamespace`
  - Your microservice namespace

## Step 3. Deploy Mock Resources
Run the following command to provision the Prism Pod and its associated resources (ECR, Namespace, Deployment, Service, VirtualService):

```
$ make run-create
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
| `istioMode`                   | Whether to use istio                      | `false`                        | No       |
| `virtualServiceRoutes`        | HTTP routes for the Istio VirtualService. Each entry supports `name` (required), `uriPrefix`, `method` (GET/HEAD/POST/PUT/PATCH/DELETE/CONNECT/OPTIONS/TRACE), `delayNanos` (>=0), and `delayPercentage` (0-100). A trailing catch-all `default` route is appended automatically. Only applied when `istioMode` is `true`. | -                              | No       |
| `istioProxyCpu`               | CPU request for Istio                     | `"500m"`                       | No       |
| `istioProxyMemory`            | Memory request for Istio                  | `"512Mi"`                      | No       |
| `priorityClassName`           | Value of priorityClassName                | -                              | No       |
| `nodeAffinity`                | Node affinity match expressions (key, operator, values). Operators: In, NotIn, Exists, DoesNotExist, Gt, Lt | -                              | No       |
| `podAntiAffinityTopologyKey`  | Topology key for pod anti-affinity (e.g., `kubernetes.io/hostname`). Prevents multiple pods of the same service from running on the same topology | -                              | No       |
| `ecrTags`                     | Pairs of ECR tag                          | -                              | No       |

sample:

```
microserviceName: "sample"
microserviceNamespace: "sample"
prismMockSuffix: "-prism-mock"
istioMode: true
virtualServiceRoutes:
  - name: "example1"
    uriPrefix: "/example1/"
    method: "GET"
    delayNanos: 100000000
    delayPercentage: 100
  - name: "example2"
    uriPrefix: "/example2/"
    method: "POST"
priorityClassName: "high-priority"
nodeAffinity:
  - key: "karpenter.sh/nodepool"
    operator: "In"
    values:
      - "default"
podAntiAffinityTopologyKey: "kubernetes.io/hostname"
ecrTags:
  - key: "CostEnv"
    value: "stg"
  - key: "CostService"
    value: "pet-store"
```

Note: `podAntiAffinityTopologyKey` uses `requiredDuringSchedulingIgnoredDuringExecution` with the `app` label of the deployed service as the label selector. This means it prevents multiple pods of the same Prism mock service from being scheduled on the same topology (e.g., same node).

# For developers
## Testing
Ensure the following tools are installed before running the tests:

- **[kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)**
- **[istioctl](https://istio.io/latest/docs/setup/getting-started/#download)**

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
