package maintmode

// This file is the swag/v2 general-info entrypoint for the maintmode service
// spec (`make swag` passes it via -g). It must live in the module root: swag's
// --parseInternal cross-package type resolution breaks when the entrypoint sits
// in a subdirectory (it then fails to disambiguate the duplicate apiauthmodels
// package name). No Go code is generated from it; the spec is written to
// docs/maintmode/swagger.{yaml,json}.

// SwaggerDocsMaintmode godoc
// @title Maintenance Calendar API
// @version 1.0.0
// @description API for planning and executing maintenance windows.
// @description
// @description Core idea: overlapping time + shared resources = risk.
// @termsOfService https://example.com/terms
// @contact.name Platform Team
// @contact.email platform@example.com
// @license.name Proprietary
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @tag.name UI
// @tag.description Maintenance UI API
// @tag.name Maintenances
// @tag.description Maintenance management API
// @tag.name Resources
// @tag.description Resource management API
// @tag.name Notifications
// @tag.description Notification channels API
func SwaggerDocsMaintmode() {}
