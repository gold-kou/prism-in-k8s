BINARY_NAME=prism-mock
GO=go
KIND_CLUSTER_NAME=prism-test-cluster
KIND_CONFIG=kind-config.yaml

build:
	$(GO) build -o $(BINARY_NAME) .

run-create: build
	./$(BINARY_NAME) -create
	$(MAKE) clean

run-delete: build
	./$(BINARY_NAME) -delete
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
        docker build -f Dockerfile.prism -t my-local-image:latest .; \
        kind load docker-image my-local-image:latest --name $(KIND_CLUSTER_NAME); \
        istioctl install --set profile=default -y; \
    else \
        echo "Cluster $(KIND_CLUSTER_NAME) already exists"; \
    fi

kind-down:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

test: kind-up
	@trap '$(MAKE) kind-down' EXIT; \
	$(GO) test ./... -v -shuffle=on -p 1
