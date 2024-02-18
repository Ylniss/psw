.PHONY: build install clean b i c

BIN_DIR := ./bin

build:
	go build -o $(BIN_DIR)
	go build -o $(BIN_DIR) ./clipclean/

install:
	go install
	go install ./clipclean

# Clean up build artifacts
clean:
	@rm -rf $(BIN_DIR)/*
	
# Shortcuts
b: build
i: install
c: clean
