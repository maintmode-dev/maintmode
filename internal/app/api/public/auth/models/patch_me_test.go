package apiauthmodels

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestUpdateMeRequestUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("present keys are recorded and decoded", func(t *testing.T) {
		t.Parallel()

		var req UpdateMeRequest
		require.NoError(t, json.Unmarshal([]byte(`{"timezone":"Asia/Nicosia","slack_tag":"ruslan"}`), &req))

		require.True(t, req.Has(FieldTimezone))
		require.True(t, req.Has(FieldSlackTag))
		require.False(t, req.Has(FieldTelegramTag))

		require.Equal(t, "Asia/Nicosia", lo.FromPtr(req.Timezone))
		require.Equal(t, "ruslan", lo.FromPtr(req.SlackTag))
		require.Nil(t, req.TelegramTag)
	})

	t.Run("explicit null is present but nil", func(t *testing.T) {
		t.Parallel()

		var req UpdateMeRequest
		require.NoError(t, json.Unmarshal([]byte(`{"timezone":null}`), &req))

		// The whole point of the presence set: null and absent both decode to a
		// nil pointer, and only Has tells them apart.
		require.True(t, req.Has(FieldTimezone))
		require.Nil(t, req.Timezone)
		require.False(t, req.Has(FieldSlackTag))
	})

	t.Run("empty object marks nothing present", func(t *testing.T) {
		t.Parallel()

		var req UpdateMeRequest
		require.NoError(t, json.Unmarshal([]byte(`{}`), &req))

		require.False(t, req.Has(FieldTimezone))
		require.False(t, req.Has(FieldTelegramTag))
		require.False(t, req.Has(FieldSlackTag))
	})

	t.Run("never-unmarshalled request behaves like an empty object", func(t *testing.T) {
		t.Parallel()

		// Echo returns from BindBody before deserializing when Content-Length is
		// 0, leaving the presence set nil. A nil map must answer exactly like the
		// empty one above.
		var req UpdateMeRequest
		require.Nil(t, req.present)

		require.False(t, req.Has(FieldTimezone))
		require.False(t, req.Has(FieldTelegramTag))
		require.False(t, req.Has(FieldSlackTag))
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		t.Parallel()

		var req UpdateMeRequest
		require.NoError(t, json.Unmarshal([]byte(`{"nope":1,"telegram_tag":"@ruslan"}`), &req))

		require.True(t, req.Has(FieldTelegramTag))
		require.False(t, req.Has(FieldTimezone))
	})

	t.Run("malformed and wrong-typed bodies error", func(t *testing.T) {
		t.Parallel()

		for _, body := range []string{`{"timezone":`, `{"timezone":123}`, `[]`} {
			var req UpdateMeRequest
			require.Error(t, json.Unmarshal([]byte(body), &req), body)
		}
	})
}
