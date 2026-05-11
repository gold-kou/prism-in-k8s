# What is this?
This tool simplifies the creation and management of [stoplightio/prism](https://github.com/stoplightio/prism) mock resources within a Kubernetes cluster. Prism serves requests based on your OpenAPI specifications.

Beyond just creating Prism Pods, this tool automates the provisioning of:

- AWS ECR
- Kubernetes Namespace, Deployment, and Service
- Istio VirtualService (allowing for advanced features like fault injection)

By configuring the VirtualService, you can introduce fixed delays using fault injection to create a more realistic testing environment.

# Prerequisites
Before using this tool, ensure you have the following installed and configured:

- Tools: kubectl and Docker.
- Credentials: Valid AWS and Kubernetes context/credentials.
  - Note: Ensure your current context is set to the target cluster where you want to deploy the mock resources.

# Installation
Download the binary for your platform from the [GitHub Releases](https://github.com/gold-kou/prism-in-k8s/releases) page. Extract the archive and place the `prism-in-k8s` binary somewhere on your `PATH` (e.g. `/usr/local/bin`).

Alternatively, use the published container image:

```
ghcr.io/gold-kou/prism-in-k8s:latest
```

## macOS users
The released binary is not signed with an Apple Developer ID, so on first run macOS Gatekeeper may block it with a message like *"Apple could not verify 'prism-in-k8s' is free of malware..."*. Remove the quarantine attribute to allow execution:

```
xattr -d com.apple.quarantine /path/to/prism-in-k8s
```

Alternatively, after the block dialog appears, open *System Settings > Privacy & Security* and click *Open Anyway*.

# Usage
## Step 1. OpenAPI
Place your OpenAPI definition file at `./openapi.yaml` in your working directory, or set the `OPENAPI_PATH` environment variable to the path of your OpenAPI file.

## Step 2. Set Parameters
Place your parameters file at `./params.yaml` in your working directory, or set the `PARAMS_CONFIG_PATH` environment variable to the path of your parameters file. At a minimum, the following are required:

- `microserviceName`
  - Your microservice name
- `microserviceNamespace`
  - Your microservice namespace

## Step 3. Deploy Mock Resources
Run the following command to provision the Prism Pod and its associated resources (ECR, Namespace, Deployment, Service, VirtualService):

```
$ prism-in-k8s -create
```

# Cleanup
```
$ prism-in-k8s -delete
```

# Parameters

| Parameter Name                | Description                               | Default                        | Required |
|-------------------------------|-------------------------------------------|--------------------------------|----------|
| `microserviceName`            | Name of microservice                      | -                              | Yes      |
| `microserviceNamespace`       | Namespace of microservice                 | -                              | Yes      |
| `prismMockSuffix`             | Suffix for the mock service name          | -                              | Yes      |
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
| `dockerBuildPlatform`         | Target platform passed to `docker build --platform` for the Prism image. Must be `linux/amd64` or `linux/arm64`. Set to `linux/arm64` when the destination cluster runs on Graviton (arm64) nodes. | `linux/amd64`                  | No       |

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
Additional requirement when building from source:

- Go

## Running from source
Clone the repository and use the provided Makefile targets, which build the binary locally and pass the in-repo `config/params.yaml` and `app/openapi.yaml` via environment variables:

```
$ make run-create
$ make run-delete
```

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
