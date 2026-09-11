.PHONY: build test lint run clean help
BINARY        := otelcol-ethernetip
BUILDER_CFG   := builder-config.yaml
COLLECTOR_CFG := config.yaml

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
build: ## Build the custom OTel Collector binary
	builder --config $(BUILDER_CFG)
test: ## Run all tests
	go test ./...
lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...
run: ## Run the OTel Collector (requires build first)
	./bin/$(BINARY) --config $(COLLECTOR_CFG)
clean: ## Remove build artifacts
	rm -rf bin/
tidy: ## Tidy go modules
	go mod tidy
