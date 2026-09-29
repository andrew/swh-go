SPEC_URL ?= https://raw.githubusercontent.com/andrew/swh-openapi/main/openapi.yaml

.PHONY: all generate spec test lint tidy

all: generate test

# Refresh the vendored description from the spec repository.
spec:
	curl -fsSL "$(SPEC_URL)" -o openapi.yaml

generate:
	go tool oapi-codegen -config oapi-codegen.yaml openapi.yaml

test:
	go test ./...

lint:
	golangci-lint run ./...
	govulncheck ./...

tidy:
	go mod tidy
