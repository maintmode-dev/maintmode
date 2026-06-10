package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/audit/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestAuditLog(t *testing.T) {
	t.Parallel()

	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	impl := initImpl(t)

	for range defaultMaxLogsCount + 1 {
		impl.auditSrv.LogLogin(ctx, entity.AuditEventLoginSuccess, &entity.User{
			ID:    xuuid.New(),
			Email: t.Name() + "@example.com",
		}, nil)
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name         string
			queryValues  url.Values
			maxLogsCount int
		}{
			{
				name:         "no query",
				maxLogsCount: defaultMaxLogsCount,
			}, {
				name:         "limit=1",
				queryValues:  url.Values{"limit": {"1"}},
				maxLogsCount: 1,
			}, {
				name:         fmt.Sprintf("limit=%d", defaultMaxLogsCount),
				queryValues:  url.Values{"limit": {fmt.Sprint(defaultMaxLogsCount)}},
				maxLogsCount: defaultMaxLogsCount,
			}, {
				name:         fmt.Sprintf("limit=%d", defaultMaxLogsCount+1),
				queryValues:  url.Values{"limit": {fmt.Sprint(defaultMaxLogsCount + 1)}},
				maxLogsCount: defaultMaxLogsCount,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{
					QueryValues: tc.queryValues,
				}.ToContextRecorder(t)

				err := impl.AuditLog(c)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))

				resp := new(apiauthmodels.AuditLogResponse)
				err = json.NewDecoder(rec.Body).Decode(resp)
				require.NoError(t, err)
				require.LessOrEqual(t, len(resp.Logs), tc.maxLogsCount)
			})
		}
	})

	t.Run("structured login entry", func(t *testing.T) {
		t.Parallel()

		// Email уникален на каждый прогон: тестовая БД общая, и фильтр по actor
		// не должен цеплять записи предыдущих прогонов (-count 2 в make tloc).
		user := &entity.User{
			ID:    xuuid.New(),
			Email: xuuid.NewString() + "@example.com",
			Name:  "Audit Tester",
		}
		sessionID := xuuid.NewString()
		impl.auditSrv.LogLogin(ctx, entity.AuditEventLoginSuccess, user, &entity.AuditMetadata{
			IP:        "203.0.113.7",
			UserAgent: "audit-test-agent/1.0",
			SessionID: sessionID,
		})

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{"actor": {user.Email}},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		resp := new(apiauthmodels.AuditLogResponse)
		require.NoError(t, json.NewDecoder(rec.Body).Decode(resp))
		require.Len(t, resp.Logs, 1)

		log := resp.Logs[0]
		require.Equal(t, user.Email, log.Actor)
		require.Equal(t, user.ID.String(), log.ActorID)
		require.Equal(t, user.Name, log.ActorDisplayName)
		require.Equal(t, user.ID.String(), log.EntityID)
		require.NotNil(t, log.Metadata)
		require.Equal(t, "203.0.113.7", log.Metadata.IP)
		require.Equal(t, "audit-test-agent/1.0", log.Metadata.UserAgent)
		require.Equal(t, sessionID, log.Metadata.SessionID)
	})

	t.Run("structured roles entry", func(t *testing.T) {
		t.Parallel()

		actor := &entity.User{
			ID:    xuuid.New(),
			Email: xuuid.NewString() + "+actor@example.com",
			Name:  "Roles Admin",
		}
		target := &entity.User{
			ID:    xuuid.New(),
			Email: xuuid.NewString() + "+target@example.com",
			Name:  "Roles Target",
		}
		impl.auditSrv.LogChangeRoles(ctx, entity.AuditEventRoleReplaced, actor, target, entity.AuditRolesChange{
			Roles:   []entity.Role{entity.RoleEditor},
			Added:   []entity.Role{entity.RoleEditor},
			Removed: []entity.Role{entity.RoleAdmin},
		})

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{"actor": {actor.Email}},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		resp := new(apiauthmodels.AuditLogResponse)
		require.NoError(t, json.NewDecoder(rec.Body).Decode(resp))
		require.Len(t, resp.Logs, 1)

		log := resp.Logs[0]
		require.Equal(t, actor.ID.String(), log.ActorID)
		require.Equal(t, actor.Name, log.ActorDisplayName)
		require.Equal(t, target.ID.String(), log.EntityID)
		require.NotNil(t, log.Metadata)
		require.Equal(t, []string{string(entity.RoleEditor)}, log.Metadata.Roles)
		require.Equal(t, []string{string(entity.RoleEditor)}, log.Metadata.RolesAdded)
		require.Equal(t, []string{string(entity.RoleAdmin)}, log.Metadata.RolesRemoved)
		require.Equal(t, target.Email, log.Metadata.TargetEmail)
		require.Equal(t, target.Name, log.Metadata.TargetDisplayName)
	})

	t.Run("limit is not a digit", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{"limit": {"not a digit"}},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))
		require.Contains(t, rec.Body.String(), "invalid limit")
	})

	t.Run("offset is not a digit", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{"offset": {"not a digit"}},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "invalid offset")
	})

	t.Run("invalid created_from", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{"created_from": {"not-a-time"}},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "invalid created_from")
	})

	t.Run("created_from after created_to", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"created_from": {"2026-05-13T12:00:00Z"},
				"created_to":   {"2026-05-13T11:00:00Z"},
			},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Contains(t, rec.Body.String(), "created_from must be")
	})
}
