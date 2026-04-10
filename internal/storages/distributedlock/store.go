package distributedlock

import (
	"github.com/redis/go-redis/v9"
)

const lockPrefix = "lock:"

// Store implements distributed locking using Redis SET NX.
type Store struct {
	client *redis.Client
}

// NewStore creates a new Redis-backed distributed lock.
func NewStore(client *redis.Client) *Store {
	return &Store{client: client}
}
