// Package docs embeds the generated OpenAPI specs so they can be served by the
// Swagger UI handler at runtime without a generated docs.go Go package.
//
// The specs themselves are produced by `make swag` (swag/v2) into
// docs/{maintmode,auth}/swagger.yaml. Only the YAML form is embedded; the UI
// reads it directly.
package docs

import _ "embed"

//go:embed maintmode/swagger.yaml
var MaintmodeSpec []byte

//go:embed auth/swagger.yaml
var AuthSpec []byte
