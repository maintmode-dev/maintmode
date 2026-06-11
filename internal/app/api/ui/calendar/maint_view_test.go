package uicalendar

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/authz"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestMaintView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name            string
			pathValues      echo.PathValues
			expResponseCode int
		}{
			{
				name: "invalid uuid",
				pathValues: echo.PathValues{
					{Name: "id", Value: "invalid-uuid"},
				},
				expResponseCode: http.StatusBadRequest,
			}, {
				name: "not found",
				pathValues: echo.PathValues{
					{Name: "id", Value: uuid.New().String()},
				},
				expResponseCode: http.StatusNotFound,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
				c.SetPathValues(tc.pathValues)

				err := impl.MaintView(c)
				require.NoError(t, err)
				require.Equal(t, tc.expResponseCode, rec.Code)
			})
		}
	})

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name            string
			prepareFunc     func(c *echo.Context)
			expectedActions *uimodels.MaintenanceActions
		}{
			{
				name: "with user in context",
				prepareFunc: func(c *echo.Context) {
					xecho.UserToEchoCtx(c, &entity.User{
						ID:    uuid.New(),
						Roles: []entity.Role{entity.RoleEditor},
						Email: "editor@test.com",
					})
				},
				expectedActions: &uimodels.MaintenanceActions{
					CanEdit:   true,
					CanCancel: true,
				},
			}, {
				name:            "without user in context",
				prepareFunc:     func(_ *echo.Context) {},
				expectedActions: &uimodels.MaintenanceActions{},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				maint := makeMaint(ctx, t)

				c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
				c.SetPathValues(echo.PathValues{
					{Name: "id", Value: maint.ID.String()},
				})
				tc.prepareFunc(c)

				err := impl.MaintView(c)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)

				resp := testjsonudils.JSONToAny[uimodels.MaintenanceViewResponse](t, rec.Body)

				require.NotNil(t, resp.Maintenance)
				require.Equal(t, tc.expectedActions, resp.Actions)

				// created_by is always present on read and carries the author id
				// from the create response. The display_name resolves from auth
				// when available, else degrades to the "Unknown user" label —
				// either way the read does not fail and the id is preserved.
				require.NotNil(t, resp.Maintenance.CreatedBy)
				require.Equal(t, maint.CreatedBy.ID, resp.Maintenance.CreatedBy.ID)
				require.NotEmpty(t, resp.Maintenance.CreatedBy.DisplayName)

				// notify_targets carries the catalog-resolved chips (id + name +
				// transport) for the read-only Notify channels section. makeMaint
				// subscribes one telegram channel named after the test.
				require.Len(t, resp.Maintenance.NotifyTargets, 1)
				require.NotEqual(t, uuid.Nil, resp.Maintenance.NotifyTargets[0].ID)
				require.Equal(t, t.Name(), resp.Maintenance.NotifyTargets[0].Name)
				require.Equal(t, string(entity.NotifyTransportTelegram), resp.Maintenance.NotifyTargets[0].Transport)
			})
		}
	})
}

func TestResolveActionsRoleAware(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	authorizer, err := authz.NewCasbinAuthorizer(config.RbacConfig{
		ModelPath:  "../../../../../deployment/maintmode/authz/model.conf",
		Adapter:    config.AuthorizationAdapterMemory,
		PolicyData: maintmodePolicy,
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		roles    []entity.Role
		maint    *calendardto.Maintenance
		expected *uimodels.MaintenanceActions
	}{
		{
			name:     "guest sees no draft mutations",
			roles:    []entity.Role{entity.RoleGuest},
			maint:    &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{},
		},
		{
			name:  "editor can edit and cancel draft",
			roles: []entity.Role{entity.RoleEditor},
			maint: &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{
				CanEdit:   true,
				CanCancel: true,
			},
		},
		{
			name:  "reviewer inherits editor and can approve draft",
			roles: []entity.Role{entity.RoleReviewer},
			maint: &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{
				CanEdit:    true,
				CanApprove: true,
				CanCancel:  true,
			},
		},
		{
			name:     "guest cannot start planned maintenance",
			roles:    []entity.Role{entity.RoleGuest},
			maint:    &calendardto.Maintenance{Status: entity.MaintenanceStatusPlanned},
			expected: &uimodels.MaintenanceActions{},
		},
		{
			name:  "editor can finish in-progress maintenance only when steps are terminal",
			roles: []entity.Role{entity.RoleEditor},
			maint: &calendardto.Maintenance{
				Status: entity.MaintenanceStatusInProgress,
				Steps: []*calendardto.MaintenanceStep{
					{Status: entity.MaintenanceStepStatusCompleted},
				},
			},
			expected: &uimodels.MaintenanceActions{
				CanCancel:   true,
				CanComplete: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := xlog.ContextWithLogger(ctx, xlog.NewZapAdapter(zaptest.NewLogger(t)))

			impl := Implementation{
				authorizer: authorizer,
			}

			actions := impl.resolveActions(ctx, tt.roles, tt.maint)
			require.Equal(t, tt.expected, actions)
		})
	}
}
