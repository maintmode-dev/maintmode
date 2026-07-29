package xsql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapeLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is untouched", "ivan", "ivan"},
		{"percent loses its wildcard meaning", "%", `\%`},
		{"underscore loses its wildcard meaning", "_", `\_`},
		{"a backslash escapes itself", `\`, `\\`},
		{"metacharacters inside a word", "ivan_petrov", `ivan\_petrov`},
		{"empty stays empty", "", ""},
		// The backslash must be escaped first: doing it last would re-escape the
		// backslashes introduced for "%" and "_" and leak a wildcard back through.
		{"an escaped percent is not re-escaped into a wildcard", `\%`, `\\\%`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, EscapeLike(tt.in))
		})
	}
}
