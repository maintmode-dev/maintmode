package entity_test

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestCanonicalTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       *string
		wantName string
		wantOK   bool
	}{
		{name: "nil resets", in: nil, wantName: "", wantOK: true},
		{name: "empty resets", in: lo.ToPtr(""), wantName: "", wantOK: true},
		{name: "whitespace resets", in: lo.ToPtr("   "), wantName: "", wantOK: true},
		{name: "valid IANA", in: lo.ToPtr("Asia/Nicosia"), wantName: "Asia/Nicosia", wantOK: true},
		{name: "valid IANA Europe", in: lo.ToPtr("Europe/Berlin"), wantName: "Europe/Berlin", wantOK: true},
		{name: "valid UTC", in: lo.ToPtr("UTC"), wantName: "UTC", wantOK: true},
		{name: "trims surrounding space", in: lo.ToPtr("  Europe/Berlin  "), wantName: "Europe/Berlin", wantOK: true},
		{name: "Local rejected (machine zone guard)", in: lo.ToPtr("Local"), wantOK: false},
		{name: "padded Local still rejected", in: lo.ToPtr("  Local  "), wantOK: false},
		{name: "lowercase local rejected by LoadLocation", in: lo.ToPtr("local"), wantOK: false},
		{name: "unknown zone rejected", in: lo.ToPtr("Mars/Phobos"), wantOK: false},
		{name: "numeric offset rejected", in: lo.ToPtr("+03:00"), wantOK: false},
		{name: "path traversal rejected", in: lo.ToPtr("../../etc/passwd"), wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotName, gotOK := entity.CanonicalTimezone(tt.in)
			require.Equal(t, tt.wantOK, gotOK)
			require.Equal(t, tt.wantName, gotName)
		})
	}
}
