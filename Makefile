VERSION := $(if $(VERSION),$(VERSION),$(shell git rev-parse HEAD))
LDFLAGS := -X main.Version=$(VERSION)

export CGO_ENABLED=0

.PHONY: bin/ssm-env
bin/ssm-env:
	go build -ldflags "$(LDFLAGS)" -o $@ .

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	@test -z $(shell gofmt -l . | tee /dev/stderr) || { echo "files above are not go fmt"; exit 1; }
	go vet ./...
	golangci-lint run
