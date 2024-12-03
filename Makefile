BINARY_NAME=prism-mock
GO=go
KIND_CLUSTER_NAME=prism-test-cluster
KIND_CONFIG=kind-config.yaml

build:
	$(GO) build -o $(BINARY_NAME) .

run-create: build
	PARAMS_CONFIG_PATH=config/params.yaml ./$(BINARY_NAME) -create
	$(MAKE) clean

run-delete: build
	PARAMS_CONFIG_PATH=config/params.yaml ./$(BINARY_NAME) -delete
	$(MAKE) clean

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)

deps:
	$(GO) mod download
	$(GO) mod tidy

kind-up:
	@if ! kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
        kind create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CONFIG); \
        kubectl config use-context kind-$(KIND_CLUSTER_NAME); \
        docker build --platform linux/amd64 -f Dockerfile.prism -t my-local-image:latest .; \
        kind load docker-image my-local-image:latest --name $(KIND_CLUSTER_NAME); \
        istioctl install --set profile=default -y; \
    else \
        echo "Cluster $(KIND_CLUSTER_NAME) already exists"; \
    fi

kind-down:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

test: kind-up
	@trap '$(MAKE) kind-down' EXIT; \
	PARAMS_CONFIG_PATH=../../config/params.yaml $(GO) test ./... -v -shuffle=on -p 1
	$(MAKE) lint

test-ci: kind-up
	PARAMS_CONFIG_PATH=../../config/params.yaml $(GO) test ./... -v -shuffle=on -p 1

lint:
	golangci-lint run
