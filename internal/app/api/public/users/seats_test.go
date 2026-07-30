package users

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_license "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/license"
)

// initImplWithLicense builds the handler over a mocked enforcement surface. The
// bootstrap wiring cannot be used for these cases: without a license config it
// hands out Noop, which reports unlimited with zeros and never fails, so
// neither a real count nor the error path would be reachable.
func initImplWithLicense(t *testing.T) (*Implementation, *mock_license.MockEnforcement) {
	t.Helper()

	licenseSrv := mock_license.NewMockEnforcement(gomock.NewController(t))

	return New(nil, licenseSrv), licenseSrv
}

func TestGetSeats(t *testing.T) {
	t.Parallel()

	seatUsage := entity.SeatUsage{
		Admin:    entity.SeatBucket{Active: 1},
		Reviewer: entity.SeatBucket{Active: 1},
		Editor:   entity.SeatBucket{Active: 2, Pending: 1},
		Guest:    entity.SeatBucket{Active: 3, Pending: 2}, // never holds a seat
	}

	cases := []struct {
		name     string
		usage    entity.SeatUsage
		license  *entity.License
		wantBody string
	}{
		{
			// Also the exactly-on-the-cap boundary: occupied equals the ceiling, the
			// state where the next seat invite is rejected. Legal state, plain 200 —
			// the handler never reads reaching the ceiling as an error.
			name:    "capped license reports the ceiling and the split",
			usage:   seatUsage,
			license: &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr[int64](5)},
			wantBody: `{"seats_purchased":5,"seats_used":4,"seats_pending":1,` +
				`"seats_occupied":5,"unlimited":false}`,
		},
		{
			name:    "no license: null ceiling, counters still reported",
			usage:   seatUsage,
			license: nil,
			wantBody: `{"seats_purchased":null,"seats_used":4,"seats_pending":1,` +
				`"seats_occupied":5,"unlimited":true}`,
		},
		{
			name:    "ceiling not reported yet: null and unlimited",
			usage:   seatUsage,
			license: &entity.License{Status: entity.LicenseStatusActive},
			wantBody: `{"seats_purchased":null,"seats_used":4,"seats_pending":1,` +
				`"seats_occupied":5,"unlimited":true}`,
		},
		{
			name:    "overage is reported as is",
			usage:   entity.SeatUsage{Admin: entity.SeatBucket{Active: 6, Pending: 1}},
			license: &entity.License{Status: entity.LicenseStatusActive, SeatsPurchased: lo.ToPtr[int64](5)},
			wantBody: `{"seats_purchased":5,"seats_used":6,"seats_pending":1,` +
				`"seats_occupied":7,"unlimited":false}`,
		},
		{
			name:  "self-hosted: zeros and no ceiling",
			usage: entity.SeatUsage{},
			wantBody: `{"seats_purchased":null,"seats_used":0,"seats_pending":0,` +
				`"seats_occupied":0,"unlimited":true}`,
		},
		{
			// A blocked license still caps seats and still has counts worth reading.
			// Blocking governs whether the org may CHANGE things; it must not alter
			// what this read reports, which is the whole reason the route sits
			// outside the block gate.
			name:    "blocked license reports its ceiling and counters unchanged",
			usage:   seatUsage,
			license: &entity.License{Status: entity.LicenseStatusBlocked, SeatsPurchased: lo.ToPtr[int64](5)},
			wantBody: `{"seats_purchased":5,"seats_used":4,"seats_pending":1,` +
				`"seats_occupied":5,"unlimited":false}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			impl, licenseSrv := initImplWithLicense(t)
			licenseSrv.EXPECT().SeatsUsage(gomock.Any()).Return(tc.usage, tc.license, nil)

			c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
			require.NoError(t, impl.GetSeats(c))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			// Asserted against the raw body, not a decoded struct: null vs 0 in
			// seats_purchased is exactly the distinction an int64 field would hide,
			// and the expected literal also pins the field set the client reads.
			require.JSONEq(t, tc.wantBody, rec.Body.String())
		})
	}
}

// TestGetSeatsStoreFailure pins the fail-closed choice: unreadable counts must
// surface as an error, never as zeros, which an admin would read as free seats.
func TestGetSeatsStoreFailure(t *testing.T) {
	t.Parallel()

	impl, licenseSrv := initImplWithLicense(t)
	licenseSrv.EXPECT().SeatsUsage(gomock.Any()).
		Return(entity.SeatUsage{}, nil, errors.New("collect seat usage: db down"))

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	require.NoError(t, impl.GetSeats(c))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "seats_occupied")
}
