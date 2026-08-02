package uicalendar

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
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

	// The assigned approver of every draft below. can_approve is "RBAC allows
	// AND (I am this user OR I am an admin)", so the actor's id — not only the
	// role — decides whether the button shows.
	assignedApproverID := uuid.New()

	tests := []struct {
		name     string
		user     *entity.User
		maint    *calendardto.Maintenance
		expected *uimodels.MaintenanceActions
	}{
		{
			name:     "guest sees no draft mutations",
			user:     &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleGuest}},
			maint:    &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{},
		},
		{
			name:  "editor can edit and cancel draft",
			user:  &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleEditor}},
			maint: &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{
				CanEdit:   true,
				CanCancel: true,
			},
		},
		{
			name: "assigned reviewer can approve their own draft",
			user: &entity.User{ID: assignedApproverID, Roles: []entity.Role{entity.RoleReviewer}},
			maint: &calendardto.Maintenance{
				Status:         entity.MaintenanceStatusDraft,
				ApproverUserID: assignedApproverID,
			},
			expected: &uimodels.MaintenanceActions{
				CanEdit:    true,
				CanApprove: true,
				CanCancel:  true,
			},
		},
		{
			name: "reviewer cannot approve a draft assigned to someone else",
			user: &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleReviewer}},
			maint: &calendardto.Maintenance{
				Status:         entity.MaintenanceStatusDraft,
				ApproverUserID: assignedApproverID,
			},
			expected: &uimodels.MaintenanceActions{
				CanEdit:   true,
				CanCancel: true,
			},
		},
		{
			name: "admin can approve a draft assigned to someone else",
			user: &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleAdmin}},
			maint: &calendardto.Maintenance{
				Status:         entity.MaintenanceStatusDraft,
				ApproverUserID: assignedApproverID,
			},
			expected: &uimodels.MaintenanceActions{
				CanEdit:    true,
				CanApprove: true,
				CanCancel:  true,
			},
		},
		{
			name: "blocked admin loses the override",
			user: &entity.User{
				ID:        uuid.New(),
				Roles:     []entity.Role{entity.RoleAdmin},
				BlockedAt: lo.ToPtr(time.Now()),
			},
			maint: &calendardto.Maintenance{
				Status:         entity.MaintenanceStatusDraft,
				ApproverUserID: assignedApproverID,
			},
			expected: &uimodels.MaintenanceActions{
				CanEdit:   true,
				CanCancel: true,
			},
		},
		{
			// MaintView falls back to a zero-valued user when none is in context.
			// Its id is uuid.Nil, so an unset approver would compare equal — the
			// RBAC pre-check is what stops it. Pinned so reordering canApprove's
			// clauses, or granting the fallback a default role, fails loudly
			// instead of showing an anonymous viewer an Approve button.
			name:     "user missing from context cannot approve a zero-approver draft",
			user:     &entity.User{},
			maint:    &calendardto.Maintenance{Status: entity.MaintenanceStatusDraft},
			expected: &uimodels.MaintenanceActions{},
		},
		{
			name:     "guest cannot start planned maintenance",
			user:     &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleGuest}},
			maint:    &calendardto.Maintenance{Status: entity.MaintenanceStatusPlanned},
			expected: &uimodels.MaintenanceActions{},
		},
		{
			name: "editor can finish in-progress maintenance only when steps are terminal",
			user: &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleEditor}},
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

			actions := impl.resolveActions(ctx, tt.user, tt.maint)
			require.Equal(t, tt.expected, actions)
		})
	}
}
