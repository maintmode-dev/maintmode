package auditor

import (
	"github.com/ruko1202/maintmode/internal/storages/audit"
)

type Auditor struct {
	store *audit.Store
}

func NewAuditor(store *audit.Store) *Auditor {
	return &Auditor{
		store: store,
	}
}
