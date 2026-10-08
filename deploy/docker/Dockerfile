# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.8
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine3.24 AS build

WORKDIR /src
ENV GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/ecoflow-ble-nutd ./cmd/ecoflow-ble-nutd

FROM alpine:3.24 AS runtime
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/ecoflow-ble-nutd /usr/local/bin/ecoflow-ble-nutd
COPY LICENSE /usr/share/licenses/ecoflow-ble-nutd/LICENSE

LABEL org.opencontainers.image.title="ecoflow-ble-nutd" \
      org.opencontainers.image.description="EcoFlow BLE to NUT daemon" \
      org.opencontainers.image.source="https://github.com/xkhronoz/ecoflow-ble-nutd"

ENV DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket \
    TZ=UTC
USER 65532:65532
EXPOSE 3493/tcp 8080/tcp
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD printf 'VER\n' | nc -w 3 127.0.0.1 3493 | grep -q '^Network UPS Tools ecoflow-ble-nutd ' || exit 1
ENTRYPOINT ["/usr/local/bin/ecoflow-ble-nutd"]
CMD ["-config", "/etc/ecoflow-ble-nutd.conf"]
