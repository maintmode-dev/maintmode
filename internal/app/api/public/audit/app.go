package audit

import (
	"github.com/ruko1202/maintmode/internal/services/auditor"
)

type Implementation struct {
	auditSrv *auditor.Auditor
}

func New(
	auditSrv *auditor.Auditor,
) *Implementation {
	return &Implementation{
		auditSrv: auditSrv,
	}
}
