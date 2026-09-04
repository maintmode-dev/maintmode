package otpemailprocessor

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/goque"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

const testTTL = 5 * time.Minute

// The round trip is the point of the whole design: the queue holds a sealed
// code, and only the processor -- holding the KEK -- can turn it back into
// something a person can read.
func TestProcessTask_SealedCodeReachesTheEmail(t *testing.T) {
	t.Parallel()

	sent := &recordingSender{}
	p, payload := newFixture(t, time.Now().Add(testTTL), sent)

	require.NoError(t, p.ProcessTask(context.Background(), typedTask(payload)))

	require.Equal(t, entity.NotifyTransportEmail, sent.transport)
	require.Equal(t, payload.Target, sent.target)
	require.Contains(t, sent.msg.Body, testCode)
	require.Equal(t, entity.HTMLMessageMIME, sent.msg.MessageMIME,
		"plain text would skip the transport's branded frame")
}

// An expired code must be canceled, not delivered and not retried. Returning
// nil would mark an undelivered code as processed successfully; returning a
// plain error would retry a code that can never become valid again.
func TestProcessTask_ExpiredCodeIsCancelledNotSent(t *testing.T) {
	t.Parallel()

	sent := &recordingSender{}
	p, payload := newFixture(t, time.Now().Add(-time.Second), sent)

	err := p.ProcessTask(context.Background(), typedTask(payload))

	require.ErrorIs(t, err, goque.ErrTaskCancel)
	require.False(t, sent.called, "an expired code must never reach the transport")
}

// A KEK the keyring no longer serves -- the rotation case. It must surface as an
// ordinary error, distinct from a cancel, so the two are told apart in the log.
func TestProcessTask_UnknownKEKIsAnError(t *testing.T) {
	t.Parallel()

	sent := &recordingSender{}
	p, payload := newFixture(t, time.Now().Add(testTTL), sent)
	payload.KEKURI = "local-kms://retired"

	err := p.ProcessTask(context.Background(), typedTask(payload))

	require.Error(t, err)
	require.NotErrorIs(t, err, goque.ErrTaskCancel)
	require.False(t, sent.called)
}

// The AAD binds the envelope to its credential row: a payload edited to claim a
// different id must fail to open rather than decrypt in the wrong context.
func TestProcessTask_CodeIsBoundToItsCredential(t *testing.T) {
	t.Parallel()

	sent := &recordingSender{}
	p, payload := newFixture(t, time.Now().Add(testTTL), sent)
	payload.CredentialID = uuid.New()

	require.Error(t, p.ProcessTask(context.Background(), typedTask(payload)))
	require.False(t, sent.called)
}

// A transport failure is retryable, so it must not be a cancel.
func TestProcessTask_TransportFailureIsRetryable(t *testing.T) {
	t.Parallel()

	sent := &recordingSender{err: errors.New("smtp unreachable")}
	p, payload := newFixture(t, time.Now().Add(testTTL), sent)

	err := p.ProcessTask(context.Background(), typedTask(payload))

	require.Error(t, err)
	require.NotErrorIs(t, err, goque.ErrTaskCancel)
}

// testCode is the code every fixture seals. Fixed rather than random: these
// tests assert on the round trip, not on generation, which xcripto covers.
const testCode = "481920"

// newFixture builds a processor over a real keyring and cipher, plus a payload
// carrying code sealed exactly as the issuing service seals it.
func newFixture(
	t *testing.T,
	expiresAt time.Time,
	sender messageSender,
) (*queueProcessor, entity.ProcessorTaskPayloadOTPEmail) {
	t.Helper()

	const kekURI = "local-kms://test"
	keyring, err := secrets.NewLocalKeyring(kekURI, map[string]string{
		// 32 bytes; a local KEK is rejected at any other length.
		kekURI: hex.EncodeToString(bytes.Repeat([]byte{0xA7}, 32)),
	})
	require.NoError(t, err)

	cipher := secrets.NewAESCipher()
	credentialID := uuid.New()

	dek, err := secrets.GenerateDEK()
	require.NoError(t, err)

	envelope, err := cipher.Encrypt(dek, []byte(testCode), secrets.OTPCodeAAD(credentialID.String()))
	require.NoError(t, err)

	wrapped, gotKEKURI, err := keyring.WrapDEK(dek)
	require.NoError(t, err)

	return newQueueProcessor(keyring, cipher, sender, testTTL, time.Now),
		entity.ProcessorTaskPayloadOTPEmail{
			CredentialID: credentialID,
			Target:       "user@example.com",
			Code:         envelope,
			DEK:          wrapped,
			KEKURI:       gotKEKURI,
			ExpiresAt:    expiresAt,
		}
}

func typedTask(p entity.ProcessorTaskPayloadOTPEmail) *goque.TypedTask[entity.ProcessorTaskPayloadOTPEmail] {
	return &goque.TypedTask[entity.ProcessorTaskPayloadOTPEmail]{Payload: p}
}

type recordingSender struct {
	called    bool
	transport entity.NotifyTransport
	target    string
	msg       entity.NotifyMessage
	err       error
}

func (s *recordingSender) Send(
	_ context.Context,
	trName entity.NotifyTransport,
	target string,
	msg entity.NotifyMessage,
	_ *entity.MessageRef,
) (entity.SendResult, error) {
	s.called = true
	s.transport = trName
	s.target = target
	s.msg = msg
	return entity.SendResult{}, s.err
}
