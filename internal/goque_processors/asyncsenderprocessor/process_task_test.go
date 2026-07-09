package asyncsenderprocessor

import (
	"context"
	"errors"
	"testing"

	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
	mock_notifytransport "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/notifytransport"
)

// resolverReturning builds a mock TransportResolver whose Get yields (tr, err)
// for any call — the processor resolves exactly once per task.
func resolverReturning(ctrl *gomock.Controller, tr notifytransport.Transport, err error) *mock_notifytransport.MockTransportResolver {
	resolver := mock_notifytransport.NewMockTransportResolver(ctrl)
	resolver.EXPECT().Get(gomock.Any(), gomock.Any()).Return(tr, err)
	return resolver
}

func newTask() *goque.TypedTask[entity.ProcessorTaskPayloadEventNotify] {
	return &goque.TypedTask[entity.ProcessorTaskPayloadEventNotify]{
		Task: &goque.Task{},
		Payload: entity.ProcessorTaskPayloadEventNotify{
			TransportName: entity.NotifyTransportSlack,
			Target:        "C123",
			Subject:       "s",
			Body:          "b",
		},
	}
}

// A disabled/unconfigured integration must be dropped best-effort: the processor
// returns nil (goque acks, no retry to the dead-letter queue) and never sends.
func TestProcessTask_DropsWhenIntegrationDisabled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	p := newQueueProcessorProcessor(resolverReturning(ctrl, nil, apperr.ErrIntegrationDisabled))

	err := p.ProcessTask(context.Background(), newTask())
	require.NoError(t, err, "a disabled integration is a best-effort drop, not a retryable failure")
}

// A wrapped disabled error (e.g. "not configured") is also a drop.
func TestProcessTask_DropsWhenWrappedDisabled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	p := newQueueProcessorProcessor(resolverReturning(ctrl, nil,
		errors.Join(apperr.ErrIntegrationDisabled, errors.New("not configured"))))

	require.NoError(t, p.ProcessTask(context.Background(), newTask()))
}

// A non-disabled resolve error (e.g. a genuine build failure) is retryable — the
// processor returns the error so goque retries.
func TestProcessTask_RetriesOnOtherResolveError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	p := newQueueProcessorProcessor(resolverReturning(ctrl, nil, errors.New("boom")))

	err := p.ProcessTask(context.Background(), newTask())
	require.Error(t, err, "a non-disabled failure must surface so goque retries")
}

// The happy path delivers via the resolved transport.
func TestProcessTask_SendsWhenResolved(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	tr := mock_notifytransport.NewMockTransport(ctrl)
	// The assertion IS the expectation: Send must be invoked exactly once for a
	// resolved transport; an unmet expectation fails the test at ctrl teardown.
	tr.EXPECT().Send(gomock.Any(), "C123", gomock.Any()).Return(nil)

	p := newQueueProcessorProcessor(resolverReturning(ctrl, tr, nil))

	require.NoError(t, p.ProcessTask(context.Background(), newTask()))
}
