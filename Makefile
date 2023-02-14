.DEFAULT_GOAL := bin/ssm-env

ARGS :=

version := $(shell git describe --tags --match 'v*')

.PHONY: run
run:
	CGO_ENABLED=0 go run -ldflags "-X main.version=$(version)" . $(ARGS)

bin/ssm-env: *.go
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(version)" -o $@ .

.PHONY: test
test:
	go test -race $(shell go list ./... | grep -v /vendor/)
