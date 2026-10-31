# kong-auth

`kong-auth` is a Go plugin for Kong Gateway. During the `access` phase, it calls an external authentication service and can copy selected fields from the authentication response into the upstream request headers.

## Features

1. Read the credential from the request header named in the plugin configuration.
2. Send a `POST` request to `auth_server_url` with the credential header and allow requests authenticated by the service.
3. Map selected top-level fields from the authentication JSON response to upstream request headers through `extra_header_mappings`.
4. Cache successful authentication responses by credential value in the plugin process. A cache hit avoids another call to the authentication service.

### Plugin schema

The schema is defined in [`kong/plugins/kong-auth/schema.lua`](kong/plugins/kong-auth/schema.lua). The plugin definition used by Konnect and decK is in [`kong/plugins/kong-auth/kong.yml`](kong/plugins/kong-auth/kong.yml).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `auth_server_url` | string | Yes | URL of the external authentication service. The plugin sends a `POST` request to it. |
| `request_header_name` | string | Yes | Name of the client request header to read and forward to the authentication service. |
| `extra_header_mappings` | array | No | Mappings from authentication response fields to upstream request headers. |
| `extra_header_mappings[].source_field` | string | Yes | Nonempty top-level field name in the authentication JSON response. |
| `extra_header_mappings[].name` | string | Yes | Nonempty name of the upstream request header to set. |

For example, if the authentication service returns `{"jwt":"signed-token","role":"admin"}`, the following configuration sets `X-Auth-Token: signed-token` and `X-Auth-Role: admin` on the upstream request:

```yaml
plugins:
  - name: kong-auth
    config:
      auth_server_url: https://auth.example.com/check
      request_header_name: Authorization
      extra_header_mappings:
        - source_field: jwt
          name: X-Auth-Token
        - source_field: role
          name: X-Auth-Role
```

Runtime settings are in [`configs/config.toml`](configs/config.toml): `auth_check.timeout` is the authentication timeout in seconds, `auth_cache.ttl` is the cache lifetime in seconds, and `auth_cache.cleanup_interval` is the expired-entry cleanup interval in seconds. A `ttl` of zero or less disables cache writes. The Docker image copies this file to `/configs/config.toml`.

## Build

Go 1.24 is required. Build a Linux executable for the host architecture:

```sh
make build
```

The output is `build/<GOOS>-<GOARCH>/kong-auth`. To build for Linux AMD64 explicitly:

```sh
make build GOOS=linux GOARCH=amd64
```

Alternatively, build a Kong image containing the plugin. The root [`Dockerfile`](Dockerfile) compiles the Go code in a builder stage and copies the executable into the runner stage:

```sh
docker build -t kong-auth:local .
```

## Unit tests

Run all Go unit tests from the repository root:

```sh
make ut
```

This runs `go test ./...` and covers configuration loading, authentication calls, request header handling, response field mapping, and the in-memory cache.

## Integration tests

### Automated tests with Pongo

Install Docker with Compose and [Kong Pongo](https://github.com/Kong/kong-pongo), and make sure `pongo` is on your `PATH`. Then run:

```sh
make integration
```

If Pongo is available as a script rather than a command on `PATH`, specify its path:

```sh
PONGO_BIN=/path/to/kong-pongo/pongo.sh make integration
```

[`tests/run.sh`](tests/run.sh) builds the test image, starts the Go plugin in Kong Gateway 3.16.0.0, and runs [`tests/spec/01-auth_spec.lua`](tests/spec/01-auth_spec.lua). Its five cases cover a missing credential, a denied credential, mapping `jwt` and `role` to configured upstream headers, mapping `jwt` to the alternate `X-Session-JWT` header, and reuse of a cached authentication response. The authentication service and upstream are local mocks, so no Konnect account is required. See [`tests/README.md`](tests/README.md) for more details.

To stop the dependency containers after a test run, run `pongo down` from the `tests/` directory.

### Manual validation with Quickstart

[`deploy.sh`](deploy.sh) provides a second end-to-end path. It builds the Kong image, uses [Kong Gateway Quickstart](https://developer.konghq.com/gateway/quickstart-reference/) to create a Konnect `quickstart` control plane and a local Data Plane, then uses decK to apply an example Service, Route, plugin definition, and plugin configuration. This path requires Docker, `curl`, [decK](https://developer.konghq.com/deck/), a Konnect Personal Access Token, and network access to the public example services used by the script.

Run the script from the repository root. Provide the token through an environment variable rather than storing it in the repository:

```sh
export KONNECT_TOKEN='your Konnect PAT'
bash ./deploy.sh
```

The script creates a `/mock` route, configures the plugin to read `Authorization`, and maps the authentication response field `origin` to the upstream request header `Origin-IP`. This value is the IP address seen by the example authentication service when Kong calls it. Its example authentication URL is `http://httpbin.org/anything`. The success check below requires that the Data Plane can reach this URL and that it returns a `2xx` JSON response containing `origin` for a `POST` request.

```sh
# Missing credential: expect HTTP 400 and ERR0002.
curl -i http://localhost:8000/mock

# With a reachable authentication service, /mock/anything reaches /anything upstream.
# Expect HTTP 200 and headers.Origin-IP in the JSON response.
curl -i -H 'Authorization: Bearer demo' http://localhost:8000/mock/anything
```

Quickstart uses a real Konnect environment and public example services for manual deployment checks. The Pongo suite provides repeatable behavior assertions.
