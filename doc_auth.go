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
