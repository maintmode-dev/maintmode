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

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
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
				name:         "negative limit",
				queryValues:  url.Values{"limit": {fmt.Sprint(-1)}},
				maxLogsCount: defaultMaxLogsCount, // don't raise an error, but return default logs count
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{
					QueryValues: tc.queryValues,
				}.ToContextRecorder(t)

				err := impl.AuditLog(c)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))

				resp := new(apiauthmodels.AuditLogResponse)
				err = json.NewDecoder(rec.Body).Decode(resp)
				require.NoError(t, err)
				require.LessOrEqual(t, len(resp.Logs), tc.maxLogsCount)
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

		resp := httperrors.NewErrorResponse(httperrors.ErrInvalidRequest, httperrors.ErrParseBody.Error())
		require.Equal(t, resp.JSON(), rec.Body.String())
	})
}
