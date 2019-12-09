ARG CGO_ENABLED

FROM circleci/golang:1.11.6 as scratch

ADD . /go/src/github.com/remind101/ssm-env

WORKDIR /go/src/github.com/remind101/ssm-env

USER root

ENV GO111MODULE=on
ENV GOOS=linux
ENV GOARCH=amd64
ARG CGO_ENABLED

RUN make bin/ssm-env
