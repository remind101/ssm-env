.PHONY: test

# Disable CGO by default.
export CGO_ENABLED=0

bin/ssm-env: *.go
	# Enforce static linking, and copy the timezone data into the runtime so we
	# can in places like scratch images.
	go build -ldflags '-extldflags "-static"' -o $@ .

test:
	# Testing for race conditions requires CGO be enabled.
	CGO_ENABLED=1 go test -race $(shell go list ./... | grep -v /vendor/)
