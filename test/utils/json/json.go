package testjsonudils

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func AnyToJSON(t *testing.T, v any) string {
	t.Helper()

	b := AnyToJSONBytes(t, v)
	return string(b)
}

func AnyToJSONBytes(t *testing.T, v any) []byte {
	t.Helper()

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(v)
	require.NoError(t, err)

	return buf.Bytes()
}

func JSONToAny[T any](t *testing.T, v io.Reader) T {
	t.Helper()

	var out T

	err := json.NewDecoder(v).Decode(&out)
	require.NoError(t, err)

	return out
}
