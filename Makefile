.PHONY: build install clean b i c

BIN_DIR := ./bin

ifeq ($(OS),Windows_NT)
    RAW_HOME_DIR := $(USERPROFILE)
    HOME_DIR := $(subst \,/,$(RAW_HOME_DIR))
else
    HOME_DIR := $(HOME)
endif

DOT_PSW_DIR := $(HOME_DIR)/.psw
CONFIG_FILE := pswcfg.toml

build:
	go mod tidy
	go build -o $(BIN_DIR)/psw
	go build -o $(BIN_DIR)/clipclean ./clipclean/
	@if [ ! -f $(BIN_DIR)/$(DOT_PSW_DIR) ]; then \
		echo "$(CONFIG_FILE) does not exist in $(DOT_PSW_DIR). Copying..."; \
		cp $(CONFIG_FILE) $(DOT_PSW_DIR)/; \
	else \
		echo "$(CONFIG_FILE) already exists in $(DOT_PSW_DIR). Skipping copy."; \
	fi

install: build
	go install
	go install ./clipclean

# Clean up build artifacts
clean:
	@rm -rf $(BIN_DIR)/*
	
# Shortcuts
b: build
i: install
c: clean
