package audit

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// everyAuditAction is every action the backend writes, read from the category
// map: every action belongs to exactly one category (entity's
// TestAuditActionCategories_Pinned), so the union is the whole enum, and an
// action left out of it would not render at all.
func everyAuditAction() []string {
	var all []string
	for _, category := range []entity.AuditCategory{
		entity.AuditCategorySignIn,
		entity.AuditCategoryUsers,
		entity.AuditCategorySettings,
		entity.AuditCategoryMaintenance,
	} {
		for _, action := range entity.AuditCategoryAction(category) {
			all = append(all, string(action))
		}
	}

	return all
}

// actionParamEnums is the Enums(...) list of the `action` query param in the
// AuditLog swag annotation.
func actionParamEnums(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("audit_log.go")
	require.NoError(t, err)

	match := regexp.MustCompile(`(?m)^// @Param action query .*Enums\(([^)]*)\)$`).FindSubmatch(src)
	require.NotNil(t, match, "the AuditLog annotation has no `@Param action ... Enums(...)`")

	return lo.Map(strings.Split(string(match[1]), ","), func(s string, _ int) string {
		return strings.TrimSpace(s)
	})
}

// The swag annotation is a hand-written copy of the action enum, and it fell
// eight actions behind before anyone noticed: the published spec offered a
// client fifteen of twenty-three filter values. swag cannot derive a query
// param's values from a Go type, so this pins the copy to the enum instead.
func TestAuditLogActionParam_ListsEveryAction(t *testing.T) {
	t.Parallel()

	enums := actionParamEnums(t)
	require.ElementsMatch(t, everyAuditAction(), enums,
		"the @Param action Enums(...) in audit_log.go must list exactly the declared audit actions")
	require.Len(t, lo.Uniq(enums), len(enums), "the Enums list repeats an action")
}

// The spec the frontend vendors (docs/auth/swagger.json) must carry the same
// list: a correct annotation with a stale spec is the same drift one step
// later. Fails until `make swag` is run after an enum change.
func TestAuditLogActionParam_SpecIsRegenerated(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../../../docs/auth/swagger.json")
	require.NoError(t, err)

	type enumSchema struct {
		Enum []string `json:"enum"`
	}
	type param struct {
		Name   string     `json:"name"`
		Schema enumSchema `json:"schema"`
	}
	var spec struct {
		Paths map[string]map[string]struct {
			Parameters []param `json:"parameters"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]enumSchema `json:"schemas"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(raw, &spec))

	action, ok := lo.Find(spec.Paths["/api/v1/audit/log"]["get"].Parameters, func(p param) bool {
		return p.Name == "action"
	})
	require.True(t, ok, "GET /api/v1/audit/log has no action param in the spec")

	require.ElementsMatch(t, everyAuditAction(), action.Schema.Enum, "regenerate the spec: make swag")
	require.ElementsMatch(t, everyAuditAction(), spec.Components.Schemas["entity.AuditAction"].Enum,
		"regenerate the spec: make swag")
}
