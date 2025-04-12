# Makefile for spaneventtologconnector

GO ?= go

.PHONY: all
all: fmt lint test build

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	# Run tests only for the current package to potentially avoid dependency issues
	$(GO) test -v -race -coverprofile=coverage.out .

.PHONY: build
build:
	# No specific build target needed for a library component,
	# but this target can be used for future build steps.
	@echo "Build step placeholder"

.PHONY: clean
clean:
	rm -f coverage.out
