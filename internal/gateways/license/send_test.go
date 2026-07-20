package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestClient_Send(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	report := &entity.HeartbeatReport{
		SeatsUsed: entity.SeatUsage{
			Admin:  entity.SeatBucket{Active: 1},
			Editor: entity.SeatBucket{Active: 2, Pending: 1},
			Guest:  entity.SeatBucket{Active: 3},
		},
		Version: "1.2.3",
	}

	t.Run("posts the contract shape and decodes the license", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/cloud/v1/instances/heartbeat", r.URL.Path)
			require.Equal(t, "Bearer tok-123", r.Header.Get("Authorization"))
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			// seats used = admin 1 + editor 2; the 3 active guests are read-only.
			require.EqualValues(t, 3, body["seats_used"])
			require.Equal(t, "1.2.3", body["version"])
			require.Nil(t, body["last_activity_at"], "no activity reports null")
			perRole, ok := body["per_role"].(map[string]any)
			require.True(t, ok)
			for _, role := range []string{"admin", "reviewer", "editor", "guest"} {
				require.Contains(t, perRole, role)
			}
			editor, ok := perRole["editor"].(map[string]any)
			require.True(t, ok)
			require.EqualValues(t, 2, editor["active"])
			require.EqualValues(t, 1, editor["pending"])

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":          "active",
				"seats_purchased": 10,
			})
		}))
		defer srv.Close()

		got, err := NewClient(config.LicenseConfig{
			URL:           srv.URL,
			InstanceToken: "tok-123",
			HTTPTimeout:   0,
		}).Send(ctx, report)
		require.NoError(t, err)
		require.Equal(t, entity.LicenseStatusActive, got.Status)
		require.EqualValues(t, 10, *got.SeatsPurchased)
		require.Nil(t, got.FetchedAt, "FetchedAt is stamped by the caller, not the gateway")
	})

	t.Run("trailing slash in base URL is normalized", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/cloud/v1/instances/heartbeat", r.URL.Path, "no //cloud double slash")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "active", "seats_purchased": 1})
		}))
		defer srv.Close()

		_, err := NewClient(config.LicenseConfig{
			URL:           srv.URL + "/",
			InstanceToken: "tok",
			HTTPTimeout:   0,
		}).Send(ctx, report)
		require.NoError(t, err)
	})

	t.Run("non-200 is an error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := NewClient(config.LicenseConfig{
			URL:           srv.URL,
			InstanceToken: "bad-token",
			HTTPTimeout:   0,
		}).Send(ctx, report)
		require.ErrorContains(t, err, "401")
	})

	t.Run("unknown status is passed through, not an error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "past_due", "seats_purchased": 1})
		}))
		defer srv.Close()

		// A status the instance does not recognize is decoded as-is: License.Blocked
		// leaves an unknown status unblocked, so the instance keeps running rather
		// than rejecting the whole heartbeat.
		lic, err := NewClient(config.LicenseConfig{
			URL:           srv.URL,
			InstanceToken: "tok",
			HTTPTimeout:   0,
		}).Send(ctx, report)
		require.NoError(t, err)
		require.Equal(t, entity.LicenseStatus("past_due"), lic.Status)
	})

	t.Run("unreachable server is an error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close() // shut down before use

		_, err := NewClient(config.LicenseConfig{
			URL:           srv.URL,
			InstanceToken: "tok",
			HTTPTimeout:   time.Second,
		}).Send(ctx, report)
		require.Error(t, err)
	})
}
