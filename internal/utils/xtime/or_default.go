package xtime

import "time"

// OrDefaultDuration resolves a configured duration against a fallback, with a
// third state that cmp.Or cannot express:
//
//	positive → the configured value
//	zero     → def, i.e. the field was omitted from config
//	negative → 0, an explicit "no limit"
//
// The negative case is why this exists. cmp.Or(cfg.Timeout, defaultTimeout) —
// used elsewhere in this repo for plain client timeouts — maps zero to the
// default and has no spelling for "I deliberately want none", which matters
// wherever 0 already means unlimited to the consumer. net/http deadlines are
// exactly that: a zero http.Server.WriteTimeout is no deadline, so "unset" and
// "unlimited" would otherwise be the same value with opposite intent.
//
// Note that a negative value must be normalized to 0 rather than passed
// through. A negative http.Server deadline is not unlimited — it is a deadline
// already in the past, which fails every request on the connection.
func OrDefaultDuration(v, def time.Duration) time.Duration {
	switch {
	case v > 0:
		return v
	case v < 0:
		return 0
	default:
		return def
	}
}
