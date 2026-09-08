package integration

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	emailtransport "github.com/ruko1202/maintmode/internal/gateways/notifytransport/email"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

const (
	// probeTimeoutCap bounds how long one probe may hold an HTTP worker. The
	// caller supplies the timeout and only its format is checked, so "10m" would
	// otherwise pin a worker for ten minutes. The request context is passed down
	// too, but Go's HTTP server sets no deadline of its own, so it only helps
	// when the client disconnects -- this cap is the actual guarantee.
	probeTimeoutCap = 30 * time.Second

	// probeTimeoutDefault applies when the caller names no timeout. Stated here
	// rather than left to the transport's own default: the cap below has to be
	// applied to a real number, and min(0, cap) is 0 -- which would hand the
	// probe an unbounded dial while looking like it was capped.
	probeTimeoutDefault = 10 * time.Second

	// probeErrorLimit caps how much of the far end's error travels back in the
	// response body. Long enough for an SMTP reply plus context, short enough
	// that a pathological error cannot be reflected wholesale.
	probeErrorLimit = 512

	probeSubject = "MaintMode SMTP configuration test"
	probeBody    = "This is a test message sent by MaintMode to check an SMTP configuration.\n" +
		"No action is required."
)

// Probe exercises a set of integration settings against the real service and
// reports whether it worked. Today only the email kind is probeable.
//
// It writes nothing -- no row, no queue task, no record of the outcome -- and
// reads nothing: the settings come entirely from cmd, so the endpoint works
// against a configuration that has never been saved, which is the point. An
// admin gets a truthful answer before committing a config rather than
// discovering it is broken when a user cannot sign in.
//
// Failures reaching the far end are wrapped in ErrIntegrationProbeFailed so the
// API layer answers 502 with the detail intact; everything the caller could fix
// by editing the form is ErrValidation.
func (s *Service) Probe(ctx context.Context, cmd *entity.ProbeIntegrationCmd) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Probe",
		xfield.String("kind", cmd.Kind),
	)
	defer span.End()

	in, err := s.registry.Get(cmd.Kind)
	if err != nil {
		return err
	}

	if err := validateProbeCmd(cmd); err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	// Undeclared keys are dropped, not used; the warning is the only trace a
	// typoed key name leaves. Same treatment as create and update -- one rule
	// for the field across every endpoint that takes it.
	warnUnknownSecretKeys(ctx, in, cmd.Secrets)

	// The same guard the save path applies, for a mistake that matters more
	// here: a password typed into the config section is dropped by Parse
	// (EmailSettings.Password is json:"-"), so without this the probe would
	// connect anonymously and report success for a config that fails on the
	// first real send.
	if err := rejectSecretKeysInConfig(in, cmd.Config); err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	settings, err := in.Parse(cmd.Config, cmd.Secrets)
	if err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}
	if err := in.Validate(settings); err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	email, ok := settings.(integrationkinds.EmailSettings)
	if !ok {
		return fmt.Errorf("%w: kind %q cannot be probed", apperr.ErrValidation, cmd.Kind)
	}

	return s.probeEmail(ctx, email, cmd)
}

func (s *Service) probeEmail(
	ctx context.Context,
	settings integrationkinds.EmailSettings,
	cmd *entity.ProbeIntegrationCmd,
) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Integration.Probe.probeEmail",
		xfield.String("actor_id", cmd.Actor.ID.String()),
		xfield.String("host", settings.Host),
		xfield.Int("port", settings.Port),
	)
	defer span.End()

	timeout, err := probeTimeout(settings.Timeout)
	if err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	client, err := emailtransport.New(emailtransport.Params{
		Host:      settings.Host,
		Port:      settings.Port,
		Username:  settings.Username,
		Password:  settings.Password,
		From:      settings.From,
		ReplyTo:   settings.ReplyTo,
		TLSPolicy: settings.TLSPolicy,
		Timeout:   timeout,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", apperr.ErrValidation, err)
	}

	// Plain text on purpose: the HTML path wraps the body in the branded layout,
	// whose footer says the reader is getting this because of their account
	// activity. That is false for a configuration probe.
	_, err = client.Send(ctx, cmd.To, entity.NotifyMessage{
		Subject:     probeSubject,
		Body:        probeBody,
		MessageMIME: entity.TextMessageMIME,
	}, nil)

	// This line is the only record the probe leaves: it writes nothing to the
	// database and is deliberately not audited (an audit publish goes through
	// the outbox, i.e. a write, on a path that must stay write-free). Naming the
	// actor here is what keeps "who asked this server to connect where"
	// answerable at all.
	//
	// Nothing about the settings is logged beyond the destination: the password
	// travels next to this logger, and the safe fields are the useful ones
	// anyway.
	if err != nil {
		xlog.Warn(ctx, "smtp probe failed", xfield.Error(err))
		return fmt.Errorf("%w: %s", apperr.ErrIntegrationProbeFailed, truncateError(err.Error()))
	}

	xlog.Info(ctx, "smtp probe delivered")
	return nil
}

// truncateError bounds the far end's error without eating it.
//
// ToValidUTF8 does both jobs at once: a byte the cut left dangling and a byte
// the server never encoded properly are the same kind of invalid, and each is
// replaced rather than dropped. An earlier version shortened the string until
// the whole prefix parsed, which turned a latin-1 answer into "535 auth
// failed" at best and into an empty string at worst -- losing the diagnostic
// to a stray byte defeats the reason this text is returned at all.
func truncateError(s string) string {
	if len(s) > probeErrorLimit {
		s = s[:probeErrorLimit]
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

// probeTimeout resolves the dial timeout: the caller's value if they gave one,
// the default otherwise, and never more than the cap.
//
// A negative value is rejected here for the message, not the outcome:
// go-mail refuses it too, so either way the caller gets a validation error --
// but "timeout must not be negative" names the field, while the library's
// "failed to apply option" leaves an admin guessing which of seven settings is
// wrong.
func probeTimeout(raw string) (time.Duration, error) {
	timeout, err := xtime.ParseTimeout(raw)
	if err != nil {
		return 0, err
	}
	if timeout < 0 {
		return 0, fmt.Errorf("timeout must not be negative, got %s", timeout)
	}
	return min(cmp.Or(timeout, probeTimeoutDefault), probeTimeoutCap), nil
}

func validateProbeCmd(cmd *entity.ProbeIntegrationCmd) error {
	if err := validation.ValidateStruct(cmd,
		validation.Field(&cmd.To, validation.Required, is.EmailFormat),
		validation.Field(&cmd.Config, validation.Required),
		// Required, not optional: the actor is the only trace this operation
		// leaves (see probeEmail), so a missing one must fail loudly rather than
		// be papered over with a nil-safe fallback logging an anonymous probe.
		validation.Field(&cmd.Actor, validation.Required),
	); err != nil {
		return err
	}
	// A recipient carrying CR/LF or a comma is header-injection shaped, and
	// is.EmailFormat already rejects every such string -- no separate guard is
	// needed here, only the assurance that this validation runs before anything
	// reaches the mail library.
	return nil
}
