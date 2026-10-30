#!/usr/bin/env bash
set -euo pipefail

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_dir=$(cd -- "$tests_dir/.." && pwd)
image_name=${KONG_AUTH_PONGO_IMAGE:-kong-auth-pongo:local}
pongo_bin=${PONGO_BIN:-pongo}

docker build -f "$tests_dir/Dockerfile" -t "$image_name" "$repo_dir"
cd "$tests_dir"
# Pongo forwards host proxy variables into its Docker build. A loopback proxy
# from the host is unreachable inside that build container.
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY \
  -u http_proxy -u https_proxy -u all_proxy \
  KONG_IMAGE="$image_name" "$pongo_bin" run -- ./spec/01-auth_spec.lua
