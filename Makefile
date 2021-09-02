.PHONY: test

bin/ssm-env: *.go
	CGO_ENABLED=0 go build -o $@ .

test:
	go test -race $(shell go list ./... | grep -v /vendor/)
