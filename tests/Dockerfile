FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/kong-auth ./cmd/server

FROM kong/kong-gateway:3.16.0.0
USER root
COPY --from=builder /out/kong-auth /usr/local/bin/kong-auth
COPY configs/config.toml /configs/config.toml
RUN chmod 755 /usr/local/bin/kong-auth && chmod 644 /configs/config.toml

ENV KONG_PLUGINS=bundled,kong-auth \
    KONG_PLUGINSERVER_NAMES=kong-auth \
    KONG_PLUGINSERVER_KONG_AUTH_QUERY_CMD="/usr/local/bin/kong-auth -dump"
USER kong
