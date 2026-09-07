package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruko1202/maintmode/internal/entity"
)

// TestDanceProvider pins the allow-list, which is narrower than
// ParseAuthMethod's on purpose.
//
// github and email parse as login methods and will one day be real ones, but
// neither has an authorization-code flow behind it today. Accepting them here
// would register a dance the backend cannot finish, and the {provider} segment
// shares a path space with the static /login/oauth/exchange/google — so an
// unvalidated parameter is how a request for one route ends up served by
// another.
func TestDanceProvider(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		segment string
		want    entity.AuthMethod
		ok      bool
	}{
		"google is the only dance provider today": {segment: "google", want: entity.AuthMethodGoogle, ok: true},
		"github parses but has no dance":          {segment: "github"},
		"email parses but has no dance":           {segment: "email"},
		"stub is never accepted from a request":   {segment: "stub"},
		"bootstrap likewise":                      {segment: "bootstrap"},
		"unknown":                                 {segment: "definitely-not-a-provider"},
		"empty":                                   {segment: ""},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, ok := entity.DanceProvider(tt.segment)

			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
