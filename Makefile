BINARY_NAME=prism-mock
GO=go
KIND_CLUSTER_NAME=prism-test-cluster
KIND_CONFIG=kind-config.yaml

build:
	$(GO) build -o $(BINARY_NAME) .

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)

run-create: build
	PARAMS_CONFIG_PATH=config/params.yaml ./$(BINARY_NAME) -create
	$(MAKE) clean

run-create-test: build
	PARAMS_CONFIG_PATH=config/params.yaml ./$(BINARY_NAME) -create -test
	$(MAKE) clean

run-delete: build
	PARAMS_CONFIG_PATH=config/params.yaml ./$(BINARY_NAME) -delete
	$(MAKE) clean

kind-up:
	@if ! kind get clusters | grep -q $(KIND_CLUSTER_NAME); then \
        kind create cluster --name $(KIND_CLUSTER_NAME) --config $(KIND_CONFIG); \
        kubectl config use-context kind-$(KIND_CLUSTER_NAME); \
        docker build --platform linux/amd64 -f Dockerfile.prism -t my-local-image:v1 .; \
        kind load docker-image my-local-image:v1 --name $(KIND_CLUSTER_NAME); \
        istioctl install --set profile=default -y; \
    else \
        echo "Cluster $(KIND_CLUSTER_NAME) already exists"; \
    fi

kind-down:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

test-go: kind-up
	@trap '$(MAKE) kind-down' EXIT; \
	PARAMS_CONFIG_PATH=../../config/params.yaml $(GO) test ./... -v -shuffle=on -p 1

# use run-create-test to create only k8s resources
# 2 curl requests because the first one doesn't work
test-e2e: kind-up
	@trap '$(MAKE) kind-down' EXIT; \
	$(MAKE) run-create-test; \
	kubectl run curl-test --image=yauritux/busybox-curl:latest --restart=Never -- sleep 3600; \
	echo "Waiting 60s for being ready..."; \
	sleep 60; \
	kubectl exec curl-test -- curl -i -v -m 1 http://sample-prism-mock.sample-prism-mock.svc.cluster.local/users; \
	kubectl exec curl-test -- curl -i -v -m 1 http://sample-prism-mock.sample-prism-mock.svc.cluster.local/users > curl_result.txt; \
	grep "HTTP/1.1 200 OK" curl_result.txt || exit 1; \
	rm curl_result.txt

lint:
	golangci-lint run

deps:
	$(GO) mod download
	$(GO) mod tidy
