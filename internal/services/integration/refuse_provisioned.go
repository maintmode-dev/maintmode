package integration

import (
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// refuseProvisioned turns every admin write to a config-owned row into
// ErrIntegrationNameReserved. Called on the row read FOR UPDATE, so a
// concurrent provisioning run cannot flip the flag between the check and the
// write.
func refuseProvisioned(row *entity.IntegrationSetting) error {
	if row.Provisioned {
		return fmt.Errorf("%w: %s/%s", apperr.ErrIntegrationNameReserved, row.Kind, row.Name)
	}

	return nil
}
