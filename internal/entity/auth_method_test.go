package entity_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestPrimaryAuthMethod(t *testing.T) {
	t.Parallel()

	t.Run("returns the first method when present", func(t *testing.T) {
		t.Parallel()
		got := entity.PrimaryAuthMethod([]entity.AuthMethod{
			entity.AuthMethodGithub,
			entity.AuthMethodGoogle,
		})
		require.Equal(t, entity.AuthMethodGithub, got)
	})

	t.Run("falls back to unknown for empty input", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, entity.AuthMethodUnknown, entity.PrimaryAuthMethod(nil))
		require.Equal(t, entity.AuthMethodUnknown, entity.PrimaryAuthMethod([]entity.AuthMethod{}))
	})
}
