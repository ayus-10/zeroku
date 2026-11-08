BIN_DIR := bin
API_BIN := $(BIN_DIR)/api
CLI_BIN := $(BIN_DIR)/cli

all: build

build: build-api build-cli

build-api:
	@mkdir -p $(BIN_DIR)
	@echo "Building API..."
	@go build -o $(API_BIN) ./cmd/api

build-cli:
	@mkdir -p $(BIN_DIR)
	@echo "Building CLI..."
	@go build -o $(CLI_BIN) ./cmd/cli

run: run-api run-cli

run-api:
	@echo "Running API..."
	@$(API_BIN)

run-cli:
	@echo "Running CLI..."
	@$(CLI_BIN)

clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
