package blacklisttoken

import (
	valkeylib "github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "blacklist:"
	// sessionKeyPrefix sits under keyPrefix so both kinds share one namespace,
	// and cannot collide with a jti entry: a jti is a bare UUID, never "session:...".
	sessionKeyPrefix = keyPrefix + "session:"
)

type Store struct {
	db *valkeylib.Client
}

func NewStore(db *valkeylib.Client) *Store {
	return &Store{db: db}
}
