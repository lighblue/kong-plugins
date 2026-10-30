return {
  name = "kong-auth",
  fields = {
    {
      config = {
        type = "record",
        fields = {
          { auth_server_url = { type = "string", required = true } },
          { request_header_name = { type = "string", required = true } },
          {
            extra_header_mappings = {
              type = "array",
              required = false,
              elements = {
                type = "record",
                fields = {
                  { source_field = { type = "string", required = true, len_min = 1 } },
                  { name = { type = "string", required = true, len_min = 1 } },
                },
              },
            },
          },
        },
      },
    },
  },
}
