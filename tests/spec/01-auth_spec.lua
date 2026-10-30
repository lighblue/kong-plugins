local helpers = require "spec.helpers"
local fixture = dofile("/kong-plugin/spec/support/auth_fixture.lua")

describe("kong-auth Go plugin [#off]", function()
  local environment
  local client

  lazy_setup(function()
    environment = fixture.start()
  end)

  lazy_teardown(function()
    fixture.stop(environment)
  end)

  before_each(function()
    client = helpers.proxy_client()
  end)

  after_each(function()
    if client then client:close() end
  end)

  local function upstream_headers(path, token)
    local response = assert(client:get(path, {
      headers = { authorization = token },
    }))
    assert.response(response).has.status(200)
    return assert.response(response).has.jsonbody()
  end

  describe("authentication", function()
    -- Scenario: omit the credential header; expect 400 and ERR0002.
    it("rejects a request without credentials", function()
      local response = assert(client:get("/auth-test"))
      assert.response(response).has.status(400)
      local body = assert.response(response).has.jsonbody()
      assert.equal("ERR0002", body.code)
    end)

    -- Scenario: send a denied credential; expect 401 and ERR0004.
    it("rejects credentials denied by the auth service", function()
      local response = assert(client:get("/auth-test", {
        headers = { authorization = "Bearer deny" },
      }))
      assert.response(response).has.status(401)
      local body = assert.response(response).has.jsonbody()
      assert.equal("ERR0004", body.code)
    end)
  end)

  describe("upstream header mapping", function()
    -- Scenario: authenticate successfully; expect jwt and role in the configured upstream headers.
    it("maps two response fields to configured header names", function()
      local headers = upstream_headers("/auth-test", "Bearer allow")
      assert.equal("signed-token", headers.token)
      assert.equal("admin", headers.role)
      assert.is_nil(headers.alternate_token)
    end)

    -- Scenario: map jwt to X-Session-JWT; expect it without the default mapping headers.
    it("uses a different header name for the same response field", function()
      local headers = upstream_headers("/auth-alternate", "Bearer allow")
      assert.equal("signed-token", headers.alternate_token)
      assert.is_nil(headers.token)
      assert.is_nil(headers.role)
    end)
  end)

  describe("cache", function()
    -- Scenario: authenticate the same credential twice; expect one auth-service call and matching upstream headers.
    it("reuses a successful authentication response", function()
      environment.auth_mock:clean()
      for _ = 1, 2 do
        local headers = upstream_headers("/auth-test", "Bearer cached")
        assert.equal("signed-token", headers.token)
        assert.equal("admin", headers.role)
      end
      assert.equal(1, #environment.auth_mock:get_all_logs())
    end)
  end)
end)
