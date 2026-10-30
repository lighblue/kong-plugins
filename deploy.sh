docker build -t kong-gateway_auth:0.0.2 .
curl -Ls https://get.konghq.com/quickstart | \
  bash -s -- -r "" -i kong-gateway_auth -t 0.0.2 -k $KONNECT_TOKEN --deck-output

export DECK_KONNECT_TOKEN=$KONNECT_TOKEN
export DECK_KONNECT_CONTROL_PLANE_NAME=quickstart
export DECK_KONNECT_ADDR=https://us.api.konghq.com
export KONNECT_CONTROL_PLANE_URL=https://us.api.konghq.com
export KONNECT_PROXY_URL='http://localhost:8000'

echo '
_format_version: "3.0"
services:
  - name: example_service
    url: https://httpbin.konghq.com
' | deck gateway apply -

echo '
_format_version: "3.0"
routes:
  - name: example_route
    service:
      name: example_service
    paths:
    - "/mock"
    protocols:
    - http
    - https
' | deck gateway apply -


deck gateway apply --include-plugin-definitions kong/plugins/kong-auth/kong.yml


echo '
_format_version: "3.0"
plugins:
  - name: kong-auth
    config:
      auth_server_url: "http://httpbin.org/anything"
      request_header_name: "headerx"
      extra_header_mappings:
        - source_field: origin
          name: X-Auth-Token
' | deck gateway apply -
