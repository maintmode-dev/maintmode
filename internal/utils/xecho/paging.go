package xecho

import (
	"errors"
	"fmt"

	"github.com/labstack/echo/v5"
)

const (
	// MaxPagingOffset ограничивает глубину offset-пагинации во всех листингах.
	// OFFSET в Postgres линеен: база проходит и отбрасывает все пропускаемые
	// строки, поэтому без потолка клиент задаёт объём работы БД сам. 10_000 —
	// заведомо глубже любого осмысленного просмотра (200-я страница при limit=50).
	MaxPagingOffset int64 = 10_000

	// defaultPagingLimit и defaultPagingMaxLimit — размер страницы и её потолок,
	// одинаковые у четырёх из пяти листингов. Вынесены сюда, чтобы 200 не
	// повторялось в каждом пакете, как будто это осознанный выбор эндпоинта.
	defaultPagingLimit    int64 = 50
	defaultPagingMaxLimit int64 = 200
)

// ErrUnparseable — значение не является числом. Отделено от выхода за
// диапазон намеренно, и вот принцип, по которому листинги здесь расходятся:
// нераспарсиваемое значение — ошибка клиента, и эндпоинт вправе её отвергнуть;
// выход за диапазон — запрос за границей возможного, на который есть
// корректный ответ. Поэтому audit отвечает 400 на "abc", но молча правит
// limit=101, а четыре read-only листинга молчат в обоих случаях.
// Слить ветки в один error нельзя: audit начнёт 400-ить limit=101.
//
// Ошибка несёт имя параметра и целиком годится в тело 400: текст вида
// "invalid limit" / "invalid offset" закреплён тестами audit, поэтому
// оборачивающему хендлеру не нужно ни разбирать сентинел, ни собирать
// сообщение самому.
var ErrUnparseable = errors.New("invalid")

// Paging — разобранные страничные параметры, уже приведённые к допустимому
// диапазону: Offset в [0, maxOffset], Limit в [1, maxLimit].
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

// WithDefaultLimit переопределяет размер страницы по умолчанию (50).
func WithDefaultLimit(def int64) PagingOption {
	return func(c *pagingConfig) { c.defaultLimit = def }
}

// WithMaxLimit переопределяет потолок размера страницы (по умолчанию 200).
// Нужен там, где страница дороже: audit отдаёт максимум 100.
func WithMaxLimit(maxLimit int64) PagingOption {
	return func(c *pagingConfig) { c.maxLimit = maxLimit }
}

// WithMaxOffset ОПУСКАЕТ глубину offset-пагинации ниже MaxPagingOffset. Поднять
// её этой опцией нельзя: значение выше глобального потолка игнорируется, иначе
// вызывающий мог бы отменить ровно то ограничение, ради которого пакет и
// существует. Комментарий такую границу не удержал бы — опция экспортирована,
// а пять листингов копируют друг у друга.
//
// У продакшн-вызывающих причин её звать нет — один потолок на все листинги
// осознанный выбор. Она нужна тестам: на дефолтном потолке подмена cfg.maxOffset
// константой MaxPagingOffset неотличима от корректного поведения.
func WithMaxOffset(maxOffset int64) PagingOption {
	return func(c *pagingConfig) { c.maxOffset = min(maxOffset, MaxPagingOffset) }
}

// PagingParams разбирает limit/offset. Возвращённый Paging валиден всегда,
// поэтому вызывающий волен ошибку проигнорировать.
//
// Приведение к диапазону различается по параметрам намеренно:
//   - limit вне [1, max] → def. Отдать меньшую страницу корректно.
//   - offset > max → max (кламп, не def). Кламп никогда не возвращает более
//     раннюю страницу, чем запрошено, — в отличие от сброса в 0, который
//     всегда возвращает самую первую.
//
// Граница в обоих случаях одна и та же: `> max`, само значение max валидно.
//
// Ошибка — только ErrUnparseable с именем параметра; выход за диапазон
// ошибкой не считается (см. док ErrUnparseable).
func PagingParams(c *echo.Context, opts ...PagingOption) (Paging, error) {
	cfg := pagingConfig{
		defaultLimit: defaultPagingLimit,
		maxLimit:     defaultPagingMaxLimit,
		maxOffset:    MaxPagingOffset,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	// Дефолт — тоже значение limit, и потолок обязан держать его так же, как
	// пришедший из query: иначе WithDefaultLimit(500) поверх WithMaxLimit(100)
	// отдавал бы 500 страницей по умолчанию.
	cfg.defaultLimit = min(max(cfg.defaultLimit, 1), cfg.maxLimit)

	limit, limitErr := pagingLimit(c, cfg)
	offset, offsetErr := pagingOffset(c, cfg)
	paging := Paging{Limit: limit, Offset: offset}

	// Оба параметра разбираются всегда, чтобы Paging был валиден целиком, но
	// наружу уходит одна ошибка — про первый некорректный параметр. Так текст
	// остаётся односоставным ("invalid limit"), как и до переезда на хелпер.
	if limitErr != nil {
		return paging, limitErr
	}

	return paging, offsetErr
}

// pagingLimit возвращает валидный размер страницы всегда: на нераспарсиваемом
// значении echo.QueryParamOr отдаёт нуль типа, а не дефолт, поэтому дефолт
// проставляется здесь явно.
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

// pagingOffset возвращает валидную позицию всегда: ниже нуля — начало набора,
// выше потолка — самая глубокая доступная страница, а не начало.
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
