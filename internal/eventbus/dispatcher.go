package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/eventbus/events"
)

// Event — маркер публикуемого события. EventName служит дискриминатором для
// логов/спанов и помечает тип как событие шины. Конкретные события живут в
// доменных пакетах (internal/events); сам eventbus от домена не зависит.
type Event interface {
	EventType() events.EventType
}

// Listener обрабатывает события, которые сам объявляет через Supports.
// Handle вызывается диспетчером только если Supports(event) == true.
type Listener interface {
	// SupportEvents возвращает список поддерживаемых эвентов
	SupportEvents() []events.EventType
	// Handle обрабатывает событие. Ошибка best-effort: диспетчер её логирует
	// и продолжает работу, не пробрасывая вызывающему.
	Handle(ctx context.Context, event Event) error
}

// Dispatcher — глобальный синхронный best-effort диспетчер событий.
// Прогоняет каждое событие по всем листенерам, заявившим Supports == true.
type Dispatcher struct {
	wg             *sync.WaitGroup
	listenersStore map[events.EventType][]Listener
}

func NewDispatcher(listeners ...Listener) *Dispatcher {
	listenersStore := make(map[events.EventType][]Listener)
	for _, l := range listeners {
		for _, e := range l.SupportEvents() {
			listenersStore[e] = append(listenersStore[e], l)
		}
	}

	return &Dispatcher{
		wg:             &sync.WaitGroup{},
		listenersStore: listenersStore,
	}
}

// AsyncDispatch асинхронно прогоняет event по подходящим листенерам.
//
// wg.Add вызывается ДО запуска горутины: иначе Stop мог бы проскочить Wait
// между возвратом AsyncDispatch и стартом горутины и потерять диспатч.
// Контекст отвязан от родителя (WithoutCancel): trace/values сохраняются, но
// отмена родительского запроса не прерывает фоновую запись.
func (d *Dispatcher) AsyncDispatch(ctx context.Context, event Event) {
	ctx, span := xlog.WithOperationSpan(context.WithoutCancel(ctx), "service.Dispatcher.AsyncDispatch",
		xfield.String("event", string(event.EventType())))

	d.wg.Add(1)
	go func() {
		defer span.End()
		defer d.wg.Done()

		if err := d.Dispatch(ctx, event); err != nil {
			xlog.Error(ctx, "event dispatch failed", xfield.Error(err))
		}
	}()
}

// Dispatch синхронно прогоняет event по подходящим листенерам.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Dispatcher.Dispatch",
		xfield.String("event", string(event.EventType())))
	defer span.End()

	listeners, ok := d.listenersStore[event.EventType()]
	if !ok {
		return fmt.Errorf("%w: %s", apperr.ErrUnsupportedEvent, event.EventType())
	}

	// Best-effort: ошибка листенера не прерывает остальных. Логирование —
	// на стороне владельца вызова (AsyncDispatch логирует; синхронный
	// вызывающий решает сам), чтобы не дублировать запись в лог.
	var dispatchErr error
	for _, l := range listeners {
		if err := l.Handle(ctx, event); err != nil {
			dispatchErr = errors.Join(dispatchErr, fmt.Errorf("listener %T: %w", l, err))
		}
	}

	return dispatchErr
}

func (d *Dispatcher) Stop(ctx context.Context) error {
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		d.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stop:
		return nil
	}
}
