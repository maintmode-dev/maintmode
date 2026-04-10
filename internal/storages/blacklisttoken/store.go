package blacklisttoken

import (
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "blacklist:"

type Store struct {
	db *redis.Client
}

func NewStore(db *redis.Client) *Store {
	return &Store{db: db}
}
