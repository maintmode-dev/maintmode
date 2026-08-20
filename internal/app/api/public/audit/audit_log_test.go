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
	auditaction "github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestAuditLog(t *testing.T) {
	t.Parallel()

	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	impl, seed := initImpl(t)

	for range defaultMaxLogsCount + 1 {
		seed(ctx, auditaction.LoginSuccess{
			User: &entity.User{
				ID:    xuuid.New(),
				Email: t.Name() + "@example.com",
			},
		})
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
			}, {
				// A negative offset is coerced to 0, not rejected: a 400 is
				// returned only for an unparseable value.
				name:         "offset=-5 is coerced, not rejected",
				queryValues:  url.Values{"offset": {"-5"}},
				maxLogsCount: defaultMaxLogsCount,
			}, {
				// A depth past the cap is clamped to the cap — the request is
				// valid, it just points past the edge of what is possible.
				name:         "offset beyond cap is clamped",
				queryValues:  url.Values{"offset": {"100000000"}},
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

	t.Run("limit above ceiling yields exactly the ceiling", func(t *testing.T) {
		t.Parallel()

		// A separate case with an actor filter: in a shared table the exact
		// length can only be asserted over rows of our own. Without exact
		// equality, a clamp to 100 is indistinguishable from a clamp to the
		// default 50 — precisely the silent regression that appears if the
		// helper is given only a ceiling instead of a ceiling and a default.
		actor := xuuid.NewString() + "@example.com"
		user := &entity.User{ID: xuuid.New(), Email: actor}
		for range defaultMaxLogsCount + 1 {
			seed(ctx, auditaction.LoginSuccess{User: user})
		}

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"actor": {actor},
				"limit": {fmt.Sprint(defaultMaxLogsCount + 1)},
			},
		}.ToContextRecorder(t)

		err := impl.AuditLog(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		resp := new(apiauthmodels.AuditLogResponse)
		require.NoError(t, json.NewDecoder(rec.Body).Decode(resp))
		require.Len(t, resp.Logs, defaultMaxLogsCount)
		require.Equal(t, int64(defaultMaxLogsCount+1), resp.Total)
	})

	t.Run("structured login entry", func(t *testing.T) {
		t.Parallel()

		// The email is unique per run: the test DB is shared, and the actor
		// filter must not pick up records from previous runs (-count 2 in make tloc).
		user := &entity.User{
			ID:    xuuid.New(),
			Email: xuuid.NewString() + "@example.com",
			Name:  "Audit Tester",
		}
		sessionID := xuuid.NewString()
		seed(ctx, auditaction.LoginSuccess{
			User: user,
			Meta: &entity.AuditMetadata{
				IP:        "203.0.113.7",
				UserAgent: "audit-test-agent/1.0",
				SessionID: sessionID,
			},
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
		seed(ctx, auditaction.RolesChanged{
			Actor:  actor,
			Target: target,
			Kind:   auditaction.RolesReplaced,
			Change: auditaction.RolesChange{
				Roles:   []entity.Role{entity.RoleEditor},
				Added:   []entity.Role{entity.RoleEditor},
				Removed: []entity.Role{entity.RoleAdmin},
			},
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

	t.Run("total and facets with action filter", func(t *testing.T) {
		t.Parallel()

		// Unique actor isolates this subtest's entries from the shared test DB.
		actor := xuuid.New().String() + "@example.com"
		user := &entity.User{ID: xuuid.New(), Email: actor}
		target := &entity.User{ID: xuuid.New(), Email: "target-" + actor}

		for range 3 {
			seed(ctx, auditaction.LoginSuccess{
				User: user, Meta: &entity.AuditMetadata{},
			})
		}
		seed(ctx, auditaction.LoginFailed{
			User: user, Meta: &entity.AuditMetadata{},
		})
		seed(ctx, auditaction.RolesChanged{
			Actor: user, Target: target, Kind: auditaction.RolesAssigned,
			Change: auditaction.RolesChange{Roles: []entity.Role{entity.RoleAdmin}},
		})
		seed(ctx, auditaction.RolesChanged{
			Actor: user, Target: target, Kind: auditaction.RolesRevoked,
			Change: auditaction.RolesChange{Roles: []entity.Role{entity.RoleAdmin}},
		})
		seed(ctx, auditaction.UserBlocked{
			Actor: user, Target: target,
		})

		wantFacets := apiauthmodels.AuditFacets{All: 7, Auth: 4, Roles: 2, Block: 1}

		for _, tc := range []struct {
			name        string
			queryValues url.Values
			wantLogs    int
			wantTotal   int64
		}{
			{
				name:        "no action filter",
				queryValues: url.Values{"actor": {actor}},
				wantLogs:    7,
				wantTotal:   7,
			}, {
				name:        "single action",
				queryValues: url.Values{"actor": {actor}, "action": {"user.blocked"}},
				wantLogs:    1,
				wantTotal:   1,
			}, {
				name:        "csv actions",
				queryValues: url.Values{"actor": {actor}, "action": {"login.success,login.failed"}},
				wantLogs:    4,
				wantTotal:   4,
			}, {
				name:        "csv actions with spaces",
				queryValues: url.Values{"actor": {actor}, "action": {"roles.changed, user.blocked"}},
				wantLogs:    3,
				wantTotal:   3,
			}, {
				name:        "paginated page keeps full total",
				queryValues: url.Values{"actor": {actor}, "limit": {"2"}, "offset": {"5"}},
				wantLogs:    2,
				wantTotal:   7,
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

				resp := new(apiauthmodels.AuditLogResponse)
				err = json.NewDecoder(rec.Body).Decode(resp)
				require.NoError(t, err)
				require.Len(t, resp.Logs, tc.wantLogs)
				require.Equal(t, tc.wantTotal, resp.Total)
				// Facets ignore the action filter: chips keep counts of the
				// whole actor/date window.
				require.Equal(t, wantFacets, resp.Facets)
			})
		}
	})

	t.Run("invalid action", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name   string
			action string
		}{
			{name: "unknown action", action: "login_success,bogus"},
			{name: "commas only", action: ", ,"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{
					QueryValues: url.Values{"action": {tc.action}},
				}.ToContextRecorder(t)

				err := impl.AuditLog(c)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), "invalid action")
			})
		}
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
