# Pongo integration tests

These tests run the compiled Go plugin inside Kong Gateway 3.16.0.0 and exercise it through the proxy. The authentication service and upstream are local mock servers, so the cases do not depend on an external API.

Prerequisites: Docker with Compose, [kong-pongo](https://github.com/Kong/kong-pongo) on `PATH`, and access to the `golang:1.24` and `kong/kong-gateway:3.16.0.0` images. Pongo 2.30.0 supports this exact Kong Gateway version.

Run from the repository root:

```sh
./tests/run.sh
```

The runner builds a Linux plugin executable for the image's architecture, creates a test Kong image, then invokes Pongo from `tests/`. Set `PONGO_BIN` to a Pongo script path or `KONG_AUTH_PONGO_IMAGE` to override the local image name. Pongo creates its own test image and starts PostgreSQL on first run, so that first run can take several minutes. Kong itself runs in DB-less mode; Pongo's test helper still initializes its PostgreSQL fixture.

The runner clears inherited HTTP proxy variables before calling Pongo because a host loopback proxy cannot be reached from Docker's build container. The image build needs direct access to Ubuntu package repositories and Kong's test dependencies.

The suite checks missing credentials, a rejected credential, two response-field mappings (`jwt` to `X-Auth-Token` and `role` to `X-Auth-Role`), an alternate route that maps `jwt` to `X-Session-JWT`, and reuse of a successful result from the in-memory cache. It runs with one Kong worker so both cache requests use the same plugin process.

Test files:

- `spec/01-auth_spec.lua`: behavior cases and assertions.
- `spec/support/auth_fixture.lua`: mock services, Kong startup, and cleanup.
- `spec/support/kong.yaml.template`: routes and plugin configurations used by the fixture.
