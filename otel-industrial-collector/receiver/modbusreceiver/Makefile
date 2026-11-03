.PHONY: build test lint simulate run clean generate help

BINARY        := otelcol-modbus
SIMULATOR     := simulator
BUILDER_CFG   := builder-config.yaml
COLLECTOR_CFG := config.yaml
TOOLS_MOD     := ../../internal/tools/go.mod
MDATAGEN      := go tool -modfile=$(TOOLS_MOD) go.opentelemetry.io/collector/cmd/mdatagen

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the custom OTel Collector binary
	builder --config $(BUILDER_CFG)

test: ## Run all tests
	go test ./...

generate: ## Regenerate mdatagen code and docs from metadata.yaml (pinned in internal/tools)
	$(MDATAGEN) metadata.yaml

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

simulate: ## Build and run the Modbus TCP simulator on localhost:5020
	go build -o $(SIMULATOR) ./cmd/simulator/
	./$(SIMULATOR) -addr localhost:5020

run: ## Run the OTel Collector (requires build first)
	./bin/$(BINARY) --config $(COLLECTOR_CFG)

clean: ## Remove build artifacts
	rm -f $(SIMULATOR)
	rm -rf bin/

tidy: ## Tidy go modules
	go mod tidy