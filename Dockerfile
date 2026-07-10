FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY main.go ./
RUN go mod download

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN set -eux; \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -trimpath -ldflags="-s -w" -o /out/geons .

FROM --platform=$BUILDPLATFORM alpine:3.24 AS certs

RUN apk -U upgrade && apk add --no-cache ca-certificates \
    && mkdir -p /app /data

FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=certs /app /app
COPY --from=certs /data /data
COPY --from=build /out/geons /app/geons

WORKDIR /app

ENV GEONS_CONFIG=/data/config.yaml

EXPOSE 5300/udp

ENTRYPOINT ["/app/geons"]