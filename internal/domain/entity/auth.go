package entity

type ExtraHeaderMapping struct {
	SourceField string `json:"source_field"`
	Name        string `json:"name"`
}
type AuthConfig struct {
	AuthServerURL       string               `json:"auth_server_url"`
	RequestHeaderName   string               `json:"request_header_name"`
	ExtraHeaderMappings []ExtraHeaderMapping `json:"extra_header_mappings"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}
