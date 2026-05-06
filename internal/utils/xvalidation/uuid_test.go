package xvalidation

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUUIDNotNil(t *testing.T) {
	t.Parallel()

	t.Run("uuid", func(t *testing.T) {
		t.Parallel()

		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			u := uuid.New()

			err := UUIDNotNil(u)
			require.NoError(t, err)
		})

		t.Run("invalid", func(t *testing.T) {
			t.Parallel()
			u := uuid.Nil

			err := UUIDNotNil(u)
			require.Error(t, err)
			require.EqualError(t, err, validation.ErrNilOrNotEmpty.Error())
		})
	})

	t.Run("str", func(t *testing.T) {
		t.Parallel()

		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			u := uuid.NewString()

			err := UUIDNotNil(u)
			require.NoError(t, err)
		})

		t.Run("invalid", func(t *testing.T) {
			t.Parallel()
			u := uuid.Nil.String()

			err := UUIDNotNil(u)
			require.Error(t, err)
			require.EqualError(t, err, validation.ErrNilOrNotEmpty.Error())
		})
	})
}
