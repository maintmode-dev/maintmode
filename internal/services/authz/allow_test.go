package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/entity"
)

const maintmodePolicy = `
g, editor, guest
g, reviewer, editor
g, admin, reviewer

p, guest, calendar.read, execute
p, guest, maintenance.read, execute
p, guest, resource.read, execute

p, editor, maintenance.create, execute
p, editor, maintenance.edit, execute
p, editor, maintenance.start, execute
p, editor, maintenance.complete, execute
p, editor, maintenance.cancel, execute
p, editor, maintenance.step.start, execute
p, editor, maintenance.step.complete, execute
p, editor, maintenance.step.cancel, execute
p, editor, resource.create, execute
p, editor, resource.archive, execute

p, reviewer, maintenance.approve, execute

p, admin, integration.read, execute
p, admin, integration.manage, execute
`

const authPolicy = `
g, editor, guest
g, reviewer, editor
g, admin, reviewer

p, guest, auth.roles.read, execute
p, guest, auth.user_roles.read, execute

p, admin, auth.roles.manage, execute
p, admin, auth.audit.read, execute
`

func TestCasbinAuthorizer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("maintmode Policy", func(t *testing.T) {
		t.Parallel()

		authorizer, err := NewCasbinAuthorizer(config.RbacConfig{
			ModelPath:  "../../../deployment/maintmode/authz/model.conf",
			Adapter:    config.AuthorizationAdapterMemory,
			PolicyData: maintmodePolicy,
		})
		require.NoError(t, err)

		tests := []struct {
			name     string
			roles    []entity.Role
			scenario entity.AuthzScenario
			allowed  bool
		}{
			{
				name:     "guest can read calendar",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioCalendarRead,
				allowed:  true,
			}, {
				name:     "guest cannot create maintenance",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioMaintenanceCreate,
				allowed:  false},
			{
				name:     "editor inherits guest read",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioMaintenanceRead,
				allowed:  true,
			}, {
				name:     "editor can create resource",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioResourceCreate,
				allowed:  true,
			}, {
				name:     "editor can archive resource",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioResourceArchive,
				allowed:  true,
			}, {
				name:     "guest cannot archive resource",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioResourceArchive,
				allowed:  false,
			}, {
				name:     "editor cannot approve",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioMaintenanceApprove,
				allowed:  false,
			}, {
				name:     "reviewer inherits editor mutation",
				roles:    []entity.Role{entity.RoleReviewer},
				scenario: entity.AuthzScenarioMaintenanceStart,
				allowed:  true,
			}, {
				name:     "reviewer can approve",
				roles:    []entity.Role{entity.RoleReviewer},
				scenario: entity.AuthzScenarioMaintenanceApprove,
				allowed:  true,
			}, {
				name:     "admin inherits approve",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioMaintenanceApprove,
				allowed:  true,
			}, {
				name:     "admin can manage integrations",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioIntegrationManage,
				allowed:  true,
			}, {
				name:     "admin can read integrations",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioIntegrationRead,
				allowed:  true,
			}, {
				name:     "editor cannot manage integrations",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioIntegrationManage,
				allowed:  false,
			}, {
				name:     "guest cannot read integrations",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioIntegrationRead,
				allowed:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				allowed, err := authorizer.Allow(ctx, tt.roles, tt.scenario)
				require.NoError(t, err)
				require.Equal(t, tt.allowed, allowed)
			})
		}
	})

	t.Run("auth Policy", func(t *testing.T) {
		t.Parallel()

		authorizer, err := NewCasbinAuthorizer(config.RbacConfig{
			ModelPath:  "../../../deployment/maintmode/authz/model.conf",
			Adapter:    config.AuthorizationAdapterMemory,
			PolicyData: authPolicy,
		})
		require.NoError(t, err)

		tests := []struct {
			name     string
			roles    []entity.Role
			scenario entity.AuthzScenario
			allowed  bool
		}{
			{
				name:     "guest can read role list",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioAuthRolesRead,
				allowed:  true,
			}, {
				name:     "guest can read users roles",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioAuthUserRolesRead,
				allowed:  true,
			}, {
				name:     "guest cannot manage roles",
				roles:    []entity.Role{entity.RoleGuest},
				scenario: entity.AuthzScenarioAuthRolesManage,
				allowed:  false,
			}, {
				name:     "editor cannot read audit",
				roles:    []entity.Role{entity.RoleEditor},
				scenario: entity.AuthzScenarioAuthAuditRead,
				allowed:  false,
			}, {
				name:     "admin can manage roles",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioAuthRolesManage,
				allowed:  true,
			}, {
				name:     "admin can read audit",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioAuthAuditRead,
				allowed:  true,
			}, {
				name:     "admin can read users roles",
				roles:    []entity.Role{entity.RoleAdmin},
				scenario: entity.AuthzScenarioAuthUserRolesRead,
				allowed:  true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				allowed, err := authorizer.Allow(ctx, tt.roles, tt.scenario)
				require.NoError(t, err)
				require.Equal(t, tt.allowed, allowed)
			})
		}
	})
}
