package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// stubAuthSettings answers the built-in flags from a fixed map, or fails every
// read to drive the fail-closed and degradation paths.
type stubAuthSettings struct {
	enabled map[entity.AuthMethodName]bool
	// unreadable fails BOTH reads, because what it models is "the table did not
	// answer" -- not a row that is individually broken. Both have to fail from
	// one switch: the gate reads Enabled and the listing reads List, and those
	// two answer the same error in opposite directions, so a stub that failed
	// only one would leave the other's branch untested.
	unreadable bool
}

func (s stubAuthSettings) List(_ context.Context) ([]*entity.AuthMethodSetting, error) {
	if s.unreadable {
		return nil, errors.New("auth_settings unreadable")
	}

	out := make([]*entity.AuthMethodSetting, 0, len(s.enabled))
	for method, enabled := range s.enabled {
		out = append(out, &entity.AuthMethodSetting{Method: method, Enabled: enabled})
	}

	return out, nil
}

func (s stubAuthSettings) Enabled(_ context.Context, method entity.AuthMethodName) (bool, error) {
	if s.unreadable {
		return false, errors.New("auth_settings unreadable")
	}

	return s.enabled[method], nil
}

// idsOf decodes the response and returns the method ids in order.
func idsOf(t *testing.T, body string) []string {
	t.Helper()

	var decoded struct {
		Methods []struct {
			ID string `json:"id"`
		} `json:"methods"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &decoded))

	ids := make([]string, 0, len(decoded.Methods))
	for _, m := range decoded.Methods {
		ids = append(ids, m.ID)
	}

	return ids
}
