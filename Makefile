BINARY_NAME=prism-mock
GO=go

build:
	$(GO) build -o $(BINARY_NAME) .

run-create: build
	./$(BINARY_NAME) -action create
	$(MAKE) clean

run-delete: build
	./$(BINARY_NAME) -action delete
	$(MAKE) clean

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)

deps:
	$(GO) mod download
