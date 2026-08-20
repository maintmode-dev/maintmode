package xecho

import (
	"net/url"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"
)

func TestPagingParams(t *testing.T) {
	t.Parallel()

	t.Run("offset", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name            string
			raw             string
			opts            []PagingOption
			wantOffset      int64
			wantUnparseable bool
		}{
			{
				name:       "absent",
				wantOffset: 0,
			}, {
				name:       "zero",
				raw:        "0",
				wantOffset: 0,
			}, {
				name:       "one",
				raw:        "1",
				wantOffset: 1,
			}, {
				// A negative position is out-of-range, not an input error:
				// it is silently coerced to the start of the set.
				name:       "negative coerced to zero without error",
				raw:        "-5",
				wantOffset: 0,
			}, {
				name:            "unparseable falls back to zero",
				raw:             "abc",
				wantOffset:      0,
				wantUnparseable: true,
			}, {
				name:       "exactly max is valid",
				raw:        "10000",
				wantOffset: MaxPagingOffset,
			}, {
				// Clamped to max, not to 0: a reset to the start would return
				// the very first page instead of the one nearest to requested.
				name:       "max plus one clamps to max",
				raw:        "10001",
				wantOffset: MaxPagingOffset,
			}, {
				name:       "far beyond max clamps to max",
				raw:        "100000000",
				wantOffset: MaxPagingOffset,
			}, {
				// The ceiling here differs from the default one: these three
				// cases catch a clamp substituted with the MaxPagingOffset
				// constant and an option dropped along the way — at the default
				// ceiling both defects are indistinguishable from correct
				// behavior.
				//
				// What this trio does not catch is the comparison shifted to
				// `>=`: the clamp yields exactly the ceiling, so at offset ==
				// max both versions produce the same number. That is an
				// equivalent mutation, not a coverage gap; nothing can kill it.
				name:       "custom max: just below max passes through unclamped",
				raw:        "9",
				opts:       []PagingOption{WithMaxOffset(10)},
				wantOffset: 9,
			}, {
				name:       "custom max: exactly max is valid",
				raw:        "10",
				opts:       []PagingOption{WithMaxOffset(10)},
				wantOffset: 10,
			}, {
				name:       "custom max: max plus one clamps to max",
				raw:        "11",
				opts:       []PagingOption{WithMaxOffset(10)},
				wantOffset: 10,
			}, {
				// The option can only lower the ceiling. An attempt to raise it
				// above the global one is ignored — otherwise a caller would
				// undo the very limit this package exists for.
				name:       "custom max above the global cap is ignored",
				raw:        "1000000",
				opts:       []PagingOption{WithMaxOffset(1_000_000)},
				wantOffset: MaxPagingOffset,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				paging, err := PagingParams(contextWith(t, "offset", tc.raw), tc.opts...)

				requireUnparseable(t, err, tc.wantUnparseable, "offset")
				require.Equal(t, tc.wantOffset, paging.Offset)
				// Parsing one parameter does not spoil the other.
				require.Equal(t, defaultPagingLimit, paging.Limit)
			})
		}
	})

	t.Run("limit", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name            string
			raw             string
			opts            []PagingOption
			wantLimit       int64
			wantUnparseable bool
		}{
			{
				name:      "absent falls back to default",
				wantLimit: defaultPagingLimit,
			}, {
				name:      "one",
				raw:       "1",
				wantLimit: 1,
			}, {
				name:      "exactly default max is valid",
				raw:       "200",
				wantLimit: 200,
			}, {
				// The key difference from offset: exceeding the ceiling yields
				// the default, not a clamp to the ceiling.
				name:      "above default max falls back to default not max",
				raw:       "201",
				wantLimit: defaultPagingLimit,
			}, {
				name:      "zero falls back to default",
				raw:       "0",
				wantLimit: defaultPagingLimit,
			}, {
				name:      "negative falls back to default",
				raw:       "-1",
				wantLimit: defaultPagingLimit,
			}, {
				name:            "unparseable falls back to default",
				raw:             "abc",
				wantLimit:       defaultPagingLimit,
				wantUnparseable: true,
			}, {
				// The audit configuration: 100 serves as both default and ceiling.
				name:      "custom max exactly at ceiling is valid",
				raw:       "100",
				opts:      []PagingOption{WithDefaultLimit(100), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				// A direct regression guard for audit: 101 must yield 100 and
				// must NOT be an error.
				name:      "custom max plus one falls back to custom default",
				raw:       "101",
				opts:      []PagingOption{WithDefaultLimit(100), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				name:            "custom default applies to unparseable",
				raw:             "abc",
				opts:            []PagingOption{WithDefaultLimit(100), WithMaxLimit(100)},
				wantLimit:       100,
				wantUnparseable: true,
			}, {
				// No caller has the def != max combination: without it the
				// contract would rest on the numbers coinciding, not on the
				// definition.
				name:      "def differs from max: violation yields def not max",
				raw:       "101",
				opts:      []PagingOption{WithDefaultLimit(50), WithMaxLimit(100)},
				wantLimit: 50,
			}, {
				name:      "def differs from max: value within range passes through",
				raw:       "100",
				opts:      []PagingOption{WithDefaultLimit(50), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				// A default above the ceiling is a caller misconfiguration; the
				// ceiling must hold it too, otherwise the helper serves a page
				// larger than the one it exists to cap.
				name:      "def above max is capped at max",
				opts:      []PagingOption{WithDefaultLimit(500), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				name:      "def above max is capped when value is out of range too",
				raw:       "101",
				opts:      []PagingOption{WithDefaultLimit(500), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				// The lower bound of the same invariant: a default of 0 or less
				// would yield an empty page for any request without limit.
				name:      "def below one is raised to one",
				opts:      []PagingOption{WithDefaultLimit(0)},
				wantLimit: 1,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				paging, err := PagingParams(contextWith(t, "limit", tc.raw), tc.opts...)

				requireUnparseable(t, err, tc.wantUnparseable, "limit")
				require.Equal(t, tc.wantLimit, paging.Limit)
				require.Equal(t, int64(0), paging.Offset)
			})
		}
	})

	t.Run("both params parsed together", func(t *testing.T) {
		t.Parallel()

		c := echotest.ContextConfig{
			QueryValues: url.Values{"limit": {"10"}, "offset": {"100000000"}},
		}.ToContext(t)

		paging, err := PagingParams(c)
		require.NoError(t, err)
		require.Equal(t, int64(10), paging.Limit)
		require.Equal(t, MaxPagingOffset, paging.Offset)
	})

	t.Run("both unparseable reports limit first", func(t *testing.T) {
		t.Parallel()

		c := echotest.ContextConfig{
			QueryValues: url.Values{"limit": {"abc"}, "offset": {"xyz"}},
		}.ToContext(t)

		// The text is single-clause: the caller drops it into a 400 as is.
		paging, err := PagingParams(c)
		require.ErrorIs(t, err, ErrUnparseable)
		require.Equal(t, "invalid limit", err.Error())

		// Paging is valid as a whole even when both parameters are invalid.
		require.Equal(t, defaultPagingLimit, paging.Limit)
		require.Equal(t, int64(0), paging.Offset)
	})
}

// contextWith builds a request context carrying a single paging query param;
// an empty raw value means the param is absent altogether.
func contextWith(t *testing.T, name, raw string) *echo.Context {
	t.Helper()

	conf := echotest.ContextConfig{}
	if raw != "" {
		conf.QueryValues = url.Values{name: {raw}}
	}

	return conf.ToContext(t)
}

// requireUnparseable asserts both the sentinel and the exact message, because
// the message itself is the audit 400 body.
func requireUnparseable(t *testing.T, err error, want bool, param string) {
	t.Helper()

	if !want {
		require.NoError(t, err)
		return
	}

	require.ErrorIs(t, err, ErrUnparseable)
	require.Equal(t, "invalid "+param, err.Error())
}
