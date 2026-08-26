# qgh — prints the current repo's GitHub URL and your open PRs against it

BINARY  := qgh
BIN_DIR := build

.DEFAULT_GOAL := help

.PHONY: help test build install clean

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

test: ## Run all unit tests
	go test ./...

build: ## Build the binary to ./build/qgh
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

install: build ## Install the binary to /usr/local/bin (sudo)
	sudo install $(BIN_DIR)/$(BINARY) /usr/local/bin/
	@echo ""
	@echo "qgh installed successfully!"

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
