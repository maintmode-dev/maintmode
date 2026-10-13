package maintmode

// This file is the swag/v2 general-info entrypoint for the auth service spec
// (`make swag` passes it via -g). It must live in the module root for the same
// cross-package type resolution reason described in doc_maintmode.go. No Go
// code is generated from it; the spec is written to docs/auth/swagger.{yaml,json}.

// SwaggerDocsAuth godoc
// @title Auth API
// @version 1.0.0
// @description Authentication, authorization and audit API.
// @termsOfService https://example.com/terms
// @contact.name Platform Team
// @contact.email platform@example.com
// @license.name Proprietary
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @tag.name Auth
// @tag.description Authentication API
// @tag.name Roles
// @tag.description Role management API
// @tag.name Users
// @tag.description User management API
// @tag.name Audit
// @tag.description Audit log API
func SwaggerDocsAuth() {}

// The s2s schemes sit in comment blocks of their own: swag v2.0.0-rc5 names
// every security definition in one block after the FIRST one in that block,
// so a second apikey there silently overwrites BearerAuth.

// @securityDefinitions.apikey S2SAppName
// @in header
// @name X-S2S-APPNAME
// @description Name of the calling service (the BFF: maintmode-ui). Token endpoints only.

// @securityDefinitions.apikey S2SToken
// @in header
// @name X-S2S-TOKEN
// @description Shared secret of the calling service. Token endpoints only.
