package openapi

import _ "embed"

//go:embed openapi.yaml
var Document []byte

//go:embed swagger.html
var SwaggerUI []byte
