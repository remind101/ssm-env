name := ssm-env
version := $(shell git describe --tags --match 'v*' HEAD)
ldflags := -X main.version=$(VERSION)

ssm-env: *.go
	CGO_ENABLED=0 go build -ldflags "$(ldflags)" -o $@ .

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	@test -z $(shell gofmt -l . | tee /dev/stderr) || { echo "files above are not go fmt"; exit 1; }
	go vet ./...

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	rm -rf ssm-env
	rm -rf _dist

dist_os_arch := \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64

dists := $(dist_os_arch:%=_dist/$(name)-%)

.PHONY: dist $(dists)
dist: $(dists)

$(dists): _dist/$(name)-%:
	mkdir -p _dist
	CGO_ENABLED=0 \
	GOOS=$(word 1,$(subst -, ,$*)) \
	GOARCH=$(word 2,$(subst -, ,$*)) \
	go build -ldflags "$(ldflags)" -o $@ .
