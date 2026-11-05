// Package api is the HTTP contract. openapi.yaml is the source of truth;
// api.gen.go is generated from it by `make generate` (oapi-codegen).
package api

import _ "embed"

// Spec is the OpenAPI document, served at GET /openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
