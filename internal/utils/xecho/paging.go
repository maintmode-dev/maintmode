package xecho

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v5"
)

const (
	// MaxPagingOffset caps offset pagination depth across every listing.
	// OFFSET in Postgres is linear: the database walks and discards every skipped
	// row, so without a ceiling the client dictates how much work the DB does.
	// 10_000 is far deeper than any meaningful browsing (page 200 at limit=50).
	MaxPagingOffset int64 = 10_000

	// defaultPagingLimit and defaultPagingMaxLimit are the page size and its
	// ceiling, identical in four listings out of five. They live here so that 200
	// is not repeated in every package as if it were a per-endpoint decision.
	defaultPagingLimit    int64 = 50
	defaultPagingMaxLimit int64 = 200
)

// ErrUnparseable means the value is not a number. Keeping it apart from
// out-of-range is deliberate, and it is the principle along which the listings
// diverge here: an unparseable value is a client mistake, and an endpoint is
// entitled to reject it; an out-of-range value is a request past the edge of
// what is possible, and it has a correct answer. That is why audit answers 400
// to "abc" yet silently fixes up limit=101, while the four read-only listings
// stay quiet in both cases. The two branches cannot be merged into one error:
// audit would start returning 400 for limit=101.
//
// The error carries the parameter name and is fit to be used as a 400 body
// verbatim: text of the form "invalid limit" / "invalid offset" is pinned by
// the audit tests, so the wrapping handler need neither unwrap the sentinel nor
// assemble the message itself.
var ErrUnparseable = errors.New("invalid")

// Paging holds the parsed paging parameters, already coerced into the allowed
// range: Offset in [0, maxOffset], Limit in [1, maxLimit].
type Paging struct {
	Limit  int64
	Offset int64
}

// pagingConfig holds per-call paging settings assembled from PagingOptions.
type pagingConfig struct {
	defaultLimit int64
	maxLimit     int64
	maxOffset    int64
}

// PagingOption tweaks a single PagingParams call. Unset options keep the
// defaults (limit 50, max 200, offset cap MaxPagingOffset), so an ordinary
// listing needs no options at all.
type PagingOption func(*pagingConfig)

// WithDefaultLimit overrides the default page size (50).
func WithDefaultLimit(def int64) PagingOption {
	return func(c *pagingConfig) { c.defaultLimit = def }
}

// WithMaxLimit overrides the page size ceiling (200 by default).
// Needed where a page is more expensive: audit serves at most 100.
func WithMaxLimit(maxLimit int64) PagingOption {
	return func(c *pagingConfig) { c.maxLimit = maxLimit }
}

// WithMaxOffset LOWERS offset pagination depth below MaxPagingOffset. It cannot
// raise it: a value above the global ceiling is ignored, otherwise a caller
// could undo the very limit this package exists for. A comment would not have
// held that boundary — the option is exported, and the five listings copy from
// one another.
//
// Production callers have no reason to use it — a single ceiling for all
// listings is a deliberate choice. It exists for the tests: at the default
// ceiling, substituting cfg.maxOffset with the MaxPagingOffset constant is
// indistinguishable from correct behaviour.
func WithMaxOffset(maxOffset int64) PagingOption {
	return func(c *pagingConfig) { c.maxOffset = min(maxOffset, MaxPagingOffset) }
}

// PagingParams parses limit/offset. The returned Paging is always valid, so the
// caller is free to ignore the error.
//
// The coercion differs per parameter on purpose:
//   - limit outside [1, max] → def. Serving a smaller page is correct.
//   - offset > max → max (clamp, not def). A clamp never returns an earlier page
//     than the one requested — unlike a reset to 0, which always returns the
//     very first one.
//
// The boundary is the same in both cases: `> max`, the value max itself is valid.
//
// The only error is ErrUnparseable with the parameter name; out-of-range is not
// treated as an error (see the ErrUnparseable doc).
func PagingParams(c *echo.Context, opts ...PagingOption) (Paging, error) {
	cfg := pagingConfig{
		defaultLimit: defaultPagingLimit,
		maxLimit:     defaultPagingMaxLimit,
		maxOffset:    MaxPagingOffset,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	// The default is a limit value too, and the ceiling must hold it just like a
	// value coming from the query: otherwise WithDefaultLimit(500) on top of
	// WithMaxLimit(100) would serve 500 as the default page.
	cfg.defaultLimit = min(max(cfg.defaultLimit, 1), cfg.maxLimit)

	limit, limitErr := pagingLimit(c, cfg)
	offset, offsetErr := pagingOffset(c, cfg)
	paging := Paging{Limit: limit, Offset: offset}

	// Both parameters are always parsed so that Paging is valid as a whole, but
	// only one error escapes — about the first invalid parameter. That keeps the
	// text single-clause ("invalid limit"), as it was before the move to the helper.
	if limitErr != nil {
		return paging, limitErr
	}

	return paging, offsetErr
}

// pagingLimit always returns a valid page size: on an unparseable value
// echo.QueryParamOr yields the zero value of the type rather than the default,
// so the default is applied explicitly here.
func pagingLimit(c *echo.Context, cfg pagingConfig) (int64, error) {
	limit, err := echo.QueryParamOr[int64](c, "limit", cfg.defaultLimit)
	if err != nil {
		return cfg.defaultLimit, fmt.Errorf("%w limit", ErrUnparseable)
	}

	if limit <= 0 || limit > cfg.maxLimit {
		return cfg.defaultLimit, nil
	}

	return limit, nil
}

// pagingOffset always returns a valid position: below zero is the start of the
// set, above the ceiling is the deepest available page rather than the start.
func pagingOffset(c *echo.Context, cfg pagingConfig) (int64, error) {
	offset, err := echo.QueryParamOr[int64](c, "offset", 0)
	if err != nil {
		return 0, fmt.Errorf("%w offset", ErrUnparseable)
	}

	if offset < 0 {
		return 0, nil
	}

	if offset > cfg.maxOffset {
		return cfg.maxOffset, nil
	}

	return offset, nil
}
