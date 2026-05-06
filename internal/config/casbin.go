package config

import (
	"fmt"

	"github.com/casbin/casbin/v3/persist"
	casbinfileadapter "github.com/casbin/casbin/v3/persist/file-adapter"
	casbinstringadapter "github.com/casbin/casbin/v3/persist/string-adapter"
)

type AuthorizationAdapter = string

const (
	AuthorizationAdapterFile   AuthorizationAdapter = "file"
	AuthorizationAdapterMemory AuthorizationAdapter = "memory"
	AuthorizationAdapterDB     AuthorizationAdapter = "db"
)

func NewCasbinAdapter(cfg RbacConfig) (persist.Adapter, error) {
	var adapter persist.Adapter
	switch cfg.Adapter {
	case AuthorizationAdapterFile:
		adapter = casbinfileadapter.NewAdapter(cfg.PolicyPath)
	case AuthorizationAdapterMemory:
		adapter = casbinstringadapter.NewAdapter(cfg.PolicyData)
	case AuthorizationAdapterDB:
		return nil, fmt.Errorf("unsupported casbin adapter: %s", cfg.Adapter)
	default:
		return nil, fmt.Errorf("unsupported casbin adapter: %s", cfg.Adapter)
	}

	return adapter, nil
}
