// Package otpemailprocessor delivers one-time sign-in codes by email.
//
// It exists instead of reusing the generic async sender because the code cannot
// be in the task as plaintext (see entity.ProcessorTaskPayloadOTPEmail): the
// payload carries a sealed code, and the body is rendered here, at delivery,
// rather than at enqueue.
package otpemailprocessor

import (
	"context"
	"time"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// keyring unwraps the per-task data key. Declared consumer-side so this package
// depends on the capability rather than on the crypto package's concrete type,
// matching how the integration service takes its keyring.
type keyring interface {
	UnwrapDEK(wrapped []byte, kekURI string) ([]byte, error)
}

// cipher opens the sealed code once its key is unwrapped.
type cipher interface {
	Decrypt(dek, envelope, aad []byte) ([]byte, error)
}

// messageSender is the delivery facade. Same shape the generic send processor
// declares: this processor only prepares the message and delegates.
type messageSender interface {
	Send(
		ctx context.Context,
		trName entity.NotifyTransport,
		target string,
		msg entity.NotifyMessage,
		replyTo *entity.MessageRef,
	) (entity.SendResult, error)
}

// clock reads the current time. Injected so a test can put a task past its
// expiry without sleeping.
type clock func() time.Time

// NewTaskProcessor returns the goque TaskProcessor that delivers one-time-code
// emails.
//
// A payload that cannot be decoded is canceled rather than retried: the bytes
// will not improve, and retrying keeps a sealed authentication code cycling
// through the queue for no reason.
func NewTaskProcessor(
	kr keyring,
	ciph cipher,
	sender messageSender,
	ttl time.Duration,
) goque.TaskProcessor {
	return goque.NewTypedTaskProcessor[entity.ProcessorTaskPayloadOTPEmail](
		newQueueProcessor(kr, ciph, sender, ttl, xtime.UTCNow),
		goque.WithCancelTaskWhenPayloadDecodeError[entity.ProcessorTaskPayloadOTPEmail](),
	)
}

type queueProcessor struct {
	keyring keyring
	cipher  cipher
	sender  messageSender
	ttl     time.Duration
	now     clock
}

func newQueueProcessor(
	kr keyring,
	ciph cipher,
	sender messageSender,
	ttl time.Duration,
	now clock,
) *queueProcessor {
	return &queueProcessor{
		keyring: kr,
		cipher:  ciph,
		sender:  sender,
		ttl:     ttl,
		now:     now,
	}
}
