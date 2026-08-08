package notifytargets

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
)

func toDB(maintID uuid.UUID, sub *entity.NotifyTarget) *model.MaintenanceNotifyTargets {
	return &model.MaintenanceNotifyTargets{
		MaintenanceID: maintID,
		ChannelID:     sub.ChannelID,
	}
}

// fromDB maps the persisted columns only; the catalog-resolved fields
// (Transport, TransportChannelID, ChannelName) are filled by read paths that
// join the catalog (ListByMaint).
func fromDB(m *model.MaintenanceNotifyTargets) *entity.NotifyTarget {
	return &entity.NotifyTarget{
		ID:          m.ID,
		MaintID:     m.MaintenanceID,
		ChannelID:   m.ChannelID,
		CreatedAt:   m.CreatedAt,
		RootRef:     rootRefFromDB(m),
		RootChannel: lo.FromPtr(m.RootChannel),
	}
}

// rootRefFromDB yields a reference only when the message id carries a value.
// Empty strings are treated like NULL: a half-written root is not a root, and
// letting one through would produce a reply to message "".
func rootRefFromDB(m *model.MaintenanceNotifyTargets) *entity.MessageRef {
	if m.RootMessageID == nil || *m.RootMessageID == "" {
		return nil
	}

	return &entity.MessageRef{MessageID: *m.RootMessageID}
}
