.PHONY: build clean install

build:
	go build -o ../bin/
	go build -o ../bin/ ./clipclean/

install:
	go install
	go install ./clipclean

# Clean up build artifacts
clean:
	@rm -rf ../bin/
