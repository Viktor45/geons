FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine3.23 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN set -eux; \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -trimpath -ldflags="-s -w" -o /out/geons ./geons

FROM --platform=$BUILDPLATFORM alpine:3.23 AS certs

RUN apk add --no-cache ca-certificates \
    && mkdir -p /data

FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/geons /usr/local/bin/geons

EXPOSE 5300

ENTRYPOINT ["geons"]
