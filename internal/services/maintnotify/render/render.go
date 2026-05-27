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
type templateData struct {
	entity.NotifyEvent
	MaintURL string
}

func (r *Service) Render(ctx context.Context, evt entity.NotifyEvent) (entity.Message, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Render.Render")
	defer span.End()

	body, err := r.render(evt)
	if err != nil {
		xlog.Error(ctx, "failed to render event", xfield.Error(err))
		return entity.Message{}, fmt.Errorf("render event: %w", err)
	}

	return entity.Message{
		Subject: evt.Kind.Subject(),
		Body:    strings.TrimSpace(body),
	}, nil
}

func (r *Service) render(evt entity.NotifyEvent) (string, error) {
	t, ok := r.tmpls[evt.Kind]
	if !ok {
		return "", fmt.Errorf("no template for event kind %q", evt.Kind)
	}

	maintURL, err := buildMaintURL(evt.FrontendURL, evt.MaintID.String())
	if err != nil {
		return "", fmt.Errorf("join maintenance URL: %w", err)
	}

	data := templateData{
		NotifyEvent: evt,
		MaintURL:    maintURL,
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
