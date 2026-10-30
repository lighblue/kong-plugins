FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/kong-auth ./cmd/server

FROM kong/kong-gateway:3.16 AS runner

# Ensure any patching steps are executed as root user
USER root

# Add the plugin built for the runner image's target platform.
COPY --from=builder /out/kong-auth /usr/local/bin/kong-auth
COPY ./configs/ /configs/
#COPY ./kong/plugins/kong-auth/schema.lua /usr/local/share/lua/5.1/kong/plugins/kong-auth/schema.lua
ENV KONG_PLUGINS=bundled,kong-auth
ENV KONG_PLUGINSERVER_NAMES=kong-auth
ENV KONG_PLUGINSERVER_KONG_AUTH_QUERY_CMD="/usr/local/bin/kong-auth -dump"
ENV KONG_LOG_LEVEL=info

# Ensure kong user is selected for image execution
USER kong

# Run kong
ENTRYPOINT ["/entrypoint.sh"]
EXPOSE 8000 8443 8001 8444
STOPSIGNAL SIGQUIT
HEALTHCHECK --interval=10s --timeout=10s --retries=10 CMD kong health
CMD ["kong", "docker-start"]
