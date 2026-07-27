package render

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/ruko1202/maintmode/internal/entity"
)

// degradedCount reads the owner-mention degradation counter for one reason.
//
// The metrics package registers its instruments against the global meter
// provider at package init, so the reader has to be installed before that
// happens — TestMain below does it. Without that, every count reads zero and
// these assertions would pass no matter what the code does.
func degradedCount(t *testing.T, reader sdkmetric.Reader, reason string) int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var total int64

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "maint_notify_owner_mention_degraded_total" {
				continue
			}

			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}

			for _, dp := range sum.DataPoints {
				if v, found := dp.Attributes.Value("reason"); found && v.AsString() == reason {
					total += dp.Value
				}
			}
		}
	}

	return total
}

// TestOwnerMentionDegradationCounted pins which degradations are counted and,
// just as importantly, which are not. An empty handle is the feature's
// switched-off state — counting it would keep a rate>0 alert permanently red.
func TestOwnerMentionDegradationCounted(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		mention        *entity.UserMention
		reason         string
		wantIncrements int64
	}{
		{
			name: "rejected handle is counted",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("foo\nbar"),
			},
			reason:         "rejected",
			wantIncrements: 1,
		},
		{
			name:           "unresolved owner is counted",
			mention:        &entity.UserMention{Name: entity.UnknownUserName},
			reason:         "unresolved",
			wantIncrements: 1,
		},
		{
			name:           "absent handle is NOT counted as rejected",
			mention:        &entity.UserMention{Name: "Ruslan Kosykh"},
			reason:         "rejected",
			wantIncrements: 0,
		},
		{
			name:           "absent handle is NOT counted as unresolved",
			mention:        &entity.UserMention{Name: "Ruslan Kosykh"},
			reason:         "unresolved",
			wantIncrements: 0,
		},
		{
			name: "empty handle is NOT counted",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr(""),
			},
			reason:         "rejected",
			wantIncrements: 0,
		},
		{
			name: "a good handle is NOT counted",
			mention: &entity.UserMention{
				Name:     "Ruslan Kosykh",
				SlackTag: ptr("ruslan.slack"),
			},
			reason:         "rejected",
			wantIncrements: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := degradedCount(t, testMetricReader, tt.reason)

			got := ownerMention(ctx, entity.NotifyTransportSlack, tt.mention)

			after := degradedCount(t, testMetricReader, tt.reason)
			assert.Equal(t, tt.wantIncrements, after-before)

			// Whatever the outcome, delivery text never ends up empty for a
			// present owner: the mention degrades to the name, never to nothing.
			assert.NotEmpty(t, got)
		})
	}
}

// TestOwnerMentionNilIsEmpty covers the step-event case: no owner at all yields
// the empty string, which makes the template skip the whole block.
func TestOwnerMentionNilIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ownerMention(context.Background(), entity.NotifyTransportSlack, nil))
}

// TestOwnerMentionUnknownTransportUsesName pins the default branch. Only slack
// and telegram are reachable today; this guards the shape for a future one.
func TestOwnerMentionUnknownTransportUsesName(t *testing.T) {
	t.Parallel()

	got := ownerMention(context.Background(), entity.NotifyTransport("future"), &entity.UserMention{
		Name:        "Ruslan Kosykh",
		TelegramTag: ptr("@ruslan_tg"),
		SlackTag:    ptr("ruslan.slack"),
	})

	assert.Equal(t, "Ruslan Kosykh", got)
}

// testMetricReader is installed as the global meter provider before the metrics
// package registers its instruments, so counter assertions observe real values.
var testMetricReader sdkmetric.Reader

func TestMain(m *testing.M) {
	testMetricReader = sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(testMetricReader)))

	m.Run()
}
