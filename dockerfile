FROM golang:1.14.2-buster

WORKDIR /go/src


COPY . .

ENV GO111MODULE=on
RUN go mod download
RUN  make bin/ssm-env
