FROM kong/kong-gateway:3.16

# Ensure any patching steps are executed as root user
USER root

# Add custom plugin to the image
COPY ./build/linux-arm64/kong-auth /usr/local/bin/kong-auth
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
