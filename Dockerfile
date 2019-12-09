FROM circleci/golang:1.11.6 as scratch

ADD . /go/src/github.com/remind101/ssm-env

WORKDIR /go/src/github.com/remind101/ssm-env

USER root

ENV GO111MODULE=on
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

RUN make bin/ssm-env

FROM alpine:3

RUN apk add --no-cache ca-certificates
COPY --from=scratch /go/src/github.com/remind101/ssm-env/bin/ssm-env /usr/bin/ssm-env

ENTRYPOINT ["/usr/bin/ssm-env", "-no-fail", "-with-decryption"]
