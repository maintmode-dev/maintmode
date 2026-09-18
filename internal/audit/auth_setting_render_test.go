package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// The toggle must render with its OWN entity type.
//
// This is the assertion that catches the arm's one silent failure: Render seeds
// the payload with AuditEntityTypeUser and every other arm of fillAuthPayload
// relies on that default, so an arm that forgot to set it would produce a
// well-formed row filed against the admin instead of the method -- correct
// action, correct category, correct details, wrong entity, no symptom.
func TestRender_AuthMethodToggled(t *testing.T) {
	t.Parallel()

	actor := &entity.User{ID: uuid.New(), Email: "admin@example.com", Name: "Admin"}
	r := fixedRenderer(uuid.New(), time.Now())

	tests := []struct {
		name        string
		enabled     bool
		wantInDetai string
	}{
		{"disabled", false, "disabled"},
		{"enabled", true, "enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := r.Render(AuthMethodToggled{
				Actor:   actor,
				Method:  entity.AuthMethodNameEmailOTP,
				Enabled: tt.enabled,
			})
			require.NoError(t, err)

			require.Equal(t, entity.AuditActionAuthMethodToggled, payload.Action)
			require.Equal(t, entity.AuditEntityTypeAuthSetting, payload.EntityType,
				"must not inherit the user default from Render")
			require.Equal(t, "email_otp", payload.EntityID)
			require.Contains(t, payload.Details, tt.wantInDetai)
			require.Contains(t, payload.Details, "email_otp")
			require.Contains(t, payload.Details, actor.Email)
		})
	}
}

// A sign-in refused for a disabled method must carry its reason all the way
// into the rendered payload, and out through the metadata the audit API binds.
//
// The spec is explicit that asserting only "the row was published with this
// reason" would pass against an implementation whose reason never reaches a
// reader. The service-level tests stop at the published action; this one
// continues through the renderer, which is the last place the value can be
// dropped before it becomes a plain string on the wire.
//
// The details column is deliberately NOT checked for the method: every login
// failure renders the same sentence on purpose, so that the details string
// cannot become the oracle the metadata rule prevents. The reason lives in the
// metadata, and that is the contract.
func TestRender_LoginFailedCarriesMethodDisabledReason(t *testing.T) {
	t.Parallel()

	r := fixedRenderer(uuid.New(), time.Now())

	payload, err := r.Render(LoginFailed{
		User: &entity.User{Email: "someone@example.com"},
		Meta: &entity.AuditMetadata{
			IP:            "10.0.0.1",
			FailureReason: entity.AuditFailureMethodDisabled,
		},
	})
	require.NoError(t, err)

	require.Equal(t, entity.AuditActionLoginFailed, payload.Action)
	require.NotNil(t, payload.Metadata)
	require.Equal(t, entity.AuditFailureMethodDisabled, payload.Metadata.FailureReason,
		"the reason must survive rendering, or an operator never sees it")

	// The value the API binds is this string, so pin the spelling too: a rename
	// would change a field operators filter on.
	require.Equal(t, "method disabled", string(payload.Metadata.FailureReason))
}
