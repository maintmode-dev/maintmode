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
				// Отрицательная позиция — выход за диапазон, а не ошибка ввода:
				// приводится к началу набора молча.
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
				// Кламп в max, а не в 0: сброс в начало вернул бы самую
				// первую страницу вместо ближайшей к запрошенной.
				name:       "max plus one clamps to max",
				raw:        "10001",
				wantOffset: MaxPagingOffset,
			}, {
				name:       "far beyond max clamps to max",
				raw:        "100000000",
				wantOffset: MaxPagingOffset,
			}, {
				// Потолок здесь не совпадает с дефолтным: эти три кейса ловят
				// подмену клампа константой MaxPagingOffset и потерю опции по
				// дороге — на дефолтном потолке оба дефекта неотличимы от
				// корректного поведения.
				//
				// Чего эта тройка не ловит — сдвиг сравнения на `>=`: кламп
				// отдаёт ровно потолок, поэтому на offset == max обе версии
				// дают одно число. Это эквивалентная мутация, а не пробел в
				// покрытии; убить её нельзя ничем.
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
				// Опция умеет только опускать потолок. Попытка поднять его выше
				// глобального игнорируется — иначе вызывающий отменял бы ровно
				// то ограничение, ради которого пакет существует.
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
				// Разбор одного параметра не портит второй.
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
				// Ключевое отличие от offset: превышение потолка даёт дефолт,
				// а не кламп в потолок.
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
				// Конфигурация audit: 100 служит и дефолтом, и потолком.
				name:      "custom max exactly at ceiling is valid",
				raw:       "100",
				opts:      []PagingOption{WithDefaultLimit(100), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				// Прямая защита audit от регресса: 101 обязан дать 100 и НЕ
				// обязан быть ошибкой.
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
				// Комбинации def != max нет ни у одного вызывающего: без неё
				// контракт держался бы совпадением чисел, а не определением.
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
				// Дефолт выше потолка — ошибка конфигурации вызывающего;
				// потолок обязан удержать и его, иначе хелпер отдаёт страницу
				// больше той, ради ограничения которой он и заведён.
				name:      "def above max is capped at max",
				opts:      []PagingOption{WithDefaultLimit(500), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				name:      "def above max is capped when value is out of range too",
				raw:       "101",
				opts:      []PagingOption{WithDefaultLimit(500), WithMaxLimit(100)},
				wantLimit: 100,
			}, {
				// Нижняя граница того же инварианта: дефолт 0 или ниже дал бы
				// пустую страницу на любом запросе без limit.
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

		// Текст односоставный: вызывающий подставляет его в 400 как есть.
		paging, err := PagingParams(c)
		require.ErrorIs(t, err, ErrUnparseable)
		require.Equal(t, "invalid limit", err.Error())

		// Paging валиден целиком, даже когда некорректны оба параметра.
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
