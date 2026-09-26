package xemail

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEqualIgnoreCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "identical", a: "invitee@example.com", b: "invitee@example.com", want: true},
		{name: "case differs", a: "Invitee@Example.COM", b: "invitee@example.com", want: true},
		{name: "different local part", a: "attacker@example.com", b: "invitee@example.com", want: false},
		{name: "different domain", a: "invitee@example.org", b: "invitee@example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, EqualIgnoreCase(tt.a, tt.b))
		})
	}
}
