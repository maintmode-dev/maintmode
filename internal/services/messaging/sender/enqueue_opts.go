package messagesender

import (
	"time"
)

type enqueueOpts struct {
	idempotencyKey string
	delay          time.Duration
}

// EnqueueOption configures a SendAsync / SendDelayed call.
type EnqueueOption func(*enqueueOpts)

// WithIdempotencyKey makes goque dedupe retries of the same logical
// message via its unique (type, external_id) index.
func WithIdempotencyKey(key string) EnqueueOption {
	return func(o *enqueueOpts) {
		if key != "" {
			o.idempotencyKey = key
		}
	}
}

func WithDelay(delay time.Duration) EnqueueOption {
	return func(o *enqueueOpts) {
		if delay > 0 {
			o.delay = delay
		}
	}
}
