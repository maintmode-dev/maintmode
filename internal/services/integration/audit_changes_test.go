package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestConfigChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before string
		after  string
		want   []entity.AuditFieldChange
	}{
		{
			name:   "a moved value is recorded before and after",
			before: `{"api_url":"https://a.test","timeout":"5s"}`,
			after:  `{"api_url":"https://b.test","timeout":"5s"}`,
			want:   []entity.AuditFieldChange{{Field: "api_url", Old: "https://a.test", New: "https://b.test"}},
		},
		{
			name:   "added and removed keys have an empty side",
			before: `{"host":"smtp.a.test"}`,
			after:  `{"port":587}`,
			want: []entity.AuditFieldChange{
				{Field: "host", Old: "smtp.a.test"},
				{Field: "port", New: "587"},
			},
		},
		{
			name:   "non-string values are compact JSON",
			before: `{"allowed_hosted_domains":["a.test"]}`,
			after:  `{"allowed_hosted_domains": ["a.test", "b.test"]}`,
			want: []entity.AuditFieldChange{
				{Field: "allowed_hosted_domains", Old: `["a.test"]`, New: `["a.test","b.test"]`},
			},
		},
		{
			name:   "formatting alone is not a change",
			before: `{"port":587,"host":"h"}`,
			after:  `{ "host": "h", "port": 587 }`,
			want:   []entity.AuditFieldChange{},
		},
		{
			name:   "from no config",
			before: ``,
			after:  `{"api_url":"https://b.test"}`,
			want:   []entity.AuditFieldChange{{Field: "api_url", New: "https://b.test"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := configChanges(json.RawMessage(tt.before), json.RawMessage(tt.after))
			assert.Equal(t, tt.want, got)
		})
	}
}
