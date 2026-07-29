package render

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// templateData is the value passed to the embedded text/template.
//
// OwnerMention and Mentions shadow the embedded NotifyEvent fields on purpose:
// the templates need the already-chosen string for this transport, never the
// struct or the slice. For Mentions the shadowing is load-bearing rather than
// merely convenient — {{if .Mentions}} on the event's slice is true for a
// non-empty list of people who all filtered out, and would render a bare
// "Mentions:" header with nothing after it.
type templateData struct {
	entity.NotifyEvent
	MaintURL     string
	OwnerMention string
	Mentions     string
}

// Render produces the message for one transport. The transport selects which
// messenger handle the owner is mentioned by, so a maintenance with both a
// Slack and a Telegram subscription is rendered once per transport rather than
// once per event.
//
// The result is always TextMessageMIME, but that is a formatting label, NOT a
// safety barrier — no transport reads the field, and the Slack gateway sends
// with slack-go's escaping disabled, so mrkdwn in the body is parsed regardless.
// Safety for user-supplied text interpolated into the body comes from isSafeTag
// for handles and sanitizeDisplayName for display names, nothing else.
func (r *Service) Render(
	ctx context.Context,
	transport entity.NotifyTransport,
	evt entity.NotifyEvent,
) (entity.NotifyMessage, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Render.Render")
	defer span.End()

	body, err := r.render(ctx, transport, evt)
	if err != nil {
		xlog.Error(ctx, "failed to render event", xfield.Error(err))
		return entity.NotifyMessage{}, fmt.Errorf("render event: %w", err)
	}

	return entity.NotifyMessage{
		Subject:     evt.Kind.Subject(),
		Body:        strings.TrimSpace(body),
		MessageMIME: entity.TextMessageMIME,
	}, nil
}

func (r *Service) render(
	ctx context.Context,
	transport entity.NotifyTransport,
	evt entity.NotifyEvent,
) (string, error) {
	t, ok := r.tmpls[evt.Kind]
	if !ok {
		return "", fmt.Errorf("no template for event kind %q", evt.Kind)
	}

	maintURL, err := buildMaintURL(evt.FrontendURL, evt.MaintID.String())
	if err != nil {
		return "", fmt.Errorf("join maintenance URL: %w", err)
	}

	data := templateData{
		NotifyEvent:  evt,
		MaintURL:     maintURL,
		OwnerMention: ownerMention(ctx, transport, evt.OwnerMention),
		Mentions:     mentionsLine(ctx, transport, evt.Mentions),
	}

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", evt.Kind, err)
	}

	return buf.String(), nil
}

func buildMaintURL(base, maintID string) (string, error) {
	u, err := url.JoinPath(base, "/maintenance/", maintID)
	if err != nil {
		return "", err
	}

	return u, nil
}
