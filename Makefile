.PHONY: build install clean test b i c t

BIN_DIR := ./bin
CONFIG_FILE_TEMPLATE := pswcfg-template.toml
CONFIG_FILE := pswcfg.toml
VERSION := $(shell cat VERSION | tr -d '[:space:]')

build:
	go mod tidy
	go build -ldflags="-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'" -o $(BIN_DIR)/psw ./cmd/psw
	go build -o $(BIN_DIR)/clipclean ./cmd/clipclean
	@if [ ! -f $(BIN_DIR)/$(CONFIG_FILE) ]; then \
		echo "$(CONFIG_FILE) does not exist in $(BIN_DIR). Copying..."; \
		cp $(CONFIG_FILE_TEMPLATE) $(BIN_DIR)/$(CONFIG_FILE); \
	else \
		echo "$(CONFIG_FILE) already exists in $(BIN_DIR). Skipping copy."; \
	fi

install: build
	go install -ldflags="-X 'github.com/ylniss/psw/internal/cli.Version=$(VERSION)'" ./cmd/psw
	go install ./cmd/clipclean

# Clean up build artifacts
clean:
	@rm -rf $(BIN_DIR)/*

# Run integration tests
test:
	go test -v ./tests/...

# Shortcuts
b: build
i: install
c: clean
t: test
