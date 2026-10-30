local helpers = require "spec.helpers"
local http_mock = require "spec.helpers.http_mock"

local fixture = {}

local function start_auth_mock()
  local mock, port = http_mock.new(nil, {
    ["/"] = {
      access = [[
        local token = ngx.req.get_headers()["authorization"]
        if token ~= "Bearer allow" and token ~= "Bearer cached" then
          return ngx.exit(403)
        end
        ngx.header.content_type = "application/json"
        ngx.say('{"jwt":"signed-token","role":"admin"}')
      ]],
    },
  }, { prefix = "servroot_auth_mock" })
  assert(mock:start())
  return mock, port
end

local function start_upstream_mock()
  local mock, port = http_mock.new(nil, {
    ["/"] = {
      access = [[
        local headers = ngx.req.get_headers()
        ngx.header.content_type = "application/json"
        ngx.say(require("cjson").encode({
          token = headers["x-auth-token"],
          role = headers["x-auth-role"],
          alternate_token = headers["x-session-jwt"],
        }))
      ]],
    },
  }, { prefix = "servroot_upstream_mock" })
  assert(mock:start())
  return mock, port
end

local function write_kong_config(upstream_port, auth_port)
  local template_file = assert(io.open("/kong-plugin/spec/support/kong.yaml.template", "r"))
  local template = assert(template_file:read("*a"))
  assert(template_file:close())

  local path = os.tmpname() .. ".yaml"
  local config_file = assert(io.open(path, "w"))
  assert(config_file:write(string.format(template, upstream_port, auth_port, auth_port)))
  assert(config_file:close())
  return path
end

function fixture.start()
  local auth_mock, auth_port = start_auth_mock()
  local upstream_mock, upstream_port = start_upstream_mock()
  local config_path = write_kong_config(upstream_port, auth_port)

  assert(helpers.start_kong({
    database = "off",
    declarative_config = config_path,
    plugins = "bundled,kong-auth",
    pluginserver_names = "kong-auth",
    pluginserver_kong_auth_socket = "/kong-plugin/servroot/kong-auth.socket",
    pluginserver_kong_auth_start_cmd = "/usr/local/bin/kong-auth -kong-prefix /kong-plugin/servroot",
    pluginserver_kong_auth_query_cmd = "/usr/local/bin/kong-auth -dump",
    nginx_worker_processes = 1,
  }))

  return {
    auth_mock = auth_mock,
    upstream_mock = upstream_mock,
    config_path = config_path,
  }
end

function fixture.stop(environment)
  helpers.stop_kong(nil, true)
  environment.upstream_mock:stop()
  environment.auth_mock:stop()
  os.remove(environment.config_path)
end

return fixture
