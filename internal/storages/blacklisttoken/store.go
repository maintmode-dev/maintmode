package blacklisttoken

import (
	valkeylib "github.com/redis/go-redis/v9"
)

const keyPrefix = "blacklist:"

type Store struct {
	db *valkeylib.Client
}

func NewStore(db *valkeylib.Client) *Store {
	return &Store{db: db}
}
