export GOFLAGS = -mod=readonly
.PHONY: test

bin/ssm-env: *.go
	go build -o $@ .

test:
	go test -race $(shell go list ./... | grep -v /vendor/)
