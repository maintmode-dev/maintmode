package render

import (
	"fmt"
	"text/template"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/render/templates"
)

var kindToTemplate = map[entity.NotifyEventKind]string{
	entity.NotifyEventMaintStarted:   "maint_started.tmpl",
	entity.NotifyEventMaintCompleted: "maint_completed.tmpl",
	entity.NotifyEventMaintCancelled: "maint_cancelled.tmpl",
	entity.NotifyEventMaintReminder:  "maint_reminder.tmpl",
	entity.NotifyEventStepStarted:    "step_started.tmpl",
	entity.NotifyEventStepCompleted:  "step_completed.tmpl",
	entity.NotifyEventStepCancelled:  "step_cancelled.tmpl",
}

// Service parses the embedded templates once at construction and
// produces a gwmsg.Message per event kind.
type Service struct {
	tmpls map[entity.NotifyEventKind]*template.Template
}

func New() (*Service, error) {
	r := &Service{
		tmpls: make(map[entity.NotifyEventKind]*template.Template, len(kindToTemplate)),
	}

	for kind, path := range kindToTemplate {
		raw, err := templates.FS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", path, err)
		}

		t, err := template.New(string(kind)).Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", path, err)
		}

		r.tmpls[kind] = t
	}

	return r, nil
}
