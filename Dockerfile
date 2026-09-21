ARG VALKEY_VERSION
ARG VALKEY_VARIANT

FROM golang:1.27.1 AS builder

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum main.go ./

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /launcher main.go

FROM valkey/valkey${VALKEY_VARIANT}:${VALKEY_VERSION:-latest}

LABEL maintainer="mix3"

COPY --from=builder /launcher /launcher

COPY LICENSE /LICENSE

RUN mkdir -p /valkey-data

EXPOSE 7000 7001 7002 7003 7004 7005 7006 7007 5000 5001 5002

ENTRYPOINT ["/launcher"]
