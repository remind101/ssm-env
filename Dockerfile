FROM golang:1.9 AS builder

COPY vendor /go/src

WORKDIR /go/src/github.com/remind101/ssm-env
COPY . .

RUN CGO_ENABLED=0 make


FROM alpine

RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/github.com/remind101/ssm-env/bin/ssm-env /bin/ssm-env

ENTRYPOINT ["ssm-env"]