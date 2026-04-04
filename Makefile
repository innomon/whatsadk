.PHONY: all build clean test lint

# Binary names
BINARY_GATEWAY=gateway
BINARY_KEYGEN=keygen
BINARY_MCP=whatsadk-mcp

# Directories
BIN_DIR=bin
CMD_DIR=cmd

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

all: build

build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_GATEWAY) ./$(CMD_DIR)/gateway
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_KEYGEN) ./$(CMD_DIR)/keygen
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_MCP) ./$(CMD_DIR)/mcp

build-mcp:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_MCP) ./$(CMD_DIR)/mcp

clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)

test:
	$(GOTEST) -v ./...

lint:
	golangci-lint run
