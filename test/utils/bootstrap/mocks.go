package testbootstraputils

import (
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/entity"
	mock_bootstrap "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/app/bootstrap"
)

// Mocks holds the generated mocks for the external dependencies a test wires
// into the bootstrap. It currently carries only the auth gateway; add fields
// here as more gateways gain mockable interfaces.
type Mocks struct {
	Auth *mock_bootstrap.MockAuthServiceGateway
}

// NewMocks builds the mock set bound to ctrl, each pre-seeded with permissive
// defaults so DB-backed integration tests run offline without per-test wiring:
// every approver is eligible, user lookups are empty, introspection returns nil.
// Tests that need specific behavior append their own EXPECT() overrides on the
// returned mocks (gomock matches the most recent matching expectation first).
func NewMocks(ctrl *gomock.Controller) *Mocks {
	auth := mock_bootstrap.NewMockAuthServiceGateway(ctrl)
	auth.EXPECT().IsEligibleApprover(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	auth.EXPECT().ListActiveUsers(gomock.Any(), gomock.Any()).
		Return(&entity.ListAssignableUsersResult{}, nil).AnyTimes()
	auth.EXPECT().GetUsersByIDs(gomock.Any(), gomock.Any()).
		Return(map[uuid.UUID]*entity.User{}, nil).AnyTimes()
	auth.EXPECT().Introspect(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	return &Mocks{Auth: auth}
}
